# Moondust

A tiny package manager for Lua. One file, no install step.

**Homepage and package browser: https://thegamingmahi.github.io/Moondust/**

> Early stage. The loader works, and the website and publishing are still being built.

## Quick start

1. Download `moondust.lua` and put it next to your script:

   ```
   curl -O https://thegamingmahi.github.io/Moondust/moondust.lua
   ```

2. Use any package with `require`:

   ```lua
   local moondust = require("moondust")

   local json = moondust("alice/json")
   print(json.encode({ hello = "world" }))
   ```

The first run downloads the package into a `lua_modules/` folder. After that it loads from disk.

## Versions

```lua
moondust("alice/json")          -- latest version
moondust("alice/json@1.0.0")    -- exact version
```

Moondust remembers which version "latest" meant in a `moondust.lock` file, so your code doesn't change under you. Run `lua moondust.lua update alice/json` to move to a newer one.

## Command line

The same file works as a command:

```
lua moondust.lua install alice/json
lua moondust.lua install alice/json@1.0.0
lua moondust.lua update alice/json
```

Logging in and publishing from the command line are coming soon.

## Publishing a package

1. Put a `moondust.json` in the root of a public GitHub repo you own:

   ```json
   {
     "name": "json",
     "version": "1.0.0",
     "description": "Small JSON encoder/decoder",
     "main": "json.lua",
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
- Pure Lua packages only. No C code or build steps.
- LÖVE 12 or newer works too. It uses LÖVE's built-in https module instead of curl, and LÖVE runs LuaJIT, so packages need to list `5.1`. Call moondust from `love.load`, because downloads block.
- It won't work where neither `curl` nor LÖVE's https module is available, such as Roblox.

## Safety

Moondust downloads and runs code from the internet. Pinning to a commit stops a package from changing later, but packages aren't reviewed. Check what you install, the same as with any package manager.

## Links

- Homepage and package browser: https://thegamingmahi.github.io/Moondust/

## License

MIT, see [LICENSE](LICENSE). The registry server is in [`server/`](server/), with setup steps in [`server/DEPLOY.md`](server/DEPLOY.md).
