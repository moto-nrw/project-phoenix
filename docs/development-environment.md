# Development environment and editor setup

## Start a checkout or worktree

1. Run `devbox run bootstrap` from the repository root. It installs the frozen
   frontend dependencies, generates Next route types without runtime secrets,
   and installs the browser used by the pinned browser-automation CLI.
2. Run `devbox run doctor`. It checks resolved tool paths, Go module agreement,
   frontend dependencies, generated types and Docker. It never prints env files.
3. Run `devbox run check-lsp` after updating language servers or editor settings.
   It starts the real servers and checks Go/TypeScript definitions and errors,
   Next's server-component rule, a custom Oxlint rule and Tailwind formatting.
   It creates disposable source files and removes them at exit. Run it when
   no build or test watcher is active, since those files deliberately contain errors.

A worktree created with `wt add` runs step 1 itself and copies the local env
files, Compose file and certificates from the main checkout with its own
ports; run `wt doctor` inside it when something is missing. To run backend
and frontend on the host instead of in Compose, use `devbox run dev up`
(`scripts/dev-native.sh`, see `docs/agents/operations.md`).

`devbox run` works without a shell's direnv hook. For an interactive shell,
use `devbox shell`, or enable direnv's shell hook and run `direnv allow`.
After tool updates, reopen editors and agent sessions; existing processes keep
their old environment. No root `go.work` is needed for normal backend navigation.
Architecture fixture modules must remain independent.

## Zed

1. Install **Oxc** from Zed's extension marketplace once.
2. Open the repository root with `devbox run editor`.
3. Open a Go file and a frontend TSX file. Use **Go to Definition** and inspect
   **zed: open log** if either server fails to start.

The committed `.zed/settings.json` selects the pinned Go and TypeScript servers,
Oxlint and project-local Prettier. It excludes Biome/Oxfmt/vtsls for JS/TS, so
personal extensions cannot replace this repo's lint and formatting pipeline.
The TypeScript SDK path points at frontend dependencies, not a server's bundled SDK.
The launcher anchors paths to this checkout and pins the child Go environment,
even when the GUI starts without Devbox. Bootstrap must have run first.

Open this repository as its own project, not only its parent directory. Relative
server paths are rooted at the opened project. Other editors can use the same
launcher and TypeScript initialization options; VS Code users should select
the SDK under `frontend/node_modules/typescript/lib` and use Oxc plus Prettier.

## Claude Code and Codex

Use `devbox run claude` or `devbox run codex` from the root. These keep child
commands in Devbox. The Claude launcher prefers the native installer under
the user's home over old global installations; clients remain vendor-updated.

Codex's project configuration uses non-login shells: host login profiles can
replace the inherited Devbox PATH with Homebrew paths. Start a new session after
changing this setting. Desktop sessions launched without Devbox still need
explicit `devbox run` commands; disabling login profiles does not install tools
or activate Devbox by itself.

Claude's project settings register the small local `moto-lsp` plugin, which
selects these same tools and SDK. Accept the normal project/plugin trust prompt
when first opening a new checkout. The generic official Go/TypeScript plugins
are disabled **only in this repo** to avoid two servers claiming the same files.
Check `/plugin` for `moto-lsp@moto-local` and restart after configuration changes.
Local marketplace paths resolve against the main checkout even in worktrees;
the plugin's tool commands resolve against the active `CLAUDE_PROJECT_DIR`.

Codex does not inherit Claude plugins. Whether a session exposes semantic LSP
tools depends on its runtime; inspect its actual tool list. In a shell-only
session, use the pinned `gopls` CLI for Go navigation and `devbox run check-lsp`
to check server health. Do not claim TypeScript semantic navigation from grep
results. Commands launched from the Codex desktop can also use `devbox run`
explicitly; a terminal's direnv activation does not configure another process.

## Low-memory machines

On a 16 GB laptop RAM runs out before the CPU does. Next dev alone holds
about 2.5 GB, a Go link about 1.3 GB, and the IDE with gopls and tsserver
several more. Measured on the reference machine:

| Step | Before | After |
|---|---|---|
| `pnpm run typecheck`, cold | 25 s, 3.0 GB | 2.9 s (TypeScript 7 native) |
| `pnpm run typecheck`, warm | 3.7 s | 1.0 s |
| Backend start (`dev-native.sh up`) | `go run` + air: two links | one link, air reuses the cache |
| Parallel Go test binaries, 16 GB | up to 8 (`test-changed`) / 10 (`test-backend`) | 4 (one per 4 GB) |

What keeps a 16 GB machine out of swap:

1. Run the app natively: `scripts/dev-native.sh up`. `docker compose up -d`
   starts only `postgres` and `mailpit`; the full container stack needs
   `--profile full`. Next dev and Go builds in the Docker VM cost the VM's
   overhead on top and compile through slow bind mounts.
   `docker-compose.yml` is a local copy: refresh it once with
   `cp docker-compose.example.yml docker-compose.yml` in the main checkout
   (new `wt` worktrees copy it from there).
2. macOS: use [OrbStack](https://orbstack.dev) or Colima instead of Docker
   Desktop. Docker Desktop reserves its VM memory up front (1.5-3 GB idle);
   OrbStack returns unused memory to macOS. With Docker Desktop, set
   Settings > Resources > Memory to 4 GB; the infrastructure services have
   `mem_limit`s that fit.
3. Windows: work inside WSL2 with the repository in the Linux filesystem
   (`~/…`, not `/mnt/c/…`, which is several times slower for Go and Node).
   Cap the WSL VM in `%UserProfile%\.wslconfig`:

   ```ini
   [wsl2]
   memory=10GB
   swap=8GB
   ```

   and set `autoMemoryReclaim=gradual` under `[experimental]` so the VM hands
   freed memory back to Windows. Run `wsl --shutdown` afterwards.
4. Run one dev loop at a time. Every running `wt` worktree adds its own
   Postgres, Next dev (2.5 GB) and air. `scripts/dev-native.sh down` and
   `docker compose stop` in worktrees you are not using.
5. Close the second editor. VS Code/Cursor and Zed each start their own
   gopls and tsserver.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Works in terminal but not editor | Actual server path in Zed logs; reopen via Devbox |
| Next-specific diagnostics absent | Workspace TypeScript and Next plugin, not just tsconfig presence |
| Missing route helpers in a new worktree | Run bootstrap/type generation before tsc |
| Editor disagrees with CI | Local Oxlint configuration and Prettier Tailwind plugin; run frontend check |
| Claude plugin enabled but unavailable | Client version, `/plugin` errors and bootstrap in the active checkout |

Official references: [Zed environments](https://zed.dev/docs/environment),
[Zed TypeScript](https://zed.dev/docs/languages/typescript),
[Oxc editor setup](https://oxc.rs/docs/guide/usage/linter/editors.html),
[Next TypeScript](https://nextjs.org/docs/app/api-reference/config/typescript),
[Go workspaces](https://go.dev/gopls/workspace),
[Claude LSP configuration](https://code.claude.com/docs/en/plugins-reference#lsp-servers).
