# Moondust

A tiny package manager for Lua. One file, no install step.

**Homepage and package browser: https://thegamingmahi.github.io/Moondust/**  
**Full documentation: https://thegamingmahi.github.io/Moondust/docs.html**

## Quick start

1. Download `moondust.lua` and put it next to your script:

   ```
   curl -O https://thegamingmahi.github.io/Moondust/moondust.lua
   ```

2. Use any package with `require`:

   ```lua
   local moondust = require("moondust")

   moondust("thegamingmahi/tinyfiles")

   writefile("hello.txt", "Hi from Moondust")
   print(readfile("hello.txt"))
   ```

The first run downloads the package into a `lua_modules/` folder. After that it loads from disk.

## Versions

```lua
moondust("thegamingmahi/tinyfiles")          -- latest version
moondust("thegamingmahi/tinyfiles@1.0.0")    -- exact version
```

Moondust remembers which version "latest" meant in a `moondust.lock` file, so your code doesn't change under you. Run `lua moondust.lua update thegamingmahi/tinyfiles` to move to a newer one.

## Editor autocomplete

On the first download, Moondust creates a small `.luarc.json` next to your script. Editors that use the Lua language server (the Lua extension for VS Code, Neovim, Zed) then autocomplete `moondust("...")` like a normal `require`:

```lua
local moondust = require("moondust")   -- keep this variable name
local tf = moondust("thegamingmahi/tinyfiles")
tf.   -- suggestions appear here
```

- Functions that a package creates as globals, like `wait()` and `writefile()` in TinyFiles, autocomplete too.
- It works even if `lua_modules` is in your `.gitignore`.
- Packages are saved as `lua_modules/author/package.lua`. Other exact versions you ask for are saved as `author/package@1.0.0.lua`.
- If you already have a `.luarc.json`, Moondust leaves it alone and prints the three settings to add.
- Set `MOONDUST_NO_LUARC=1` to turn this off. If suggestions don't show up, reload your editor window once.

## Command line

The same file works as a command:

| Command | What it does |
|---|---|
| `lua moondust.lua install author/package` | Download the latest version and record it in `moondust.lock` |
| `lua moondust.lua install author/package@1.0.0` | Same, for an exact version |
| `lua moondust.lua update author/package` | Switch one package to its newest version |
| `lua moondust.lua update` | Update every package in `moondust.lock` |
| `lua moondust.lua list` | Show installed packages with their version and checksum |
| `lua moondust.lua verify` | Check every installed file against `moondust.lock` (exit code 1 if anything is wrong) |
| `lua moondust.lua uninstall author/package` | Remove a package and its lock entry |

Logging in and publishing from the command line are coming soon. For now, publish on the website.

## Publishing a package

1. Put a `moondust.json` in the root of a public GitHub repo you own:

   ```json
   {
     "name": "tinyfiles",
     "version": "1.0.0",
     "description": "Tiny Lua library for reading and writing files",
     "main": "tinyfiles.lua",
     "lua": ["5.1", "5.3", "5.4"]
   }
   ```

   `main` is a single `.lua` file in the repo root (up to 512 KB). `lua` lists the versions it works on and defaults to `["5.3"]`. For LÖVE games include `"5.1"`. `description` is optional.

2. On the [homepage](https://thegamingmahi.github.io/Moondust/), log in with GitHub, claim your username (once, permanent), paste your repo, and press Publish.

Each version is pinned to your repo's latest commit at that moment. To release an update, bump `version` and publish again. A version can't be published twice. There's a small example in [`example-package/`](example-package/).

## How it works

- Packages are always named `username/package`. Usernames are claimed once, tied to a GitHub account, and can't be changed.
- The registry is a list on the [homepage](https://thegamingmahi.github.io/Moondust/) that says where each package lives and which exact commit to use.
- Moondust downloads the package's Lua file straight from GitHub at that commit, so the code can't change after it's published.
- A package is a GitHub repo with a `moondust.json` manifest in its root (name, version, main file, supported Lua versions). Publishing happens on the homepage.

## Requirements

- Lua 5.1 to 5.5 or LuaJIT. If a package only supports certain versions, you get a clear error instead of a confusing syntax error.
- `curl`, which is already on macOS, Linux and Windows 10 and later.
- LÖVE 12 or newer works too. It uses LÖVE's built-in https module instead of curl, and LÖVE runs LuaJIT, so packages need to list `5.1`. Call moondust from `love.load`, because downloads block.
- It won't work where neither `curl` nor LÖVE's https module is available, such as Roblox.

## Checksums and safety

- Every version is pinned to an exact commit, so a package can't change after it's published.
- `moondust.lock` also stores a SHA-256 checksum of each package it installs. If you download that version again (a fresh clone, a restored file), the download must match the checksum or it's refused. Commit `moondust.lock` to git so your whole team gets the same files.
- `lua moondust.lua verify` checks your installed files against the lock. Set `MOONDUST_VERIFY=1` to check on every load (slower).
- Checksums need `sha256sum`, `shasum`, `openssl` or `certutil` (all common), or LÖVE. Without one, Moondust warns and skips the check.
- Packages aren't reviewed. Moondust downloads and runs code from the internet, so check what you install, the same as with any package manager.

## Limits

Moondust is small on purpose. Right now:
- A package is one `.lua` file in the root of its repo. Several files, data files and dependencies are not supported yet.
- Pure Lua only. No C code and no build steps.
- Packages are published on the website, and only from repos you own personally (not organizations).
- The registry has no review process, and no checksum is stored there yet. The first download is trusted by its commit.
- There's no way to remove a package from the website yet. Report problems by opening an issue on this repo.

## Links

- Homepage and package browser: https://thegamingmahi.github.io/Moondust/
- Documentation: https://thegamingmahi.github.io/Moondust/docs.html

## License

MIT, see [LICENSE](LICENSE). The registry server source is in [`server/`](server/).
