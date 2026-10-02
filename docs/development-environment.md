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

On a 16 GB laptop RAM runs out before the CPU does. Next dev holds 2.5-4.6 GB,
a heavy Go test link about 1.1 GB, gopls about 1 GB and tsserver about 2 GB.
The full backend suite (`scripts/test-backend.sh`) peaks near 10 GB of
process memory even with the parallelism a 16 GB machine gets.

### Measure your machine

`scripts/dev-bench.sh` records wall time, CPU time and the peak memory of each
step's whole process tree:

1. Stop a running dev loop (`scripts/dev-native.sh down`) and close what you
   do not need. Docker must be running; the Go tests and the dev loop start
   their databases themselves.
2. Run `scripts/dev-bench.sh` from the checkout. The default stages are
   `typecheck vitest go-tests devloop-native` and take about 6 minutes on the
   reference Mac; name stages to run fewer, for example
   `scripts/dev-bench.sh typecheck go-tests`.
3. Send the printed table (or `tmp/dev-bench/<timestamp>.tsv`) together with
   the two header lines (commit, CPU count, RAM) and what else was running.

`--root DIR` runs the stages against another checkout for before/after
comparisons. `--simulate-ram-gb 16` makes the test scripts pick a 16 GB
machine's parallelism on a larger machine. Run `devloop-docker` only from the
main checkout; it does not work inside `wt` worktrees (see the script header).

Reference numbers from `scripts/dev-bench.sh`, same 64 GB Mac with other work
running (before: commit `384bb24562`, after: `1f5e74e781`):

| Step | Before | After |
|---|---|---|
| `pnpm run typecheck`, cold | 26.1 s, 2.8 GB | 3.2 s, 2.8 GB (TypeScript 7 native) |
| `pnpm run typecheck`, warm | 4.8 s, 1.6 GB | 1.1 s, 0.9 GB |
| Full Vitest suite | 206 s, 962 s CPU, 2.7 GB | 114 s, 578 s CPU, 2.3 GB |
| Full Go suite (`test-backend.sh`) | 265 s, 1146 s CPU at `-p 10`, peak 11.3 GB | 226 s, 1097 s CPU at `-p 4` (16 GB simulated), peak 10.2 GB |
| Go hot reload (`dev-native.sh`) | 4.8 s | 3.2 s |

Go suite times are wall time including the test-database bootstrap;
gotestsum itself reported 245 s and 203 s. The old commit's run also had 191
failures, nearly all `i/o timeout` connecting to the shared test Postgres, so
its time is not a clean baseline.

### macOS: skip the malware scan of fresh test binaries

macOS checks every newly linked executable (XProtect) on its first start.
A fresh 118 MB `backend/api` test binary took 1.0-1.2 s to start the first
time and 0.03-0.09 s afterwards (6 binaries, load average about 58). An
earlier run at lower load measured 0.6-0.76 s. A full backend run starts
about 65 such heavy binaries.

1. Run `spctl developer-mode enable-terminal`. It adds Terminal to
   **System Settings > Privacy & Security > Developer Tools**.
2. In that pane, switch on the app you run tests from. Add other terminals
   (Ghostty, iTerm) or editors with **+**.
3. Restart that app.

Programs started from an app on that list skip these checks, so only add apps
you develop in. Not verified: the before/after timing with the setting
switched on, because it can only be changed interactively.

### Editor language servers

Measured headlessly on the 64 GB Mac (load average 15-110 from parallel
work): server start at the repository root, open two files, one Go to
Definition, one Find References, then 15 s idle. Values are the peak memory
of the server's process tree in three interleaved runs, sorted.

| Server | Setting | Peak memory | Result |
|---|---|---|---|
| gopls 0.23 | default | 0.79 / 1.26 / 1.71 GB | |
| gopls 0.23 | `GOMEMLIMIT=1GiB` | 0.95 / 1.04 / 1.09 GB | adopted |
| gopls 0.23 | `GOMEMLIMIT=1536MiB` | 1.04 / 1.29 / 1.32 GB | |
| gopls 0.23 | `build.directoryFilters` (`-frontend`, `-tmp`, `-.devbox`, testdata and tool modules) | 1.38 / 1.39 / 1.76 GB | no gain, not adopted |
| TypeScript 6 tsserver (Next plugin) | default | 2.04 / 2.24 / 2.25 GB | default |
| TypeScript 7 `tsc --lsp` | opt-in | 1.14 / 1.17 / 1.29 GB | opt-in |

`scripts/editor-tool.sh` starts gopls with `GOMEMLIMIT=1GiB` unless the
environment sets another value, so Zed, the Claude `moto-lsp` plugin and
`devbox run check-lsp` all use it. Definitions and the 1525 references were
identical, and gopls CPU time stayed at 11-14 s with and without it. It is a
soft limit: if the workspace outgrows it, gopls collects garbage more often
instead of failing.
gopls already ignores `node_modules` and only loads the modules of files
you open, so directory filters change nothing measurable.

The TypeScript 7 server reached its first diagnostics in 1-4 s instead of
7-24 s and returned the same definition and JSX references. It omits import
lines and the export declaration from Find References. It has no plugin API, so Next's TypeScript plugin
does not run: no editor diagnostics for client hooks in server components,
invalid page/layout exports, wrong segment config values or `metadata` misuse,
and no completions for those options. Next's compiler and `next build` still
catch most of these (not measured here), but only after a compile. `check-lsp` tests the
Next server-component diagnostic, so TypeScript 7 stays opt-in. To try it,
change the `typescript-language-server` binary arguments in your local
`.zed/settings.json`, or the `args` of the `typescript` entry in
`.claude/plugins/moto-lsp/.claude-plugin/plugin.json`, to
`["typescript-native"]`, and do not commit that change.

### Keep a 16 GB machine out of swap

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
