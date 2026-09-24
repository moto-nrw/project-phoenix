# Public demo environment

The public demo runs on its own VM, apart from the staging and production host
(ADR 0029, host decision in #3460). Its Compose project is `demo`, its
directory is `~/demo`, and its data is
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
and PostgreSQL as `postgres:5432`, never through a public host. For read-only
database access, open a tunnel: `ssh -L 5434:127.0.0.1:5434 root@<DEMO_HOST>`.

## Host

| Setting | Value |
| --- | --- |
| Provider | Hetzner Cloud, project `moto-demo`, server `moto-demo` |
| Location | `nbg1`, same as production |
| Type | CPX22 (2 vCPU, 4 GB RAM, 80 GB). Scale up to 4 vCPU / 8 GB when load or active demo schools grow; rescaling reboots and keeps the IP, and a disk upgrade cannot be undone. |
| OS | Ubuntu 26.04 LTS |
| Firewall | Hetzner firewall `moto-demo-fw`: inbound TCP 22, 80, 443 only |
| SSH | `root`, with a personal key and the CI deploy key behind `DEMO_SSH_KEY` |
| Backups | None at Hetzner (synthetic data); take a snapshot before trade fairs |

Ubuntu 26.04 ships Rust coreutils (uutils) and `sudo-rs`. The release scripts
use only basic flags. If a GNU-specific behavior breaks a host command,
`apt install coreutils-from-gnu` switches back without rebuilding the VM.

Docker Engine and the Compose plugin come from Docker's apt repository.
journald stores logs persistently; all four services log to it with the tags
`demo-postgres`, `demo-server`, `demo-frontend` and `demo-runtime`.

## DNS, certificate and Caddy

Cloudflare points `demo.moto-app.de` and `*.demo.moto-app.de` (A and AAAA) at
the VM, DNS only (grey cloud): Caddy terminates TLS itself. The wildcard covers
operator, parents, school and tenant hosts; the bare `demo.moto-app.de` needs
its own certificate name. A certificate for `*.moto-app.de` does **not** cover
these second-level hosts.

Caddy obtains and renews one certificate for both names through ACME DNS-01,
the same wiring as production:

- Caddy comes from its apt repository with the `caddy-dns/cloudflare` module
  added by `caddy add-package`. The package is on `apt-mark hold`, since an apt
  upgrade would replace the custom binary; update it with `caddy upgrade`,
  which keeps added modules.
- The Cloudflare token (`Zone:DNS:Edit` on `moto-app.de`, restricted to the
  VM's IPs) lives in `/etc/caddy/cloudflare.env` (mode 600, owner `caddy`) and
  reaches Caddy through the systemd drop-in
  `/etc/systemd/system/caddy.service.d/cloudflare.conf`.

Caddy must route `api.demo.moto-app.de` to port 8082 before its wildcard
frontend route to 3002. `reverse_proxy` preserves Host and the forwarded
protocol. The operator surface stays internal: the demo operator signs in
without a second factor (#3460), so Caddy answers 404 for the operator host
and every operator path on all demo hosts. The demo process reaches the API
as `server:8080` and is not affected. `/etc/caddy/Caddyfile`:

```caddyfile
demo.moto-app.de, *.demo.moto-app.de {
	tls {
		dns cloudflare {env.CLOUDFLARE_API_TOKEN}
	}
	@operator host operator.demo.moto-app.de
	handle @operator {
		respond 404
	}
	@operatorPaths path /operator /operator/* /api/operator /api/operator/*
	handle @operatorPaths {
		respond 404
	}
	@api host api.demo.moto-app.de
	handle @api {
		reverse_proxy 127.0.0.1:8082
	}
	handle {
		reverse_proxy 127.0.0.1:3002
	}
}
```

`caddy validate` needs the token in its environment; run it through
`systemd-run --pipe -p EnvironmentFile=/etc/caddy/cloudflare.env`.

## Configuration and images

`environments/demo.sops.env` has the same keys as staging and production.
The initial configuration reuses staging SMTP with the name `moto Demo`.
Database, application-role, JWT, NextAuth, metrics, admin, operator and device
secrets are independently generated. Frontend analytics use the shared PostHog
project (free plan, one project); demo events carry `deployment=demo`, so they
filter apart from schools. They need the build secret
`DEMO_NEXT_PUBLIC_POSTHOG_KEY` besides the SOPS value. Backend analytics
(`POSTHOG_API_KEY`) use the same project token; the backend stamps
`deployment=demo` on every event, so they stay apart from schools too. Sentry and Web Push are disabled;
configure their demo destinations before enabling them. Frontend Sentry also
needs the build secret `DEMO_NEXT_PUBLIC_SENTRY_DSN`; absent demo keys disable
the integrations without falling back to production keys.
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

1. Create `~/demo` with owner-only permissions, plus `~/scripts/demo` and
   `~/backups/demo`. Install the demo Compose and SOPS-decrypted env in
   `~/demo` as `docker-compose.yml` and `.env` (mode 600). Do not print
   the decrypted file or render Compose values into logs.
2. Configure GitHub environment `demo` and repository secrets `DEMO_HOST`,
   `DEMO_SSH_KEY`, `DEMO_SSH_KNOWN_HOSTS`, plus the existing `SOPS_AGE_KEY`.
   The SOPS recipients match the other deployments. SSH host-key checking
   remains enabled. Missing demo secrets must not fall back to production.
3. Before the first workflow deployment, establish a healthy baseline using
   images built from `main`: pin backend `<short SHA>` and frontend
   `demo-<short SHA>`, pull, start PostgreSQL, run the `migrate` service,
   start server and frontend with `--wait`, then start `demo-runtime` with
   `docker compose up -d --no-deps demo-runtime`. Record
   `CURRENT_SHA=<short SHA>` in `.deploy-state`. The release script
   deliberately requires a running baseline to capture a complete rollback
   snapshot; it is not a bootstrap tool.
4. Configure DNS and Caddy as above.

## Deploy and rollback

Run **Build and Push Docker Images** with branch `main` and environment `demo`.
Only `workflow_dispatch` can deploy demo. Pushes to `main` still deploy only
production; pushes to `development` still deploy only staging. The demo job
uses GitHub environment `demo` and concurrency group `deploy-demo`. Each deploy
also starts the standing demo school (`demo-runtime`) on the released backend
image; see [standing demo school](standing-demo.md#deployed-sidecar).

Release scripts are copied to `~/scripts/demo/`, the same per-environment
layout as staging and production, which share their host and rely on separate
script directories.

For recovery, run **Manual Rollback**, also from `main`, and choose `demo`.
It shares the deploy concurrency group and server-side lock. Three complete
snapshot sets are retained in `~/backups/demo/`. Environment identity is
checked before stopping services or restoring anything. A staging or production
snapshot is rejected even when selected by absolute path. See
[complete release backup and rollback](release-backup-rollback.md) for recovery
and the data-loss boundary.

## Availability alarm

A failed deploy notifies by e-mail and Slack. A crashed container, a failed
certificate renewal or an unreachable machine does not, and the demo answers
visitors without anyone watching. The **Demo uptime** workflow probes
`https://demo.moto-app.de/api/health` and `https://api.demo.moto-app.de/health`
from outside, three attempts each, and reads the certificate's remaining days.
On failure it posts to the deploy Slack webhook and the run turns red.

It runs only when the repository variable `DEMO_MONITOR_ENABLED` is `true`, so
it stays quiet until the demo answers; `workflow_dispatch` ignores the variable
and always probes. Set the variable right after the first successful deploy.
Scheduled runs can be delayed by several minutes under GitHub load, so this is
a safety net rather than monitoring with a guaranteed interval. For event days,
either tighten the cron schedule or add an external monitor that pages faster.

## Verification

`scripts/env-check.sh` checks key parity across all three encrypted files,
service allowlists and missing-value rejection in all three Compose files.
`node --test scripts/release-backup.test.mjs` checks demo image pinning,
deploy/rollback and rejection of foreign snapshots in every direction.
Frontend env tests require a known build-time identity, including `demo`.
These local checks do not prove live DNS, certificate renewal, SSH access or
delivery through the external SMTP service.
