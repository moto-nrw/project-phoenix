# Runtime environment boundaries

Staging and production use the same service allowlists. SOPS remains the source
of secrets. CI delivers the decrypted file for Compose interpolation; containers
receive only their explicit `environment` entries.

## Reviewed service-to-variable matrix

The exact, machine-checked key sets live in
[`environments/runtime-env-allowlist.json`](../environments/runtime-env-allowlist.json).
The groups below explain why each set exists.

| Consumer | Configuration and authority | Source |
| --- | --- | --- |
| Frontend runtime | `NEXTAUTH_SECRET` signs portal sessions; `METRICS_BEARER_TOKEN` protects metrics. JWT expiry/refresh timing remains required, without the backend JWT signing key. API URL, portal/domain URLs, log level, public analytics/Sentry settings, proxy trust, timezone, port and hostname complete its allowlist. | `frontend/src/env.js`, `lib/env-validation.js`, `lib/server-runtime-env.ts`, `proxy.ts`, NextAuth host-trust configuration |
| Frontend build | Public `NEXT_PUBLIC_*` values plus internal `API_URL` and `TENANT_DOMAIN`. CI build arguments supply browser-inlined values. Runtime values cannot replace an already built browser bundle. No authentication, database, SMTP or push private key is a build argument. | `frontend/Dockerfile.prod`, `.github/workflows/build.yml` |
| Serving backend | Backend JWT key, application-role password, SMTP credentials, device PIN, VAPID private key, metrics token, analytics settings; pool sizing, public API and portal URLs, auth timing, rate limits, logging, scheduler and checkout configuration. | `backend/cmd/serve.go`, `database/database_config.go`, `services/factory.go`, `api/base.go`, `services/scheduler/` |
| PostgreSQL | `POSTGRES_PASSWORD` and `TZ` only. Database administration stays inside this boundary. | PostgreSQL image startup; existing backup/restore commands |
| Migration job | Privileged `DB_DSN`, pool sizing, `APP_ENV`, `TZ`, `PHOENIX_AUTH_PASSWORD`, and admin/operator bootstrap credentials. No HTTP signing, SMTP, device or push secrets. | `backend/cmd/migrate.go`; migrations `001006002`, `001011003`, `001014001` |
| Backup and restore | `pg_dump`, `pg_dumpall`, `psql`, and `pg_restore` execute inside PostgreSQL over its Unix socket. They need no backend or frontend environment. Globals dumps contain role credentials and remain protected backup artifacts. | `scripts/deploy-remote.sh`, `scripts/restore-db.sh` |

The serving backend receives a credential-free endpoint:
`postgres://phoenix_auth@postgres:5432/postgres`, with `sslmode=require` in
production and `sslmode=disable` in staging. These match the tracked Compose
topology and encrypted maintenance DSN endpoints. `GetServeDSN` adds only
`PHOENIX_AUTH_PASSWORD`, using URL encoding. The privileged SOPS `DB_DSN`
is passed only to the one-shot `migrate` service. No SOPS key or credential
changes are needed.

Local development still combines migration and hot reload in its existing
source-mounted service. This deployment boundary does not claim to isolate
the developer's filesystem or change that workflow.

### Empty and unsupported settings

`${KEY:?KEY is required}` rejects unset and empty required configuration.
Optional SMTP authentication, analytics, Sentry and VAPID values use
`${KEY?KEY is required}`: a deliberate empty value remains valid, but an
absent key is an error. Existing application validation checks dependent values.
There are no invented configuration defaults.

`DB_DEBUG`, `MAILPIT_URL`, `SKIP_ENV_VALIDATION`, and bootstrap account
credentials do not belong in deployed serving processes. Node production mode
and telemetry settings already come from the final image.

Legacy `GUARDIAN_INVITATION_TOKEN_EXPIRY_HOURS` and enrollment CAPTCHA
environment settings are absent from both encrypted deployment configurations.
The allowlist does not introduce these optional features. Adding deployment
support requires the usual settings/environment review and key synchronization.
`PHOENIX_ALLOW_GUARDIAN_RESET` remains an explicit maintenance opt-in, never
enabled by the deployment service.

## Maintenance and validation

1. Deploy migrations with `docker compose run --rm migrate`. The
   `maintenance` profile excludes the job from normal `up`; explicitly running
   the service enables it. CI pins both backend service images to the same SHA.
2. Run `scripts/env-check.sh`. It checks SOPS key parity, example keys, parsed
   Compose allowlists, credential-free serve DSNs, and fail-fast bindings.
   `python3 scripts/check-runtime-env.py --revision HEAD` can test a historical
   configuration against the current policy.
3. Run `python3 scripts/rehearse-runtime-env.py` for an isolated Compose
   rehearsal with fixture credentials and disposable storage. It builds production
   images, migrates, starts both apps, checks application/tenant/admin roles,
   backs up and restores through the existing restore script, and checks health
   again. It never loads SOPS or the developer's dotenv files.

### Separately authorized post-deploy check

After deployment is authorized separately, run this in the deployed Compose
directory for each service (`frontend`, `server`, `postgres`):

```bash
docker compose exec -T frontend sh -c 'env | cut -d= -f1 | sort'
```

Compare the names with the allowlist, allowing image-owned runtime names such
as `PATH`, `HOME`, `NODE_ENV` and `NEXT_TELEMETRY_DISABLED`. To check the
backend DSN without disclosing it, pipe its environment directly into
`scripts/inspect-env-boundary.py` on a trusted machine. That script reports
only names and booleans. Do not print raw `docker inspect` environment arrays
or rendered Compose values.

## Local verification, 2026-09-05

| Gate | Command or observation | Result |
| --- | --- | --- |
| Original configuration | `python3 scripts/check-runtime-env.py --revision 13b0e69080d1852a466b38cb3fd2d72cf7a6c018` | Rejected: frontend whole-file injection. |
| Patched configuration | `scripts/env-check.sh` | Passed SOPS parity, both Compose files, exact key sets, endpoint constraints and missing-key rejection. |
| Focused DSN checks | `scripts/run-go-toolchain.sh go -C backend test ./database -run 'TestServeDSN\|TestGetDatabaseDSN'` | Passed credential-free endpoints, escaped passwords, missing configuration, test-DB isolation and redacted parser errors. |
| Runtime and maintenance | `python3 scripts/rehearse-runtime-env.py` | Built production images; observed frontend key set without backend/database secrets; application role was neither superuser nor BYPASSRLS; tenant/admin role transactions passed. Migration, database/global backup, existing restore script, fixture preservation and post-restore health passed. Disposable containers were removed. |
| Changed-package checks | `scripts/test-changed.sh` | 143 backend packages and 18 frontend files / 309 tests passed. |
| Full backend suite | `scripts/run-go-toolchain.sh scripts/test-backend.sh` | 24,645 tests, 8 conditional skips, no failures. Skips covered unconfigured push, seed coverage and near-midnight reminder cases. |
| Backend lint | Pinned `golangci-lint run --timeout 10m` from `backend/` | Zero issues. |
| Frontend quality | `pnpm --dir frontend run check` | Locale checks, lint and typechecking passed. |
| Full frontend suite | `pnpm --dir frontend run test -- --run` | 1,092 files and 14,807 tests passed. |
| Independent review | Standards and spec/security reviewers | No actionable standards violations, surviving bypasses, regressions or omitted implementation requirements. |

No deployment, live exploitation, credential rotation or post-deploy inspection
was performed. The observed runtime result is from the disposable local project,
not a claim about currently deployed containers.
