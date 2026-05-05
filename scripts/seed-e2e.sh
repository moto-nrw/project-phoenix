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
API_URL="${API_URL:-http://localhost:8081}"

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

# Bring up the e2e profile services. Idempotent — `up -d` is a no-op if
# they are already healthy. We only target the two services we need so we
# don't accidentally start the dev `frontend`/`server`/`postgres` stack.
echo "Bringing up isolated E2E backend (postgres-test + server-e2e)..."
docker compose up -d --wait postgres-test server-e2e

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
  --url http://localhost:8080

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
echo "  tenant URL:     http://${TENANT_SLUG}.localtest.me:3030  (only while Playwright is running)"
echo "  second tenant:  http://second-school.localtest.me:3030"
echo
echo "Run Playwright with:"
echo "  cd frontend && pnpm exec playwright test"
