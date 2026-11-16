// Moondust registry server. Standard library only.
// It never stores package code: it logs people in with GitHub, checks repo
// ownership, and commits entries to registry.json + index.txt in your Pages repo.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type User struct {
	GitHubID int64 `json:"github_id"`
}
type Version struct {
	Repo   string   `json:"repo"`
	Commit string   `json:"commit"`
	Main   string   `json:"main"`
	Lua    []string `json:"lua"`
}
type Package struct {
	Description string             `json:"description"`
	Versions    map[string]Version `json:"versions"`
}
type Registry struct {
	Users    map[string]User    `json:"users"`
	Packages map[string]Package `json:"packages"`
}
type Me struct {
	ID    int64
	Login string
}
type apiErr struct {
	code int
	msg  string
}

func (e *apiErr) Error() string { return e.msg }
func bad(c int, m string) error { return &apiErr{c, m} }

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var (
	clientID  = os.Getenv("GITHUB_CLIENT_ID")
	clientSec = os.Getenv("GITHUB_CLIENT_SECRET")
	ghToken   = os.Getenv("REGISTRY_TOKEN")
	secret    = []byte(os.Getenv("SESSION_SECRET"))
	baseURL   = strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
	regRepo   = env("REGISTRY_REPO", "thegamingmahi/thegamingmahi.github.io")
	regDir    = env("REGISTRY_DIR", "Moondust")
	siteURL   = env("SITE_URL", "https://thegamingmahi.github.io/Moondust/")
	addr      = env("ADDR", "127.0.0.1:8080")

	client = &http.Client{Timeout: 15 * time.Second}
	reg    atomic.Pointer[Registry]
	mu     sync.Mutex // one registry write at a time
	cmu    sync.Mutex
	last   = map[int64]time.Time{}

	nameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,28}[a-z0-9]$`)
	repoRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	pkgRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	verRe    = regexp.MustCompile(`^\d{1,4}\.\d{1,4}\.\d{1,4}$`)
	mainRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.lua$`)
	reserved = map[string]bool{"admin": true, "moondust": true, "moonrocks": true, "root": true, "api": true, "www": true, "official": true}
	okLua    = map[string]bool{"5.1": true, "5.2": true, "5.3": true, "5.4": true, "5.5": true}
)

// signed tokens: no database, nothing to store
func sign(v map[string]any) string {
	b, _ := json.Marshal(v)
	p := base64.RawURLEncoding.EncodeToString(b)
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func verify(t string) map[string]any {
	i := strings.LastIndex(t, ".")
	if i < 0 {
		return nil
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(t[:i]))
	if !hmac.Equal([]byte(base64.RawURLEncoding.EncodeToString(m.Sum(nil))), []byte(t[i+1:])) {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(t[:i])
	if err != nil {
		return nil
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	if exp, _ := v["exp"].(float64); int64(exp) < time.Now().Unix() {
		return nil
	}
	return v
}

func gh(method, u, token string, body, out any) (int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, u, r)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil {
		json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
	}
	return resp.StatusCode, nil
}

type fileResp struct {
	Content string `json:"content"`
	SHA     string `json:"sha"`
}

func fileURL(name string) string {
	return "https://api.github.com/repos/" + regRepo + "/contents/" + regDir + "/" + name
}

func load() (*Registry, string, error) {
	r := &Registry{Users: map[string]User{}, Packages: map[string]Package{}}
	var f fileResp
	code, err := gh("GET", fileURL("registry.json"), ghToken, nil, &f)
	if err != nil {
		return nil, "", err
	}
	if code == 404 {
		return r, "", nil
	}
	if code != 200 {
		return nil, "", fmt.Errorf("github returned %d while reading registry", code)
	}
	b, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err := json.Unmarshal(b, r); err != nil {
		return nil, "", err
	}
	if r.Users == nil {
		r.Users = map[string]User{}
	}
	if r.Packages == nil {
		r.Packages = map[string]Package{}
	}
	return r, f.SHA, nil
}

func put(name string, data []byte, sha, msg string) error {
	body := map[string]any{"message": msg, "content": base64.StdEncoding.EncodeToString(data)}
	if sha != "" {
		body["sha"] = sha
	}
	code, err := gh("PUT", fileURL(name), ghToken, body, nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 201 {
		return fmt.Errorf("github returned %d while writing %s", code, name)
	}
	return nil
}

// index.txt is what moondust.lua reads: one line per version
func makeIndex(r *Registry) []byte {
	var lines []string
	for name, p := range r.Packages {
		for v, e := range p.Versions {
			lua := "*"
			if len(e.Lua) > 0 {
				lua = strings.Join(e.Lua, ",")
			}
			lines = append(lines, fmt.Sprintf("%s %s %s %s %s %s", name, v, e.Repo, e.Commit, e.Main, lua))
		}
	}
	sort.Strings(lines)
	return []byte("# moondust index\n" + strings.Join(lines, "\n") + "\n")
}

// mutate reloads the registry, applies a change, and commits it
func mutate(msg string, fn func(*Registry) error) error {
	mu.Lock()
	defer mu.Unlock()
	r, sha, err := load()
	if err != nil {
		return err
	}
	if err := fn(r); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	if err := put("registry.json", data, sha, msg); err != nil {
		return err
	}
	reg.Store(r)
	var f fileResp
	if code, _ := gh("GET", fileURL("index.txt"), ghToken, nil, &f); code != 200 {
		f.SHA = ""
	}
	return put("index.txt", makeIndex(r), f.SHA, msg)
}

func usernameOf(r *Registry, id int64) string {
	for n, u := range r.Users {
		if u.GitHubID == id {
			return n
		}
	}
	return ""
}

func publish(m *Me, in string) (string, error) {
	in = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(in), "/"), ".git")
	in = strings.TrimPrefix(strings.TrimPrefix(in, "https://github.com/"), "http://github.com/")
	parts := strings.Split(in, "/")
	if len(parts) != 2 || !repoRe.MatchString(parts[0]) || !repoRe.MatchString(parts[1]) || strings.Contains(in, "..") {
		return "", bad(400, "give a GitHub repo like owner/name")
	}
	if usernameOf(reg.Load(), m.ID) == "" {
		return "", bad(403, "claim a username first")
	}
	cmu.Lock()
	if time.Since(last[m.ID]) < 10*time.Second {
		cmu.Unlock()
		return "", bad(429, "slow down, try again in a few seconds")
	}
	last[m.ID] = time.Now()
	cmu.Unlock()

	var info struct {
		DefaultBranch string `json:"default_branch"`
		Owner         struct {
			ID int64 `json:"id"`
		} `json:"owner"`
	}
	api := "https://api.github.com/repos/" + in
	if code, err := gh("GET", api, ghToken, nil, &info); err != nil || code != 200 {
		return "", bad(404, "repo not found (it must be public)")
	}
	if info.Owner.ID != m.ID {
		return "", bad(403, "you can only publish repos you own")
	}
	var c struct {
		SHA string `json:"sha"`
	}
	if code, err := gh("GET", api+"/commits/"+url.PathEscape(info.DefaultBranch), ghToken, nil, &c); err != nil || code != 200 || len(c.SHA) != 40 {
		return "", bad(502, "could not read the latest commit")
	}
	raw := func(path string, max int64) ([]byte, error) {
		resp, err := client.Get("https://raw.githubusercontent.com/" + in + "/" + c.SHA + "/" + path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("%s not found in the repo root", path)
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, max+1))
		if int64(len(b)) > max {
			return nil, fmt.Errorf("%s is too big", path)
		}
		return b, nil
	}
	mb, err := raw("moondust.json", 16<<10)
	if err != nil {
		return "", bad(400, err.Error())
	}
	var man struct {
		Name, Description, Version, Main string
		Lua                              []string
	}
	if json.Unmarshal(mb, &man) != nil {
		return "", bad(400, "moondust.json is not valid JSON")
	}
	switch {
	case !pkgRe.MatchString(man.Name):
		return "", bad(400, "name must be letters, numbers, - or _ (max 40)")
	case !verRe.MatchString(man.Version):
		return "", bad(400, "version must look like 1.2.0")
	case !mainRe.MatchString(man.Main) || strings.Contains(man.Main, ".."):
		return "", bad(400, "main must be a .lua file in the repo root")
	}
	lua := man.Lua
	if len(lua) == 0 {
		lua = []string{"5.3"}
	}
	for _, v := range lua {
		if !okLua[v] {
			return "", bad(400, "lua versions must be from 5.1 to 5.5")
		}
	}
	if _, err := raw(man.Main, 512<<10); err != nil {
		return "", bad(400, err.Error())
	}
	desc := []rune(man.Description)
	if len(desc) > 200 {
		desc = desc[:200]
	}

	var full string
	err = mutate("publish "+man.Name+" "+man.Version, func(g *Registry) error {
		u := usernameOf(g, m.ID)
		if u == "" {
			return bad(403, "claim a username first")
		}
		full = u + "/" + man.Name
		p := g.Packages[full]
		if p.Versions == nil {
			p.Versions = map[string]Version{}
		}
		if _, ok := p.Versions[man.Version]; ok {
			return bad(409, "that version is already published, bump the version in moondust.json")
		}
		p.Versions[man.Version] = Version{Repo: in, Commit: c.SHA, Main: man.Main, Lua: lua}
		if len(desc) > 0 {
			p.Description = string(desc)
		}
		g.Packages[full] = p
		return nil
	})
	return full + "@" + man.Version, err
}

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	if e, ok := err.(*apiErr); ok {
		reply(w, e.code, map[string]string{"error": e.msg})
		return
	}
	log.Println(err)
	reply(w, 500, map[string]string{"error": "server error, try again"})
}

func authed(r *http.Request) *Me {
	v := verify(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if v == nil || v["kind"] != "session" {
		return nil
	}
	id, _ := v["id"].(float64)
	login, _ := v["login"].(string)
	return &Me{int64(id), login}
}

func api(h func(http.ResponseWriter, *http.Request, *Me)) http.HandlerFunc {
	su, _ := url.Parse(siteURL)
	origin := su.Scheme + "://" + su.Host
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		m := authed(r)
		if m == nil {
			reply(w, 401, map[string]string{"error": "log in first"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<14)
		h(w, r, m)
	}
}

func main() {
	if clientID == "" || clientSec == "" || ghToken == "" || len(secret) < 16 || baseURL == "" {
		log.Fatal("set GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, REGISTRY_TOKEN, SESSION_SECRET (16+ chars) and BASE_URL")
	}
	r, _, err := load()
	if err != nil {
		log.Fatal(err)
	}
	reg.Store(r)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		state := sign(map[string]any{"kind": "state", "n": time.Now().UnixNano(), "exp": time.Now().Add(10 * time.Minute).Unix()})
		http.Redirect(w, r, "https://github.com/login/oauth/authorize?client_id="+url.QueryEscape(clientID)+
			"&redirect_uri="+url.QueryEscape(baseURL+"/auth/callback")+"&state="+url.QueryEscape(state), 302)
	})
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if v := verify(r.URL.Query().Get("state")); v == nil || v["kind"] != "state" {
			http.Error(w, "login expired, go back and try again", 400)
			return
		}
		var tok struct {
			AccessToken string `json:"access_token"`
		}
		gh("POST", "https://github.com/login/oauth/access_token", "", map[string]string{
			"client_id": clientID, "client_secret": clientSec, "code": r.URL.Query().Get("code")}, &tok)
		var u struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		}
		if tok.AccessToken != "" {
			gh("GET", "https://api.github.com/user", tok.AccessToken, nil, &u)
		}
		if u.ID == 0 {
			http.Error(w, "GitHub login failed", 400)
			return
		}
		t := sign(map[string]any{"kind": "session", "id": u.ID, "login": u.Login, "exp": time.Now().Add(30 * 24 * time.Hour).Unix()})
		http.Redirect(w, r, siteURL+"#token="+t, 302)
	})
	mux.HandleFunc("/api/me", api(func(w http.ResponseWriter, r *http.Request, m *Me) {
		reply(w, 200, map[string]string{"login": m.Login, "username": usernameOf(reg.Load(), m.ID)})
	}))
	mux.HandleFunc("/api/username", api(func(w http.ResponseWriter, r *http.Request, m *Me) {
		var in struct{ Username string }
		json.NewDecoder(r.Body).Decode(&in)
		name := strings.ToLower(strings.TrimSpace(in.Username))
		if !nameRe.MatchString(name) || reserved[name] {
			fail(w, bad(400, "username: 3-30 letters, numbers or -, and not reserved"))
			return
		}
		err := mutate("claim username "+name, func(g *Registry) error {
			if usernameOf(g, m.ID) != "" {
				return bad(409, "you already claimed a username")
			}
			if _, ok := g.Users[name]; ok {
				return bad(409, "that username is taken")
			}
			g.Users[name] = User{m.ID}
			return nil
		})
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]string{"username": name})
	}))
	mux.HandleFunc("/api/publish", api(func(w http.ResponseWriter, r *http.Request, m *Me) {
		var in struct{ Repo string }
		json.NewDecoder(r.Body).Decode(&in)
		pub, err := publish(m, in.Repo)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]string{"published": pub})
	}))

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second}
	log.Println("moondust server listening on", addr)
	log.Fatal(srv.ListenAndServe())
}
