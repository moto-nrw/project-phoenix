# Development environment and editor setup

## Start a checkout or worktree

1. Run `devbox run bootstrap` from the repository root. It installs the frozen
   frontend dependencies and generates Next route types without runtime secrets.
   The browser-automation CLI finds an installed Google Chrome by itself; on a
   machine without one, run `agent-browser install` once (it installs per user
   in `~/.agent-browser`, not per checkout).
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

A 16 GB laptop has little headroom for several dev processes. Earlier
measurements put Next dev at 2.5-4.6 GB, a heavy Go test link at about
1.1 GB, gopls at about 1 GB and tsserver at about 2 GB. The reference Go
suite below used four package workers and peaked near 10 GB; it predates
the current two-worker budget and later architecture-test changes.

### Use the smallest feedback loop

1. Start only the app you need: `scripts/dev-native.sh up backend` or
   `scripts/dev-native.sh up frontend`. Bare `up` still starts both. Each
   `up` replaces the previously managed app processes. Shared Postgres and
   Mailpit stay running; frontend-only startup skips Go builds and migrations.
   API calls from the frontend still need a running backend.
2. During a fix, run a named test/package or
   `scripts/test-changed.sh --fast origin/development`. The fast mode omits
   transitive importers and repository-wide ratchets. Run without `--fast`
   before pushing; it is not a replacement for the full gate.
3. Run only one full backend suite or whole-repository analysis at a time
   across your worktrees. Each command has its own budget, not a shared
   machine-wide limit. Extra language-server sessions also keep their own
   copies of the project graph in RAM.

### Go CPU and package budgets

Devbox, `scripts/run-go-toolchain.sh`, the native backend launcher and the
pre-push runner default to `GOFLAGS=-p=4` and `GOMAXPROCS=4`. The pinned
runner also supplies these values outside a Devbox shell and passes them
to nested commands. Explicit caller values remain intact.

The two controls cover different work: [`-p`](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies)
limits concurrent package commands, while
[`GOMAXPROCS`](https://pkg.go.dev/runtime#GOMAXPROCS) limits Go CPU execution
within each process. `deadcode` uses the latter for type checking and SSA
construction, so `-p` alone does not bound its analysis load. Neither is a
hard CPU limit on an entire process tree.

Full and changed backend tests share `scripts/go-package-workers.sh`:
half the available CPUs, one worker per 8 GiB RAM, at most two on macOS
or four on Linux, and at least one. A 16 GiB machine therefore gets at
most two package workers. `-parallel 8` stays fixed for the test-result
cache and database-pool sizing. Full-suite callers can still override
the package budget with an explicit `-p` argument.

To reduce analysis load further for a local run, use, for example,
`GOMAXPROCS=2 scripts/run-go-toolchain.sh scripts/check-deadcode.sh`.
Lower concurrency can make an isolated command slower while leaving more
CPU available for the editor and hot reload. It does not remove the
memory needed to hold the analysis graph.

#### Measured analysis tradeoff

On 2026-10-14, backend commit `af3bfe5f97`, a 64 GiB / 16-core Mac ran
three production `go tool deadcode ./...` passes per setting, sequentially
in alternating order with `GOFLAGS=-p=4`. Other work remained active;
these are medians, not an idle-machine benchmark.

| Setting | Wall time | CPU time | Sampled peak process-tree CPU | Sampled peak process-tree RSS |
| --- | --- | --- | --- | --- |
| `GOMAXPROCS=16` | 5.25 s | 21.73 s | 1004% | 3690 MiB |
| `GOMAXPROCS=4` | 6.81 s | 16.28 s | 385% | 3786 MiB |

All six passes produced identical findings. Four threads reduced sampled
peak CPU by about 62% and CPU time by about 25%, at the cost of about
30% more wall time. RSS did not improve in this run. CPU/RSS were sampled
every 250 ms, so short spikes can be missed. This does not measure the full
test suite, hot-reload latency under load or a real 16 GiB machine.

The deadcode gate still needs separate production and test-inclusive graphs:
tests can make a production-unreachable function reachable. Neither mode is
removed. The architecture and affected-package improvements below remove
redundant work while retaining their existing checks.

### Faster architecture and affected-package checks

The architecture gate now avoids three repeated costs:

1. All own `./...` packages retain typed syntax and function-body checking.
   External dependencies supply compiler-exported types instead of source
   syntax and type-info maps. Imported generics and promoted methods retain
   their types.
2. Semantic traversal keeps an ancestor stack instead of building a parent
   map for every AST node. Receiver-chain and diagnostic-location behavior
   stays unchanged.
3. Policy strictness comparison reuses pure rule decisions for identical
   scope/owner/role points within that invocation. Base and candidate policies
   have separate maps. Reviewed path-specific exceptions still run for every
   edge; rule-use and stale-rule checks are unchanged.

Affected-package selection uses one `go list -test` invocation for ordinary
packages and test variants instead of loading those graphs separately. It
retains transitive and test-only importers, including real packages whose
names end in `.test`.

On 2026-10-14, a 64 GiB / 16-core Mac ran three before/after passes per
command, sequentially in alternating order, with warm Go caches and the
same `GOFLAGS=-p=4` / `GOMAXPROCS=4`. The full architecture check used
base commit `ac58fb4562`, including policy strictness, legacy baseline and
composition comparison. Other work remained active; these are medians.

| Check | Wall time before / after | CPU time before / after | Sampled peak process-tree RSS before / after |
| --- | --- | --- | --- |
| Full architecture gate | 20.74 / 13.27 s | 31.21 / 21.70 s | 2405 / 1897 MiB |
| Affected-package selection | 3.26 / 2.15 s | 4.32 / 2.80 s | Not sampled |

The full gate used about 36% less wall time, 30% less CPU time and 21% less
sampled peak RSS. All six runs produced byte-identical output: 55 legacy
violations and 252 composition targets. A separate raw-check comparison
also matched all 503 diagnostic lines, including physical source locations.
Affected-package output was byte-identical in all six selection runs.

The real `TestCompositionLegacyCallerInventory` semantic-load profile reduced
cumulative allocations from about 3.52 GiB to 1.86 GiB (47%). That is total
allocation volume, not peak resident memory. RSS sampling at 250 ms can miss
short spikes. Compiler export data uses Go's normal build cache; a cold first
run may need to compile missing exports. These measurements cover repeated
feedback, not a clean install, the full test suite or actual 16 GiB hardware.

### Reuse within an architecture check

A follow-up removes three more sources of repeated work within one invocation:

1. The CLI reuses its loaded candidate package graph in policy comparison.
   The standalone comparison API still loads and validates its own graph.
2. Composition snapshots share parsed syntax only for the same relative path
   with byte-identical source. Every candidate file is still read, including
   build-excluded files. Changed and new files are parsed; each snapshot keeps
   its own module, imports, type resolution and inventory.
3. Contract analysis reuses completed value/pointer method sets for the exact
   `go/types.Named` instance. Generic instantiations remain distinct. Findings,
   diagnostic targets and recursive traversal guards are evaluated each time.

All reuse ends with the invocation. There is no persistent cache, host-wide
queue, policy relaxation, new dependency or removed check.
The existing architecture command and hooks use this path automatically.

On 2026-10-14, the same 64 GiB / 16-core Mac compared binaries built from
`b1fafeb0b8ea7a37c95fafd7f87423c8024f87da` with isolated Go build overlays for
each candidate. Every binary checked the same frozen working tree against
that base, with warm caches, pinned Go 1.27.3, `GOFLAGS=-p=4` and
`GOMAXPROCS=4`. Runs were sequential in forward/reverse order; the table gives
three-run medians, excluding the initial warm-up.

| Implementation | Wall time | CPU time | Sampled peak process-tree RSS |
| --- | --- | --- | --- |
| Before | 11.32 s | 18.88 s | 1940 MiB |
| Package-graph reuse only | 10.09 s | 17.27 s | 1923 MiB |
| Syntax reuse only | 10.37 s | 17.92 s | 1572 MiB |
| Method-set reuse only | 11.58 s | 19.47 s | 1861 MiB |
| All three | 9.23 s | 16.12 s | 1521 MiB |

Together this reduced wall time by 18%, CPU time by 15% and sampled peak RSS
by 22%. All fifteen measured outputs were byte-identical: 46 legacy
violations and 252 composition targets. These numbers cover the architecture
gate, not app runtime, the full test suite or a real 16 GiB machine. Other
work remained active; RSS was sampled every 250 ms and may miss short spikes.
The raw check also matched all 347 diagnostic lines, including source locations.

The method-set-only series did not establish a time improvement. A separate
three-pair comparison with graph and syntax reuse already enabled measured
9.98 / 9.23 s wall time and 16.71 / 16.01 s CPU time without / with method-set
reuse. Its sampled RSS was 1458 / 1529 MiB, so retaining completed sets has a
memory tradeoff. A full-GC allocation profile reduced cumulative
`types.NewMethodSet` allocations from about 212 MiB to 4 MiB. The combined
profile reduced total allocations from 4.61 GiB to 3.64 GiB (21%). Allocation
volume is not peak resident memory.

### Deadcode experiment not adopted

The fourth candidate profiled the pinned `golang.org/x/tools` v0.51.0 tool
and tried `GOGC=50` against the default, without changing either analysis
mode. Three runs per mode/setting used the same frozen tree and pinned
toolchain, sequentially in alternating order. Each mode's findings were
byte-identical across settings. Medians:

| Mode / GC setting | Wall time | CPU time | Sampled peak process-tree RSS |
| --- | --- | --- | --- |
| Production / default | 6.17 s | 15.24 s | 3896 MiB |
| Production / `GOGC=50` | 6.95 s | 19.66 s | 3215 MiB |
| Including tests / default | 13.43 s | 31.82 s | 6996 MiB |
| Including tests / `GOGC=50` | 15.22 s | 40.82 s | 6113 MiB |

More frequent GC saved about 13–17% peak RSS, but added about 13% wall time
and 28–29% CPU time. It does not meet the goal of less work and faster
feedback, so no GC default or deadcode command was changed. The profile
points to whole-program type checking, SSA construction and reachability
analysis inside the external tool; no safe repo-local algorithm reduction
was identified. Production and test-inclusive passes remain separate because
test entry points can hide production-unreachable functions.

### Fresh Postgres volumes

The dev template gives Postgres a 60-second healthcheck start period and
checks TCP readiness. Previously two failed one-second checks could abort
`docker compose up --wait` during `initdb`; this occurred in the native
startup check on this machine. The image's temporary initialization server accepts
only Unix sockets, so socket readiness is not enough for the app.
See the [Postgres image entrypoint](https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh)
and [Docker healthcheck semantics](https://docs.docker.com/reference/dockerfile/#healthcheck).
A successful check still marks the service ready before the start period
ends. Existing local `docker-compose.yml` copies need the same healthcheck
update; changing the template does not rewrite them.
A fresh-volume check after the change reached TCP-ready health in 7.0 seconds
on the same Mac. The disposable check removed only its own volume afterwards.

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
   For backend-only work, use `scripts/dev-native.sh up backend` to avoid
   keeping Next dev in memory.
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
