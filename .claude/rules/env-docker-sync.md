---
paths:
  - "**/*.env*"
  - "**/Dockerfile*"
  - "**/*compose*.yml"
  - "environments/**"
  - "frontend/src/env.js"
  - "frontend/src/lib/env-validation.js"
  - ".github/workflows/build.yml"
---

# Environment & Docker File Sync

**RULE: When adding, removing, or renaming an environment variable, update ALL affected files.** A variable added in one place but missing from its counterparts causes silent misconfiguration or lefthook sync warnings.

## Required configuration and exceptions

Required infrastructure configuration must fail at startup/build with a clear
missing-key error. No implicit localhost URLs, credentials, or other runtime
fallbacks via `??`, `||`, Zod defaults, or Compose `${VAR:-default}`.

The explicit env-default exceptions are `NODE_ENV`, backend `LOG_LEVEL`, and
frontend `NEXT_PUBLIC_LOG_LEVEL`. The frontend's optional PostHog/Sentry
integration fields remain optional under `frontend/src/lib/env-validation.js`;
PostHog's host is required when its key is set. Optional does not mean a
fallback endpoint or credential may be invented.

Tenant settings resolve only from tenant overrides or registry defaults;
environment fallbacks are not allowed, including legacy compatibility chains.
Follow `settings-system.md`; env vars are for infrastructure, not school-admin
runtime settings.

## File Inventory

### Git-Tracked (templates/examples)

| File | Purpose |
|------|---------|
| `.env.example` | Template for root `.env` (docker-compose variable substitution) |
| `backend/dev.env.example` | Template for local backend dev (`go run main.go serve`) |
| `frontend/.env.example` | Template for frontend (`.env.local`) |
| `docker-compose.example.yml` | Template for local `docker-compose.yml` |
| `backend/Dockerfile` | Production backend image |
| `backend/Dockerfile.dev` | Dev backend image (air hot reload) |
| `frontend/Dockerfile` | Frontend dev image |
| `frontend/Dockerfile.prod` | Production frontend image |

### Git-Ignored (local secrets)

| File | Purpose |
|------|---------|
| `.env` | Root env, **auto-loaded by docker-compose** for `${VAR}` interpolation |
| `backend/dev.env` | Backend local dev env (loaded by viper) |
| `frontend/.env.local` | Frontend local env (loaded by Next.js) |
| `docker-compose.yml` | Local docker-compose config |

## How Docker Compose Uses Environment Variables

**Docker Compose auto-loads the root `.env` file** for `${VAR}` substitution in `docker-compose.yml`. This is how all services receive their configuration when running `docker compose up`. Use bare `${VAR}` for required values; exceptions are defined above.

The flow:
```
/.env  -->  docker-compose.yml (${VAR} substitution)  -->  container environment
```

- The `server` service `environment:` block maps root `.env` vars into the Go container
- The `frontend` service `environment:` block maps root `.env` vars into the Next.js container
- `backend/dev.env` is volume-mounted into the container (`./backend:/app`) but is secondary to the docker-compose environment block

**`backend/dev.env` is for local-only development** (`go run main.go serve` outside Docker).

## Critical: os.Getenv() vs viper.GetString()

The backend has TWO env var access patterns with different behavior:

| Access Pattern | Sees `docker-compose.yml` env block | Sees `backend/dev.env` |
|---------------|--------------------------------------|------------------------|
| `viper.GetString("key")` | Yes (via AutomaticEnv) | Yes (via config file) |
| `os.Getenv("KEY")` | Yes | **NO** (not an OS env var) |

**Code using `os.Getenv()` directly**: migrations (`OPERATOR_*`, `ADMIN_*`), scheduler (`CLEANUP_*`, `SESSION_*`), CORS, rate limiting, device auth.

**Consequence**: Any var consumed by `os.Getenv()` **MUST** be in the `docker-compose.yml` `environment:` block to work in Docker. Having it only in `backend/dev.env` is insufficient.

## Sync Pairs

When modifying any file, update its counterpart:

| Local File (git-ignored) | Template (git-tracked) |
|--------------------------|------------------------|
| `.env` | `.env.example` |
| `backend/dev.env` | `backend/dev.env.example` |
| `frontend/.env.local` | `frontend/.env.example` |
| `docker-compose.yml` | `docker-compose.example.yml` |

## Adding a New Backend Env Var Checklist

- [ ] Add to `backend/dev.env.example` (with safe default or placeholder)
- [ ] Add to `docker-compose.example.yml` server `environment:` block (bare `${VAR}` — no `:-` default)
- [ ] Add to `.env.example` (with placeholder value)
- [ ] Add to both `environments/*.sops.env` files via `sops` CLI
- [ ] If used by `os.Getenv()`: confirm it is in docker-compose `environment:` block (not just dev.env)

## Adding a New Frontend Env Var Checklist

- [ ] Add to `frontend/.env.example`
- [ ] If needed in Docker: add to `docker-compose.example.yml` frontend `environment:` block
- [ ] If needed in Docker: add to `.env.example`
- [ ] Add to both `environments/*.sops.env` files via `sops` CLI
- [ ] If needs per-service override: add to `environment:` block in `environments/*.compose.yml`
- [ ] If `NEXT_PUBLIC_*`: client-accessible, no server import restrictions
- [ ] If server-only: use `getServerApiUrl()` pattern, don't import in mixed client/server files
- [ ] If required in `env.js` (no `.optional()` / `.default()`): add as `ARG` + `ENV` in `frontend/Dockerfile.prod`
- [ ] If required in `env.js` (no `.optional()` / `.default()`): add as `build-args` in `.github/workflows/build.yml`

## Deployed Environments (SOPS)

Staging and production use SOPS-encrypted env files in `environments/`. CI decrypts one `.env` for Compose interpolation. Explicit service allowlists control what enters each container; see `docs/runtime-environment-boundaries.md` and `environments/runtime-env-allowlist.json`. Read `docs/agents/operations.md` Environment Management (SOPS) for deployment and rollback details.

Keep the frontend's explicit `PORT: 3000` and the serving backend's credential-free application DSN. Privileged `DB_DSN` interpolation belongs only to the explicit `migrate` job. Update the allowlist and its matrix when changing service variables; `scripts/env-check.sh` verifies the boundary.

## Automated Sync Checks

**Local dev** — lefthook `post-merge` hook runs `dotenv-linter diff` on three pairs and `dyff between` on docker-compose:

```bash
# Manual verification
dotenv-linter diff .env .env.example
dotenv-linter diff backend/dev.env backend/dev.env.example
dotenv-linter diff frontend/.env.local frontend/.env.example
dyff between --omit-header docker-compose.example.yml docker-compose.yml
```

**Deployed environments** — CI `env-sync-check` job runs `scripts/env-check.sh` on every PR:

```bash
# Manual verification
./scripts/env-check.sh  # Validates key sync across all .sops.env files + .env.example
```
