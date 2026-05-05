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
# shellcheck source=assert-local-url.sh
source "$SCRIPT_DIR/assert-local-url.sh"
assert_local_url "$API_URL"

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
docker compose exec -T server-e2e go run main.go seed \
  --tenant-slug "$TENANT_SLUG" \
  --admin-email "$ADMIN_EMAIL" \
  --staff-password "$STAFF_PASSWORD" \
  --email "$OPERATOR_EMAIL" \
  --password "$OPERATOR_PASSWORD" \
  --pin "$OPERATOR_PIN" \
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
echo "  tenant URL:     http://${TENANT_SLUG}.localtest.me:3000"
echo "  second tenant:  http://second-school.localtest.me:3000"
echo
echo "Run Playwright with:"
echo "  cd frontend && pnpm exec playwright test"
