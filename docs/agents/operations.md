# Operations and PR evidence

Read the relevant section for service commands, test-database maintenance,
deployment, or PR screenshots. Commands start at the repo root unless noted.

## Service commands

Use Docker Compose to run, build, migrate, and debug services. Host-side quality
and test commands below are intentional exceptions; Go uses the repo toolchain.
Add tools through `devbox search <tool>` / `devbox add <tool>@latest`, not a global install.

For editor/LSP setup, fresh-worktree dependencies or mismatched tool versions,
read [development environment](../development-environment.md). Run shell commands
through `devbox run` when the current process has not loaded the project environment.

| Task | Command |
|---|---|
| Start services | `docker compose up -d` |
| Rebuild backend after go.mod / Dockerfile changes | `docker compose build server && docker compose up -d server` (air reloads plain Go edits) |
| Migrate | `docker compose run server go run . migrate` |
| Reset local DB | `docker compose run server go run . migrate reset` (seed credentials: `docs/getting-started.md`) |
| Logs | `docker compose logs -f server` |
| Frontend quality | `cd frontend && pnpm run check` |
| Backend suite | `cd backend && ../scripts/run-go-toolchain.sh go test ./...` |
| Backend suite with immediate sweep | `scripts/run-go-toolchain.sh scripts/test-backend.sh` |
| Backend unit-only loop | `cd backend && ../scripts/run-go-toolchain.sh go test -short ./...` (skips DB tests) |
| Changed-code tests | `scripts/test-changed.sh origin/development` |
| Fast inner loop | `scripts/test-changed.sh --fast origin/development` (run without `--fast` before push) |
| Generate route docs | `docker compose run server go run . gendoc --routes` |

The seeder is **dev-only**. Staging/production infrastructure belongs in data
migrations or the admin UI, never the seeder. The one deployed exception is the
public demo environment (`APP_ENV=demo`, ADR 0029): `seed`, `simulate`,
`seed-parents`, and the seed-token exposure name `demo` in their allow-lists;
`staging` and `production` stay rejected. See Cleanup CLI below for cleanup
command shapes: some commands delete data and silently ignore extra arguments.

## Cleanup CLI

Run these inside the server container; use the documented dry-run shape before
a deletion. `cleanup visits` ignores extra arguments, so appending `preview`
does not make it a dry-run.

```bash
# CLI (run inside the container via `docker compose run server go run . <cmd>`)
go run . migrate status|validate|reset
go run . migrate backfill request-child-storage [--tenant ID] [--verify-only] [--restart]  # resumable #2713 copy; see docs/operations/enrollment-storage-backfill-2713.md
go run . seed --email <op-email> --password <pw> --pin 1234   # flags required; seeds via the HTTP API, server must be running
go run . cleanup preview|stats      # visit-retention dry-run / statistics
go run . cleanup visits             # REAL deletion — there is no `cleanup visits preview`; extra args are silently ignored
go run . cleanup timetable|time-tracking [preview|stats]      # nested dry-runs exist only for these two
go run . cleanup tokens|invitations|rate-limits|attendance|sessions|supervisors
go run . backfill staff-owner [status|reset]   # resumable users.staff → Membership/Workforce copy (#2752); exits 1 while unstable
go run . backfill student-owner [status|reset] # resumable users.students → People/Membership/Care Plan copy (#2758); exits 1 while unstable
go run . gendoc                     # Generates routes.md + docs/openapi.yaml
```

## Test database

Fixture rules: [backend test fixtures and lifecycle](backend-testing.md).
Tests start a postgres-test server for the configured `TEST_DB_PORT`, prepare
migration-hash templates, and clone a database per package. Package exit drops
its clone; generation GC cleans interrupted runs. The wrapper also sweeps at
exit. Worktrees using the same port share a server; different ports are isolated.

On macOS and Linux (including WSL), new managed test servers stop automatically
after 15 minutes without a protected test process, checked every 30 seconds.
The next test starts the server again. Containers and volumes are retained.
Each DB test binary holds a kernel lease before connecting, including the
`PHX_TEST_TEMPLATE` fast path. Exit, failure, Ctrl-C and SIGKILL release leases;
paused or CPU-only test processes remain protected. Cleanup and startup share
an exclusive/shared lock so a new test cannot connect during shutdown.

A content-addressed helper in the user's cache starts automatically using the
existing Go toolchain. It needs no cronjob, LaunchAgent or global installation
and continues after its originating worktree is removed. It uses the original
local Docker socket, not whichever context is selected later. If Docker discovery
is unavailable, database readiness and startup keep their existing behavior;
no new watcher is started. Saved registrations for the same test port still
receive process leases, so an existing watcher cannot stop a live test.
Registration and lease errors remain failures rather than bypassing protection.
CI, remote Docker engines and native Windows are unchanged. Use WSL for
Windows-local cleanup.

Inspect registered servers without stopping anything:

```bash
cd backend
../scripts/run-go-toolchain.sh go run ./internal/testdb/cmd/bootstrap --idle-status
```

Logs and registrations live under `moto-testdb/v1` in the OS user cache.
A healthy watcher exits after stopping its server; the next test starts a new
one. After a manually killed watcher, the next test process restores it.
Do not remove lock files while tests or watchers are running.

Only the `postgres-test` service in a `project-phoenix-testdb-<port>` project,
with label `de.moto.testdb.lifecycle=1` and matching published port, is eligible.
Legacy/unlabelled servers and application containers are not adopted or stopped.
Migrate an old test server only after its users finish; do not recreate it under
active tests. Apply the tracked Compose example when updating local Compose
files. Open SQL client connections also defer cleanup, but older binaries and
manual clients without leases cannot participate in the atomic startup lock.
Do not mix those clients with managed tests on the same port.

Manual clone maintenance (usually unnecessary):

```bash
cd backend && ../scripts/run-go-toolchain.sh go run ./internal/testdb/cmd/sweep
```

Do not use project-wide `docker compose down` to clean a test run: the legacy
`project-phoenix` project may also contain the development stack. Never prune
Docker volumes as part of automatic test-server cleanup.

## Environment Management (SOPS)

Edit `environments/{staging,production,demo}.sops.env` only through the SOPS CLI.
Never hand-edit ciphertext or deployed `.env` files over SSH. Share age private
keys through 1Password/Signal, never Slack/email.

1. `sops environments/staging.sops.env` decrypts into the editor and re-encrypts.
2. Push to `development` deploys staging; push to `main` deploys production.
   Demo deploys only through manual dispatch from `main`; see
   [demo environment](../operations/demo-environment.md) for host setup and ports.
3. CI decrypts and copies `.env`, compose, and `deploy-remote.sh` to the server.
4. Deployment pulls images, runs `migrate preflight`, backs up the DB, migrates,
   starts, and healthchecks; failures after the backup trigger rollback.

The preflight runs while the previous release is still serving and runs no
migration. It asks every pending migration that declares a `Precondition` whether
the data it needs is already in the shape it requires, so a migration that would
refuse on data somebody has to correct aborts the release with exit 1 — nothing
stopped, nothing restored — instead of failing mid-migration. A migration whose
refusal a human has to resolve registers that same check next to its `Up`.

| File | Purpose |
|---|---|
| `environments/{staging,production,demo}.sops.env` | Encrypted values, plaintext keys for sync checking |
| `environments/{staging,production,demo}.compose.yml` | GHCR images pinned to commit SHA |
| `.sops.yaml`, `scripts/sops-setup.sh` | Encryption config and age setup |
| `scripts/env-check.sh` | Key parity across deployed envs and `.env.example`; dev-only exceptions are declared in the script |
| `scripts/deploy-remote.sh` | Exit 0: success; 1: aborted before migration; 10: rollback succeeded; 11: rollback failed (critical) |

All encrypted files must have identical keys and match `.env.example` except
the script's dev-only whitelist. The shared `.env` supplies Compose interpolation
only. Each service receives an explicit environment allowlist; `migrate` alone
receives the privileged DSN. Read [runtime environment boundaries](../operations/runtime-environment-boundaries.md)
before changing deployment environments or maintenance jobs.

CI uses `SOPS_AGE_KEY`, `STAGING_SSH_*`, `PRODUCTION_SSH_*`, and `DEMO_SSH_*` secrets;
failure recipients are in the `DEPLOY_NOTIFY_EMAILS` repository variable.
Server layout is `~/{staging,production,demo}/` (`.env`, `docker-compose.yml`,
`.deploy-state`) and `~/backups/{env}/` (3 staging / 7 production / 3 demo complete
snapshot sets). See [complete release backup and rollback](../operations/release-backup-rollback.md)
for snapshot contents, verification, manual recovery and data-loss boundaries.
Rollbacks restore matching images, configuration, roles, database and uploads;
an old image alone is not compatible with every newer schema.
For env changes, read `.claude/rules/env-docker-sync.md` before editing.

## PR screenshots and QA evidence

Upload images and videos with `gh --attach` (gh >= 2.99, pinned in
`devbox.json`). GitHub hosts them as `https://github.com/user-attachments/assets/...`.

```bash
gh pr comment <NR> --body-file comment.md \
  --attach './shots/01-overview.png#Übersicht nach der Änderung' \
  --attach ./shots/02-detail.png
```

- Works the same on `gh pr create`, `gh issue create`, and `gh issue comment`.
  Up to 50 files per command; PNG, JPEG, GIF, WebP, SVG, MP4, MOV, WebM.
- Text after `#` is the alt text; without it gh uses the filename.
- An image in the body that points at the local path of an attached file is
  rewritten to the uploaded URL, so the body controls order and captions. Files the body
  does not reference are appended at the end.
- Copy files with spaces or `@` in the name to a plain filename first.
- Updating a series: rerun the same command with `--edit-last` so one comment
  holds the current series. It edits your newest comment on that PR or issue;
  check it is the screenshot comment first.
- After posting, read the comment body back and confirm every image points at
  `user-attachments/assets`.

Host QA images only through attachments: no releases, tags, Gists, or
screenshot branches.
