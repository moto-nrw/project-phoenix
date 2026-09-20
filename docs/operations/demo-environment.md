# Public demo environment

The public demo is a separate deployment on the existing host (ADR 0029).
Its Compose project is `demo`, its directory is `~/demo`, and its data is
synthetic. Never restore staging or production data into this environment.
The provisioning API, simulator and demo banner are separate work in #3456.

## Network and storage

| Service | Loopback listener | Public route |
| --- | --- | --- |
| Frontend | `127.0.0.1:3002` | `demo.moto-app.de`, `{slug}.demo.moto-app.de` |
| Parents portal | Same frontend | `eltern.demo.moto-app.de` |
| School portal | Same frontend | `schule.demo.moto-app.de` |
| Operator portal | Same frontend | `operator.demo.moto-app.de` |
| API | `127.0.0.1:8082` | `api.demo.moto-app.de` |
| PostgreSQL | `127.0.0.1:5434` | None; SSH tunnel only |

Compose owns the `demo` network and PostgreSQL volume within its own project.
The uploads volume is explicitly named `phoenix-demo-uploads`. Nothing mounts
the staging or production volumes. Containers reach the API as `server:8080`
and PostgreSQL as `postgres:5432`, never through a public host.

Point `demo.moto-app.de` and `*.demo.moto-app.de` at the host. Caddy must route
`api.demo.moto-app.de` to port 8082 before its wildcard frontend route to 3002.
Preserve Host and the forwarded protocol. The wildcard covers operator,
parents, school and tenant hosts; the bare `demo.moto-app.de` needs its own
certificate name. A certificate for `*.moto-app.de` does **not** cover these
second-level hosts.

Provision a certificate covering `demo.moto-app.de` and `*.demo.moto-app.de`
through ACME DNS-01. Use the host's DNS-provider module and secret store, or
mount a certificate managed by the existing DNS-01 tooling. With a managed
certificate, the Caddy routing shape is:

```caddyfile
demo.moto-app.de, *.demo.moto-app.de {
    tls /path/to/demo-fullchain.pem /path/to/demo-private-key.pem
    @api host api.demo.moto-app.de
    handle @api {
        reverse_proxy 127.0.0.1:8082
    }
    handle {
        reverse_proxy 127.0.0.1:3002
    }
}
```

Paths above are operator-supplied certificate paths, not repository files.
Keep renewal automated. Installing DNS, certificates and host routes is a
separate host operation; adding this configuration does not publish the demo.

## Configuration and images

`environments/demo.sops.env` has the same keys as staging and production.
The initial configuration reuses staging SMTP with the name `moto Demo`.
Database, application-role, JWT, NextAuth, metrics, admin, operator and device
secrets are independently generated. Analytics, Sentry and Web Push are
disabled in the initial demo configuration; configure their demo destinations
before enabling them. Frontend analytics also need the build secrets
`DEMO_NEXT_PUBLIC_POSTHOG_KEY` and `DEMO_NEXT_PUBLIC_SENTRY_DSN`; absent demo
keys disable the integrations without falling back to production keys.
No staging or production application secret is reused.
`scripts/create-demo-env.py` creates the initial SOPS file through SOPS only
and refuses to overwrite an existing file. Use `sops edit` for later changes.

The server receives `APP_ENV=demo`. The frontend receives the build-time
`NEXT_PUBLIC_APP_ENV=demo` identity for the future banner. Its hostnames and
identity are build arguments, not runtime switches. Other environments set
their own explicit identity. Never promote a frontend image between environments.

The backend uses the release SHA tag. The demo frontend uses `demo-<short SHA>`
and a convenience `demo` tag, without overwriting the production frontend's
SHA or `latest` tags. Deployment pins the SHA tags; rollback restores saved
image digests and configuration.

## Initial host setup

1. Create `~/demo` with owner-only permissions. Install the demo Compose and
   SOPS-decrypted env there as `docker-compose.yml` and `.env`. Do not print
   the decrypted file or render Compose values into logs.
2. Configure GitHub environment `demo` and repository secrets `DEMO_HOST`,
   `DEMO_SSH_KEY`, `DEMO_SSH_KNOWN_HOSTS`, plus the existing `SOPS_AGE_KEY`.
   The SOPS recipients match the other deployments. SSH host-key checking
   remains enabled. Missing demo secrets must not fall back to production.
3. Before the first workflow deployment, establish a healthy baseline using
   images built from `main`: pin backend `<short SHA>` and frontend
   `demo-<short SHA>`, pull, start PostgreSQL, run the `migrate` service, then
   start server and frontend with `--wait`. Record `CURRENT_SHA=<short SHA>`
   in `.deploy-state`. The release script deliberately requires a running
   baseline to capture a complete rollback snapshot; it is not a bootstrap tool.
4. Configure DNS and Caddy as above. Public access should wait for the demo
   provisioning and mail-delivery restrictions from #3456. This infrastructure
   change does not add those application restrictions.

## Deploy and rollback

Run **Build and Push Docker Images** with branch `main` and environment `demo`.
Only `workflow_dispatch` can deploy demo. Pushes to `main` still deploy only
production; pushes to `development` still deploy only staging. The demo job
uses GitHub environment `demo` and concurrency group `deploy-demo`.

For recovery, run **Manual Rollback**, also from `main`, and choose `demo`.
It shares the deploy concurrency group and server-side lock. Three complete
snapshot sets are retained in `~/backups/demo/`. Environment identity is
checked before stopping services or restoring anything. A staging or production
snapshot is rejected even when selected by absolute path. See
[complete release backup and rollback](release-backup-rollback.md) for recovery
and the data-loss boundary.

## Verification

`scripts/env-check.sh` checks key parity across all three encrypted files,
service allowlists and missing-value rejection in all three Compose files.
`node --test scripts/release-backup.test.mjs` checks demo image pinning,
deploy/rollback and rejection of foreign snapshots in every direction.
Frontend env tests require a known build-time identity, including `demo`.
These local checks do not prove live DNS, certificate renewal, SSH access or
delivery through the external SMTP service.
