# Complete release backup and rollback

Use a maintenance window for releases that change database contracts. This
release changes the email outbox: the old image cannot run against the new
schema. Restore the complete pre-deployment snapshot, not just the old image.
Rollback discards database and file changes since the snapshot. It cannot
recall sent emails, push messages, or other external effects.

## Deployment

The deployment pulls candidate images, stops the server and frontend, then
captures the previous state under `~/backups/<environment>/release-<timestamp>-<sha>/`:

- `database.dump`: PostgreSQL schema, data, sequences and ACLs.
- `roles.sql`: cluster roles, attributes, password hashes and memberships.
- `uploads.tar.gz`: the actual server upload volume, including hidden files.
- `.env`, `compose.yml`, `images.tsv`, `deploy-state`: previous configuration,
  immutable image digests and deployment state.
- `environment`, `created-at`, `uploads-volume`, `checksums.sha256`, `complete`:
  snapshot identity and verification data.

The directory is owner-only. These files contain secrets and personal data;
keep them on the server, never in Git or CI logs. Host certificates and the
Docker installation are unchanged infrastructure, not part of this
application rollback. This snapshot is not an off-host disaster-recovery copy.
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

After migration and healthchecks, `.deploy-state` records `BACKUP_ID`. This is
the matching pre-deployment snapshot, not whichever dump happens to be newest.
Automatic rollback uses the same snapshot if migration or startup fails.

## Manual rollback

1. Run **Manual Rollback** on the deployed branch. Production requires `main`.
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
