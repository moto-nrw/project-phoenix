#!/usr/bin/env bash
# ==============================================================================
# Project Phoenix — Deterministic Seed for Playwright E2E Tests
#
# Brings up the isolated E2E backend stack (postgres-test + server-e2e),
# resets it, and re-seeds with fixed flags so every test run starts from a
# known state. The developer's dev `server` and `postgres` are NEVER touched.
#
# Constants here MUST stay in sync with `frontend/e2e/helpers/seed-data.ts`.
#
# Usage:  ./scripts/seed-e2e.sh
# ==============================================================================

set -euo pipefail

# The seeder targets server-e2e on host port 8081. The local-only guard
# refuses anything that isn't loopback/localhost/Docker-internal.
API_URL="${API_URL:-http://localhost:8081}"  # NOSONAR S5332 — loopback only, never a remote target

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/assert-local-url.sh
source "$SCRIPT_DIR/lib/assert-local-url.sh"
assert_local_url "$API_URL"

# Pre-flight: refuse to seed if :3030 is already taken. Playwright's
# webServer is configured with reuseExistingServer=false, so it will try
# to spawn its own dev server on :3030 and fail with EADDRINUSE later.
# Catch it now with a useful message instead of letting Playwright's
# error scroll past 200 lines of Next.js startup output.
if (echo > /dev/tcp/127.0.0.1/3030) >/dev/null 2>&1; then
  echo "error: port 3030 is already in use." >&2
  echo "       The Playwright suite spawns its own Next.js dev server on :3030 — it cannot proceed while" >&2
  echo "       another process is bound there." >&2
  echo
  echo "       Find the offender:" >&2
  echo "         lsof -nP -iTCP:3030 -sTCP:LISTEN" >&2
  echo
  echo "       Common causes: an aborted earlier Playwright run, or a stale 'pnpm run dev --port 3030'." >&2
  exit 1
fi

TENANT_SLUG="demo-school"
ADMIN_EMAIL="admin@e2e.local"
STAFF_PASSWORD="E2EPass1234!"
OPERATOR_EMAIL="operator@e2e.local"
OPERATOR_PASSWORD="E2EOp1234!"
OPERATOR_PIN="1234"

REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Both .env and docker-compose.yml are gitignored — devs manage their own
# customisations of the tracked .env.example / docker-compose.example.yml.
# On a fresh clone neither file exists, and the docker compose calls below
# would either fail with "no configuration file provided" or silently
# substitute empty strings for required env vars (PHOENIX_AUTH_PASSWORD,
# AUTH_JWT_SECRET, POSTGRES_PASSWORD) — which then makes server-e2e exit
# at startup before the healthcheck can even run, surfacing as a confusing
# unhealthy-container failure with no actionable error.
#
# Materialise from the examples when missing. NEVER overwrite an existing
# file so dev-side customisations are preserved; the stale-compose check
# below catches the one pre-existing case that's not safe to leave alone.

if [[ ! -f "$REPO_ROOT/.env" ]]; then
  echo ".env not found — creating from .env.example"
  echo "  NOTE: .env.example ships with placeholder secrets that are fine for"
  echo "  local dev / E2E but MUST be replaced before pushing to staging or"
  echo "  production (AUTH_JWT_SECRET / POSTGRES_PASSWORD / etc.)."
  cp "$REPO_ROOT/.env.example" "$REPO_ROOT/.env"
fi

if [[ ! -f "$REPO_ROOT/docker-compose.yml" ]]; then
  echo "docker-compose.yml not found — creating from docker-compose.example.yml"
  cp "$REPO_ROOT/docker-compose.example.yml" "$REPO_ROOT/docker-compose.yml"
fi

# Defence against stale gitignored compose files from BEFORE this branch:
# `server-e2e` and `postgres-test` were added recently, so a pre-existing
# local docker-compose.yml may still lack them. The next `docker compose
# up server-e2e` would otherwise fail with "no such service: server-e2e"
# instead of something actionable. Don't silently overwrite — surface
# the mismatch with a clear remediation hint.
if ! grep -qE '^\s+server-e2e:' "$REPO_ROOT/docker-compose.yml"; then
  echo "error: docker-compose.yml is missing the server-e2e service." >&2
  echo "" >&2
  echo "Your local docker-compose.yml predates the e2e profile services" >&2
  echo "added in this branch. Either:" >&2
  echo "  - rm docker-compose.yml && rerun this script" >&2
  echo "    (creates a fresh copy from docker-compose.example.yml)" >&2
  echo "  - or merge the postgres-test + server-e2e blocks from" >&2
  echo "    docker-compose.example.yml into your local file" >&2
  exit 1
fi

# Bring up the e2e profile services. We force-recreate server-e2e so any
# backend code change since the last run is picked up: server-e2e runs
# `go run . serve` (no hot reload — air would clash with the dev `server`
# service that shares the same ./backend:/app mount), so a healthy-but-
# stale container would otherwise keep serving the previous binary while
# the seeder writes fresh data, producing misleading test results.
# postgres-test does NOT need recreation — its data is wiped by
# `migrate reset` below, and recreating drops the cached page cache for
# no benefit.
echo "Bringing up isolated E2E backend (postgres-test + server-e2e)..."
docker compose up -d --wait postgres-test
docker compose up -d --wait --force-recreate --no-deps server-e2e

echo "Resetting test database (operator: $OPERATOR_EMAIL)..."
# Override OPERATOR_* env so the bootstrap migration creates our E2E
# operator rather than whatever the dev .env has set. Targets server-e2e,
# which is bound to postgres-test (APP_ENV=test, sslmode=disable).
docker compose exec -T \
  -e OPERATOR_EMAIL="$OPERATOR_EMAIL" \
  -e OPERATOR_PASSWORD="$OPERATOR_PASSWORD" \
  -e OPERATOR_DISPLAY_NAME="E2E Operator" \
  server-e2e go run main.go migrate reset

echo "Seeding with deterministic flags..."
# --state-path writes to .seed-state-e2e.json (instead of the default
# .seed-state.json) so a developer running `docker compose run server
# ./main seed` against the dev stack does not overwrite the E2E state
# file, and vice versa. The two stacks share the same mounted ./backend
# directory, so without this flag both seeds write to the same file.
# Keep in sync with frontend/e2e/helpers/seed-state.ts:SEED_STATE_PATH.
docker compose exec -T server-e2e go run main.go seed \
  --tenant-slug "$TENANT_SLUG" \
  --admin-email "$ADMIN_EMAIL" \
  --staff-password "$STAFF_PASSWORD" \
  --email "$OPERATOR_EMAIL" \
  --password "$OPERATOR_PASSWORD" \
  --pin "$OPERATOR_PIN" \
  --state-path .seed-state-e2e.json \
  --url http://localhost:8080  # NOSONAR S5332 — loopback only

# The seeder writes .seed-state-e2e.json from inside the container as root.
# On Linux (CI runners), the bind-mounted file ends up owned by root and the
# default umask leaves it unreadable to the host runner user — Playwright
# specs that read it via helpers/seed-state.ts then fail with EACCES. macOS
# Docker Desktop hides this by remapping UIDs, so it only bites in CI.
# chmod inside the container (where root can always set mode bits) makes
# the file world-readable so the host process can read it regardless of UID.
docker compose exec -T server-e2e chmod 644 .seed-state-e2e.json

echo
echo "Provisioning second tenant for tenant-switch coverage..."
# The tenant-switch spec asserts the TenantSwitcher dropdown — which only
# renders when the account has access to >=2 tenants. We always run the
# multi-tenant setup so the canonical seed leaves the suite in a state
# where every spec exercises real behaviour. The script is idempotent and
# tolerates 409s on re-run.
"$SCRIPT_DIR/seed-e2e-multi-tenant.sh"

echo
echo "Seed complete. Test stack:"
echo "  backend:        $API_URL  (server-e2e -> postgres-test)"
echo "  admin:          demo1@mail.de  / $STAFF_PASSWORD"
echo "  staff:          demo11@mail.de / $STAFF_PASSWORD"
echo "  tenant URL:     http://${TENANT_SLUG}.localtest.me:3030  (only while Playwright is running)"  # NOSONAR S5332 — localtest.me resolves to 127.0.0.1
echo "  second tenant:  http://second-school.localtest.me:3030"  # NOSONAR S5332 — localtest.me resolves to 127.0.0.1
echo
echo "Run Playwright with:"
echo "  cd frontend && pnpm exec playwright test"
