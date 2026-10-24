# Complete release backup and rollback

Use a maintenance window for releases that change database contracts. This
release changes the email outbox: the old image cannot run against the new
schema. Restore the complete pre-deployment snapshot, not just the old image.
Rollback discards database and file changes since the snapshot. It cannot
recall sent emails, push messages, or other external effects.

## Deployment

Before pulling, the deployment tags every running image locally as
`<repository>:moto-release-<service>`. A rebuild of the same revision (every
demo dispatch rebuilds `demo-<sha>`) or an upstream `postgres` update moves the
tag the container was created from. On Docker's containerd image store the
running image would then lose its name and registry digest, and the snapshot
below would fail. The local tag keeps the digest resolvable.

The deployment pulls candidate images, stops the server and frontend (in the
demo stack also its `demo-runtime` sidecar, which writes to the database), then
captures the previous state under `~/backups/<environment>/release-<timestamp>-<sha>/`:

- `database.dump`: PostgreSQL schema, data, sequences and ACLs.
- `roles.sql`: cluster roles, attributes, password hashes and memberships.
- `uploads.tar.gz`: the actual server upload volume, including hidden files.
- `.env`, `compose.yml`, `images.tsv`, `deploy-state`: previous configuration,
  immutable image digests and deployment state.
- `environment`, `created-at`, `uploads-volume`, `checksums.sha256`, `complete`:
  snapshot identity and verification data.

The directory is owner-only (0700, files 0600). It is **not encrypted**: `.env`
contains plaintext operating secrets and `roles.sql` contains password hashes.
Database and uploads contain personal data. This is access control on the
source host, not protection against host/root compromise or site loss.
Keep these files on the server, never in Git or CI logs. Host certificates and the
Docker installation are unchanged infrastructure, not part of this
application rollback. This snapshot is not an off-host disaster-recovery copy.
Release snapshots are rollback points; the [encrypted daily backup](daily-backup.md) is the disaster-recovery backup for #3836.
The PostgreSQL container must be dedicated to the application: one `postgres`
database and default tablespaces. The scripts reject shared clusters. Restore
recreates non-system roles as well as the database, so newer role memberships
and privileges do not survive the rollback.

Missing containers, missing image digests, an unsuccessful application stop,
or an incomplete backup prevent migrations. Verification checks all required
files, their checksums, image availability, archive readability and Compose
configuration. Only verified snapshots receive a usable completion marker.
Retention removes complete sets, never individual components. Existing loose
backups are left untouched but cannot be selected by the new rollback workflow.

## Snapshot retention (Slice 3836-c)

**Implemented in code:** Deployments retain at most 3 staging, 7 production and
3 demo complete snapshots, additionally expiring snapshots after **30 days**.
`RELEASE_SNAPSHOT_MAX_AGE_DAYS` is explicit infrastructure configuration, an
integer from 1 to 90; missing or invalid values abort before a deployment changes
anything. The workflow sets 30 for all environments. Keep the independent
timer's value in sync when changing it. `BACKUP_RETENTION` remains the count
limit. Neither is an application/Compose variable or a tenant setting.

The age starts at the UTC creation timestamp in
`release-YYYYMMDDTHHMMSSZ-<sha>`, not the directory modification time. At the
configured age, cleanup deletes the whole directory, including interrupted
snapshots without `complete`. Recent interrupted snapshots do not count as
usable rollback points. Symlinks and names outside that format are not followed
or deleted. Operators must inventory and securely remove any older loose dumps
or nonstandard directories separately; they are not covered by this automation.
The count limit keeps the newest remaining complete snapshots; during a deploy,
the just-created rollback point always occupies one of those slots, even after
a backward clock adjustment. No snapshot is exempt from age expiry, even when
`.deploy-state` still names it. After a long period
without deployments, manual rollback can therefore be unavailable; use the
separate daily disaster-recovery procedure instead.

`scripts/release-backup-cleanup.sh` runs the same policy without Docker or backup
credentials, independent of deployments and daily-backup success. It acquires
the same `.release-operation.lock` as deploy, rollback and daily backup. A busy
lock or failed deletion returns nonzero, not success. The low-level
`release-backup.sh prune` command is only for callers already holding this lock.
An age limit is enforced at each cleanup run, not continuously. The daily timer
checks once per day and catches up after downtime (`Persistent=true`).

**Human installation and production evidence still required:**

1. Install the matching `release-backup.sh` and `release-backup-cleanup.sh` as
   root-owned, non-writable-by-others files under `/usr/local/lib/moto-release/`.
   This copy must be updated alongside future changes to release retention;
   deployment continues to use its environment-specific script copy.
2. Copy `scripts/systemd/moto-release-retention@.{service,timer}` to
   `/etc/systemd/system/`. For each environment present on this host, copy
   [release-retention.env.example](release-retention.env.example) to
   `/etc/moto-release-retention/<environment>.env` (root, 0600). Set the real
   absolute deployment directory, count (3/7/3) and age (30). This dedicated
   host configuration has no secrets; do not alter app env files or SOPS envs.
3. Run `systemd-analyze verify` on both units, then `systemctl daemon-reload`.
   Start `moto-release-retention@<environment>.service` once and inspect its
   exit status/journal and remaining snapshot ages without printing contents.
   Enable `moto-release-retention@<environment>.timer` with `systemctl enable
   --now`; confirm the next scheduled run via `systemctl list-timers`.
4. Record successful deletion and subsequent daily runs in the protected
   operating evidence. Investigate failed units, retry after a busy release
   lock clears, and configure the operator's existing failed-unit monitoring.
   Never remove a lock while another release/restore/backup owns it. Production
   installation, effective permissions, cleanup and monitoring have not been
   verified by this slice.

After migration and healthchecks, `.deploy-state` records `BACKUP_ID`. This is
the matching pre-deployment snapshot, not whichever dump happens to be newest.
Automatic rollback uses the same snapshot if migration or startup fails.

Only a healthy, recorded release then removes every image no container uses,
including the previous release's. Rollbacks do not depend on them: verification
pulls missing snapshot images by digest before anything is stopped. A failed
prune prints a warning and leaves the deployment's result unchanged.

## Manual rollback

1. Run **Manual Rollback** on the deployed branch. Production and demo require `main`.
2. Choose the environment. Leave `backup_id` empty to use its recorded
   pre-deployment snapshot, or supply a complete backup directory's basename.
3. The workflow validates the full snapshot and old images before any
   destructive action, stops the application, restores configuration, roles,
   database and uploads, then starts the old images and checks health.
4. Check login, attendance/check-in, a stored file and email delivery before
   reopening normal operation. A failed restore or healthcheck leaves the
   application stopped; inspect the failing component before retrying the same
   snapshot. Do not deploy a different image onto a partially restored state.

For a manual server-shell recovery, run the deployed `rollback-remote.sh` with
`DEPLOY_DIR` and optional `BACKUP_ID`. Both workflows share a CI concurrency
group and a per-environment server lock. An interrupted operation can leave
`.release-operation.lock`; remove it only after confirming no deployment or
restore process remains active. Never bypass it during a running operation.

`restore-db.sh <absolute-backup-directory>` restores the full snapshot but
does not start the application. Standalone `.dump` files and independently
chosen image tags are deliberately rejected. No down migration is needed.

## Verification

`node --test scripts/release-backup.test.mjs` exercises shell failure paths.
`node --test scripts/release-backup.integration.test.mjs` creates an isolated
PostgreSQL 17 Compose project, backs up and changes data, roles, outbox schema
and files, then restores through the manual rollback entrypoint. It verifies
the old schema, privileges, hidden files, deletion of post-backup files, saved
configuration and image digests. It does not exercise real production images,
production data volumes or external notification delivery.
