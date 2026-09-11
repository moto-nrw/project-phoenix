# Environment and LSP audit

Historical baseline. The follow-up implementation is documented in
[development environment setup](../development-environment.md). The observations
below describe the machine before those fixes, not the current configuration.

Date: 2026-09-05. Scope: repository configuration, this agent's shell,
installed tools, selected local Zed/Claude settings and Zed logs, official docs.
No environment or editor settings changed.

## Verdict

Core tooling is installed, but editor and agent environments are not uniformly
reproducible. Basic Go LSP diagnostics and package definition lookup work.
Full editor diagnostics, TypeScript SDK selection, Next plugin activation,
and Claude LSP requests remain unverified.

## Observed local evidence

| Surface | Observation |
| --- | --- |
| Devbox | Installed Go 1.27.0 matches backend/go.mod; Node 24.12.0; pnpm pin 10.34.4 |
| Agent shell | Global Go 1.27.1 and Node 26.8.1; pnpm 10.34.4; direnv reports no environment loaded |
| Go LSP | Global gopls v0.20.0 built with Go 1.25.4; absent from devbox.json |
| TypeScript | frontend/node_modules has TypeScript 6.0.3 and Next 16.3.3; global typescript-language-server 6.0.0 |
| Claude | User settings enable gopls-lsp and typescript-lsp; both plugin caches and required binaries exist |
| Zed | Global language servers enabled; formatter is language_server; Biome auto-install enabled; no repository .zed/settings.json found |
| Zed logs | A Phoenix worktree used global gopls on August 31; Biome started in Phoenix paths; September 5 Git operations used Devbox Git |
| Frontend rules | Oxlint with custom date, logging and UI-kit plugins; Prettier with Tailwind plugin; Next TS plugin configured |
| Docker | Compose 5.5.0 available; daemon answered with 29.7.2 |

Zed's Devbox Git path is evidence that its environment can differ from this
agent's shell. The shell mismatch does not prove all Zed processes are wrong.
Biome startup does not prove that it formatted a TypeScript file incorrectly.

## Recommended work, in priority order

1. **Make tool selection consistent.** Keep commands under Devbox or the
   existing scripts/run-go-toolchain.sh wrapper. Launch Zed from the project
   Devbox shell and inspect the actual LSP process path. Zed passes CLI
   environment through to projects; GUI launches resolve project environments
   separately. [Zed environment](https://zed.dev/docs/environment)
2. **Pin language servers.** Add an appropriate maintained gopls version through
   Devbox, following repository tool policy. The current global copy is outside
   the lockfile. Also make Claude's TypeScript server provision reproducible.
   Distinguish gopls build Go, runtime Go, and source Go versions; a successful
   one-file check is not a compatibility certification.
   [Go guidance](https://go.dev/gopls/)
3. **Match editor lint and formatting to CI.** Configure this project to use
   local Oxlint diagnostics and Prettier, including its Tailwind plugin. Avoid
   Biome taking ownership of this project's formatting. Oxc provides a Zed
   extension using project-local oxlint. Verify a representative custom-rule
   diagnostic and format result before considering parity established.
   [Oxc editor setup](https://oxc.rs/docs/guide/usage/linter/editors.html)
4. **Verify the actual TypeScript SDK and Next plugin.** Zed defaults to vtsls,
   while Claude uses typescript-language-server. They are independent clients.
   Check that the opened frontend resolves its installed TypeScript 6.0.3 and
   loads the Next plugin, rather than assuming tsconfig alone proves this.
   [Zed TypeScript](https://zed.dev/docs/languages/typescript),
   [Next TypeScript](https://nextjs.org/docs/app/api-reference/config/typescript),
   [TypeScript server version notification](https://github.com/typescript-language-server/typescript-language-server#typescript-version-notification)
5. **Add a small onboarding health check.** Report resolved Go/Node/pnpm/LSP
   paths and versions, frontend dependency presence, and Docker availability.
   Do not print complete environments or secrets. Check a fresh worktree too.

## Easy-to-miss distinctions

- An enabled Claude plugin still needs its server binary in the Claude process
  PATH. Both prerequisites are present here; successful live requests were not
  observed. This Codex session exposes no dedicated LSP tool, so Claude's setup
  should not be treated as proof of Codex semantic navigation.
  [Claude LSP reference](https://code.claude.com/docs/en/plugins-reference#lsp-servers)
- Missing generated Next types in a fresh worktree can resemble an editor
  failure. Next documents dev, build, and typegen as generation paths. Verify
  installed command support before choosing the onboarding command.
  [Next generated types](https://nextjs.org/docs/app/api-reference/config/typescript#route-aware-type-helpers)
- Do not add a root go.work merely because backend/go.mod is nested. Modern
  gopls detects modules when files are opened. Add a workspace only for a real
  multi-module need; do not sweep architecture fixture modules into it.
  [Go workspace guidance](https://go.dev/gopls/workspace)

## Verification performed and limits

Ran installed version checks, direnv status, Docker version/daemon queries,
scripts/run-go-toolchain.sh go version, gopls check backend/main.go (exit 0,
no diagnostics), and gopls definition backend/main.go:1:9 (resolved the backend
module and package). Inspected manifests, editor settings and selected logs.

No application build, database health check, complete test suite, live editor
rename test, or TypeScript LSP handshake was performed. This is an environment
audit, not a certification that all application checks pass.
