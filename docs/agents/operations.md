# Operations and PR evidence

Read the relevant section for service commands, test-database maintenance,
deployment, or PR screenshots. Commands start at the repo root unless noted.

## Service commands

When a task needs a running UI or API, use the smallest native mode below:
`postgres` and `mailpit` run in Compose, the selected apps natively. Tests start
their own test database and need neither app server. Use the full container
stack for explicit container work; `server` and `frontend` otherwise stay behind
the `full` Compose profile.
Go uses the repo toolchain.
Add tools through `devbox search <tool>` / `devbox add <tool>@latest`, not a global install.

For editor/LSP setup, fresh-worktree dependencies or mismatched tool versions,
read [development environment](../development-environment.md). Run shell commands
through `devbox run` when the current process has not loaded the project environment.

| Task | Command |
|---|---|
| Start infra + backend + frontend | `scripts/dev-native.sh up` |
| Start infra + backend only | `scripts/dev-native.sh up backend` |
| Start infra + frontend only | `scripts/dev-native.sh up frontend` |
| After go.mod changes | restart `scripts/dev-native.sh up` (air reloads plain Go edits) |
| Migrate | `scripts/dev-native.sh backend go run . migrate` |
| Reset local DB | `scripts/dev-native.sh backend go run . migrate reset` (seed credentials: `docs/getting-started.md`) |
| Logs | `tmp/dev-native/backend.log`, `tmp/dev-native/frontend.log` |
| Full container stack | `docker compose --profile full up -d`, logs via `docker compose logs -f server` |
| Frontend quality | `cd frontend && pnpm run check` |
| Frontend E2E flows | seeded native stack, then `scripts/dev-native.sh frontend pnpm run test:e2e` |
| Backend suite | `cd backend && ../scripts/run-go-toolchain.sh go test ./...` |
| Backend suite with immediate sweep | `scripts/run-go-toolchain.sh scripts/test-backend.sh` |
| Backend unit-only loop | `cd backend && ../scripts/run-go-toolchain.sh go test -short ./...` (skips DB tests) |
| Changed-code tests | `scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development` |
| Fast inner loop | `scripts/run-go-toolchain.sh scripts/test-changed.sh --fast origin/development` (run without `--fast` before push) |
| Generate route docs | `scripts/dev-native.sh backend go run . gendoc --routes` |
| Standalone background jobs | `scripts/dev-native.sh backend go run . worker` with a free `PORT`; it waits in standby while `serve` leads (lease, cutover and rollback: [worker lease runbook](../operations/worker-lease-2726.md)) |

### Native dev loop

`scripts/dev-native.sh` (or `devbox run dev <command>`) runs the backend
(`air`) and frontend (`next dev`) on the host while `postgres` and `mailpit`
stay in Compose. Both processes read the root `.env` that Compose uses; only
Compose hostnames (`DB_DSN`, `EMAIL_SMTP_*`, `MAILPIT_URL`, `API_URL`, `PORT`)
are rewritten to localhost with the published ports. Nothing else changes, so
`.env` stays the single source and the env-sync rule applies unchanged.

| Task | Command |
|---|---|
| Start infra + backend + frontend (foreground) | `scripts/dev-native.sh up` |
| Start infra + backend only (foreground) | `scripts/dev-native.sh up backend` |
| Start infra + frontend only (foreground) | `scripts/dev-native.sh up frontend` |
| Stop native processes (infra keeps running) | `scripts/dev-native.sh down` |
| Status | `scripts/dev-native.sh status` |
| Backend CLI with the native env | `scripts/dev-native.sh backend go run . migrate` |
| Frontend command with the native env | `scripts/dev-native.sh frontend pnpm run check` |
| Back to the full Compose stack | `scripts/dev-native.sh docker` |

Logs land in `tmp/dev-native/backend.log` and `tmp/dev-native/frontend.log`.
`up` stops the `server` and `frontend` containers first and refuses to start
when a selected app's published port is taken. Each `up` replaces both
previously managed native app processes, then starts only the selected apps.
All modes keep Postgres and Mailpit; frontend-only mode skips Go and migrations
and expects a separately running backend for API calls. Backend children default
to `GOFLAGS=-p=4` and `GOMAXPROCS=4`, preserving explicit caller values.
After `go.mod` changes restart `up`; plain Go
edits reload through air. Worktrees created with `wt` carry their own ports in
the copied `.env`, so several native loops run side by side; on 16 GB
machines run one at a time ([low-memory machines](../development-environment.md#low-memory-machines)).

### Worktrees

`wt add <issue|branch>` creates a worktree with copied local files (`.env`,
`backend/dev.env`, `frontend/.env.local`, `docker-compose.yml`, the Postgres
certificates), rewritten host ports, an isolated Compose project, and the
frozen frontend dependencies; see `.wtconfig`. `wt doctor` inside a worktree
lists ignored files the worktree lacks and leased ports that the shell or
`.envrc` shadows. `wt remove` tears down the worktree's Compose project and
its `project-phoenix-testdb-<port>` test server, including volumes.

The seeder is **dev-only**. Staging/production infrastructure belongs in data
migrations or the admin UI, never the seeder. The one deployed exception is the
public demo environment (`APP_ENV=demo`, ADR 0029): `seed`, `simulate`,
`seed-parents`, and the seed-token exposure name `demo` in their allow-lists;
`staging` and `production` stay rejected. `seed`, `simulate` and `demo` live
in the dev tools binary `cmd/devtools` (ADR 0060): run them with
`go run ./cmd/devtools <cmd>`. The production image does not contain it; only
the demo image does. See Cleanup CLI below for cleanup
command shapes: some commands delete data and silently ignore extra arguments.

Seed and simulation commands keep refresh sessions in `.seed-sessions/` in
their working directory (normally `backend/`, gitignored, private files).
Separate commands renew the verified MFA session and switch schools through
the regular API. Expired or revoked proof falls back to a real login using
Mailpit. Delete that directory to force fresh logins; it is local credential
state and must not be uploaded. Keep the same working directory between seed
and simulation commands.

## Cleanup CLI

Run these inside the server container; use the documented dry-run shape before
a deletion. `cleanup visits` ignores extra arguments, so appending `preview`
does not make it a dry-run.

```bash
# CLI (run inside the container via `docker compose run server go run . <cmd>`)
go run . migrate status|validate|reset
go run ./cmd/devtools seed --email <op-email> --password <pw> --pin 1234   # flags required; seeds via the HTTP API, server must be running
go run . cleanup preview|stats      # visit-retention dry-run / statistics
go run . cleanup visits             # REAL deletion — there is no `cleanup visits preview`; extra args are silently ignored
go run . cleanup timetable|time-tracking [preview|stats]      # nested dry-runs exist only for these two
go run . cleanup tokens|invitations|rate-limits|attendance|sessions|supervisors
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
3. CI decrypts and copies `.env` and compose to `~/<environment>/` and the release
   scripts to `~/scripts/<environment>/`. Staging and production deploy
   concurrently on one host, so they never share a script directory; demo runs
   on its own VM with the same layout.
4. Deployment pulls images, runs `migrate preflight`, backs up the DB, migrates,
   starts, and healthchecks; failures after the backup trigger rollback. A
   healthy release finally prunes unused images; a failed prune only warns.

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
failure recipients are in the `DEPLOY_NOTIFY_EMAILS` repository variable. The
PostHog dashboards use `POSTHOG_READ_API_KEY` (repository) and
`POSTHOG_PERSONAL_API_KEY` (environment `posthog`); see
[Nutzungsanalyse](../operations/nutzungsanalyse.md#dashboards-3604).
Server layout on each host is `~/{staging,production,demo}/` (`.env`, `docker-compose.yml`,
`.deploy-state`) and `~/backups/{env}/` (3 staging / 7 production / 3 demo complete
snapshot sets). See [complete release backup and rollback](../operations/release-backup-rollback.md)
for snapshot contents, verification, manual recovery and data-loss boundaries.
Rollbacks restore matching images, configuration, roles, database and uploads;
an old image alone is not compatible with every newer schema.
For env changes, read `.claude/rules/env-docker-sync.md` before editing.

## GitGuardian findings

The hosted **GitGuardian Security Checks** GitHub App scans every commit in a
PR, not just its final diff. Editing or deleting a literal in a later commit
does not remove its earlier occurrence. Local configuration does not change
its detectors or clear its historical incidents.

### Local prevention

Lefthook runs the pinned gitleaks scanner through `scripts/check-secrets.sh`.
It works offline and needs no account or token; `devbox install` installs it.

1. Pre-commit scans the staged changes. Pre-push scans every commit from the
   merge-base with the comparison branch through HEAD. Both redact secrets and
   fail on findings or a missing scanner. Pre-push never uses its success
   cache to skip secret scanning.
2. `.gitleaks.toml` extends the gitleaks default rules. It skips test files
   (`*_test.go`, `*.test.ts(x)`, `testdata/`, `*test/` directories) so
   throwaway fixture credentials do not block commits.
3. For a verified false positive elsewhere, add a `gitleaks:allow` comment to
   the line or its fingerprint from the scan output to `.gitleaksignore`.

### Existing hosted findings

For a failing hosted check:

1. Open the check's findings table. Inspect the flagged commit, file, callers,
   and credential use. A test filename alone does not prove a value is safe.
2. For a verified test-only credential, open its linked GitGuardian incident
   and choose **Ignore → Test credential**. For a detector match that is not a
   credential, choose **Ignore → False positive**. Closed incidents no longer
   appear in check runs. Keep real credentials on the revoke/rotate path.
3. Re-run the check and verify it succeeds. If it still fails, inspect the new
   findings rather than assuming the same incident caused it.

A **Neutral** result with **Pull request too large to scan** means GitGuardian
skipped scanning, not that it found no secrets. Report this coverage gap; do not
describe the rerun as a clean scan.

For backend password fixtures, use `testpkg.NewTestPassword()`; the
[fixture guide](backend-testing.md#credential-fixtures) distinguishes generated
credentials, mapping placeholders and deliberately invalid input. Rewriting
shared history is not needed to classify an intentional test credential.

References: [hosted check behavior and incident handling](https://docs.gitguardian.com/internal-monitoring/prevent/detect-secrets-in-real-time-in-github),
[gitleaks configuration](https://github.com/gitleaks/gitleaks#configuration),
[dashboard exclusion settings](https://docs.gitguardian.com/secrets-detection/customize-detection/exclusion-rules).

## Rollback window of a storage cutover

A storage cutover leaves the old shape in place as a compatibility mirror so the
previous image can be deployed again. The period until the matching Contract
ticket drops that mirror is the rollback window. **It is not a waiting period,
and it has no fixed length.** Sitting out hours or a school day proves nothing
that the counters do not already say.

The window ends as soon as all three hold:

1. The cutover shipped in a production release, so the owner tables are filled
   and in use there. This is ordering, not waiting: the Contract migration may
   not drop columns the running image still needs.
2. Both compatibility hit counters read zero and the cutover's caller-inventory
   test is green. Together they answer who still uses the old path: the counters
   at runtime, the test in the code.
3. A restorable backup is verified, not merely present. After the Contract
   migration, rolling the image back is no longer enough; recovery means
   restoring that backup.

Then the Contract ticket goes into the next release. Recording an observation
day, a minimum duration or a separate evidence file is not required
(decided 2026-09-23, same reasoning as the deployment gates removed in #3453
and #3455).

## Product screenshots (marketing images)

Product screenshots are not PR evidence. They are the images for website, sales
material and social media: every **Shot** of `frontend/scripts/product-screenshots/shots.yaml`
photographed from the tenant, parents and Träger portal, plus **Geräte-Mockups** in the
official Apple bezels (see `CONTEXT.md`, "Produkt-Screenshots"; #3759).

```bash
scripts/product-screenshots.sh                  # all shots into tmp/product-screenshots/
scripts/product-screenshots.sh <shot-id> ...    # only these shots
scripts/product-screenshots.sh --out DIR --version NAME
cd frontend && pnpm run test:screenshots        # pipeline test against the running stack
```

- Needs the native stack (`scripts/dev-native.sh up`) seeded with the school
  profile `vollbetrieb`, the seeder's default and the demo's profile, followed by
  `scripts/dev-native.sh backend go run ./cmd/devtools simulate full-day`
  (without it only a few children are present); the tenant, parents and
  Träger logins come from
  `backend/.seed-state.json` (Träger: the profile's school admin). Seed and capture must happen on the same Berlin
  calendar day, or the command asks for a fresh seed instead of showing stale
  attendance. Only `*.localhost` hosts are photographed.
- Tenant screenshot and E2E logins complete the real MFA flow through local
  `MAILPIT_URL` before saving a browser session. The disappearance of the
  password form is not proof of a completed login. Sessions are reused within
  the browser worker; MFA policy and email-code limits remain unchanged.
- Output per shot: `roh-<device>.png`, `<device>.png` (mockup, transparent
  background) and both as WebP in 640/960/1280/1600/2400/3200 px, plus
  `manifest.json` (format marker, version, time, shots, files, sizes). The output
  directory is replaced only when its marker and files match a previous pipeline
  result; unrelated files and older outputs without the marker stay untouched.
  Remove or move an older output explicitly before rerunning. Nothing is uploaded.
- The browser clock stands at 13:30 Berlin of today's weekday
  (`reference-time.ts`). The server renders with its real clock, so the
  reference day must be the server's day; on a weekend a local run aborts on the
  resulting hydration error instead of printing an inconsistent image. The
  workflow sets the runner's clock to the last Tuesday, 07:20 Berlin, after its
  downloads and before migrating and seeding, and restores it before uploading.
  `simulate full-day --clock-command "$RUNNER_TEMP/set-clock"` then moves the
  clock forward wave by wave (`backend/internal/devtools/simulate/fullday_clock.go`):
  early care from 07:45, staff stamping in at their shift start (08:00, 11:30),
  sessions from 11:52, children class by class from 12:00, 12:15 and 12:35,
  pickups at 12:45. Every stamp lies before the browser's 13:30, and `set-clock`
  keeps the shift to the real time in `clock-shift` for the restore. Without
  `--clock-command` (a local run) all waves run at once and carry the real time.
  Text that depends on the time of day must therefore not be rendered on the
  server: the home greeting uses `useTimeBasedGreeting` (`src/lib/greeting.ts`).
- Adding a shot: the list follows the help flows and PostHog usage (#3764),
  with the reason in a comment above each shot. Ids follow the names in
  `frontend/public/help/screens/` where one exists for the same view, otherwise
  the page name; parents-portal shots start with `eltern-`. Never rename an
  existing id: it names the files in Drive. Detail pages get ids from the seed, so a shot starts on
  the list and a `klicken` step opens the detail; follow such a click with a
  `warten_auf` on an element of the target page (the dev server compiles the
  route on first use). The redirect check applies to `pfad`, the page the shot
  lands on before its steps.
- A broken shot (HTTP error, error page, silent redirect away from its `pfad`,
  login redirect, unexpected dialog, loading state after the timeout, console
  error) aborts the whole run before anything is written.
- `warten_auf` and the wait for loading indicators keep waiting while the page
  has requests to the stack open (`next dev` on CI needs 20-40 s for the
  parents child page). They give up after 30 s / 20 s without an open request,
  and after 2 minutes at the latest (`network-activity.ts`).
- Bezels live unchanged in `frontend/scripts/product-screenshots/bezels/`; the
  README there explains how to add one.

### Releases and the screenshot release asset

- Releases come from release-please (ADR 0042): every push to `main` keeps a
  release PR with the next version and `CHANGELOG.md`; merging it tags `vX.Y.Z`
  and publishes the GitHub release (`.github/workflows/release-please.yml`,
  `release-please-config.json`).
- `.github/workflows/product-screenshots.yml` runs after each release, on a
  manually published release, and by hand (`gh workflow run
  product-screenshots.yml --ref main [-f version=1.4.0]`; empty version = latest
  `vX.Y.Z` release). A manual run photographs the ref it starts from: `main`
  for the current state, `--ref v1.4.0` for exactly that release; runs from
  other branches attach nothing. It seeds a fresh stack, captures every shot,
  attaches the output as the artifact `product-screenshots-<version>` (7 days)
  and as `product-screenshots-<version>.zip` to the GitHub release `v<version>`.
  A rerun of the same version replaces the ZIP. Pull requests that touch the
  pipeline run the capture without the release asset.
- Only Berlin weekdays: on a weekend the run stops before seeding; start it by
  hand on the next weekday from the release tag.
- A broken shot fails the capture job, and the release-asset job never starts,
  so the release stays unchanged.
- Google Drive upload is unhooked until there is a shared Google account. The
  adapter (`publish.ts`, `publish-drive.ts`, `publish.pw.ts`) and
  `scripts/product-screenshots-drive-wizard.sh` stay in place; contract tests:
  `cd frontend && pnpm exec vitest run scripts/product-screenshots/publish.test.ts`.

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
