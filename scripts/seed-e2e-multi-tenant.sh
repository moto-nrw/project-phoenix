#!/usr/bin/env bash
# ==============================================================================
# Project Phoenix — Add a Second Tenant for Tenant-Switch E2E Tests
#
# Run AFTER `scripts/seed-e2e.sh`. Creates a second school under the existing
# operator's organization and grants demo1@mail.de admin access to it. After
# this runs, demo1 has active mappings to both demo-school and second-school
# in auth.account_tenants — required for the TenantSwitcher dropdown to render
# and for /auth/switch-tenant to succeed.
#
# Idempotent: 409 conflicts on re-run are tolerated (we treat them as "already
# done"). Re-running after a fresh `seed-e2e.sh` is the supported workflow.
#
# Usage:  ./scripts/seed-e2e-multi-tenant.sh
# Prereqs:
#   - docker compose stack running with a seeded demo-school tenant
#   - jq + curl on PATH
# ==============================================================================

set -euo pipefail

# Defaults to the isolated E2E backend (server-e2e on :8081), set up by
# scripts/seed-e2e.sh. Override only with another local URL.
API_URL="${API_URL:-http://localhost:8081}"

# Guard against staging/production targets — every credential in this script
# is well-known and would compromise a real environment.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/assert-local-url.sh
source "$SCRIPT_DIR/lib/assert-local-url.sh"
assert_local_url "$API_URL"

# Operator account (matches scripts/seed-e2e.sh)
OPERATOR_EMAIL="operator@e2e.local"
OPERATOR_PASSWORD="E2EOp1234!"

# Second tenant config (must match frontend/e2e/helpers/seed-data.ts)
SECOND_SLUG="second-school"
SECOND_NAME="Demo School 2"
SECOND_ADMIN_EMAIL="admin-b@e2e.local"
SECOND_ADMIN_PASSWORD="E2EPass1234!"

# Account to link into the second tenant
LINK_EMAIL="demo1@mail.de"

for tool in curl jq; do
  command -v "$tool" >/dev/null || { echo "error: $tool is required" >&2; exit 1; }
done

# --- helpers ------------------------------------------------------------------

# api_post PATH BODY [EXTRA_HEADER...]
# Prints the HTTP body on stdout and returns 0 for 2xx/3xx, 1 otherwise.
# We bypass curl's --fail because we want to inspect 409s. Note: shell exit
# codes wrap mod 256, so returning the raw HTTP code (e.g. 256, 512) could
# silently mean "success". A boolean is enough for callers.
api_post() {
  local path=$1; local body=$2; shift 2
  local args=( -sS -X POST "$API_URL$path" -H "Content-Type: application/json" -d "$body" -w '\n%{http_code}' )
  while [[ $# -gt 0 ]]; do args+=( -H "$1" ); shift; done
  local out; out=$(curl "${args[@]}")
  local code=${out##*$'\n'}
  local payload=${out%$'\n'*}
  echo "$payload"
  return $(( code >= 400 ? 1 : 0 ))
}

api_get() {
  local path=$1; local bearer=$2
  curl -fsS "$API_URL$path" -H "Authorization: Bearer $bearer"
}

require_token() {
  local val=$1; local context=$2
  if [[ -z "$val" || "$val" == "null" ]]; then
    echo "error: failed to extract token in step: $context" >&2
    exit 1
  fi
}

# --- 1. operator login --------------------------------------------------------

echo "1/8  Operator login..."
OP_RESP=$(curl -fsS -X POST "$API_URL/operator/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$OPERATOR_EMAIL\",\"password\":\"$OPERATOR_PASSWORD\"}")
OP_TOKEN=$(echo "$OP_RESP" | jq -r '.data.access_token // .access_token // empty')
require_token "$OP_TOKEN" "operator login"

# --- 2. resolve organization id ----------------------------------------------

echo "2/8  Discover organization..."
ORG_LIST=$(api_get "/operator/organizations" "$OP_TOKEN")
ORG_ID=$(echo "$ORG_LIST" | jq -r '.data[0].id // empty')
if [[ -z "$ORG_ID" ]]; then
  echo "error: no organization found — run scripts/seed-e2e.sh first" >&2
  exit 1
fi
echo "      organization_id=$ORG_ID"

# --- 3. create School B (idempotent) -----------------------------------------

echo "3/8  Create school '$SECOND_SLUG'..."
SCHOOL_RESP=$(api_post "/operator/schools" \
  "{\"organization_id\":$ORG_ID,\"name\":\"$SECOND_NAME\",\"slug\":\"$SECOND_SLUG\",\"subdomain\":\"$SECOND_SLUG\"}" \
  "Authorization: Bearer $OP_TOKEN") && CREATE_OK=1 || CREATE_OK=0
SCHOOL_ID=$(echo "$SCHOOL_RESP" | jq -r '.data.id // empty')
if [[ "$CREATE_OK" -eq 0 || -z "$SCHOOL_ID" ]]; then
  # Fall back to listing schools to find an existing one with our slug.
  SCHOOL_LIST=$(api_get "/operator/schools?organization_id=$ORG_ID" "$OP_TOKEN" || true)
  SCHOOL_ID=$(echo "$SCHOOL_LIST" | jq -r ".data[]? | select(.slug == \"$SECOND_SLUG\") | .id" | head -n1)
  if [[ -z "$SCHOOL_ID" ]]; then
    echo "error: could not create or find school '$SECOND_SLUG'." >&2
    echo "       last response: $SCHOOL_RESP" >&2
    exit 1
  fi
  echo "      already exists, school_id=$SCHOOL_ID"
else
  echo "      created, school_id=$SCHOOL_ID"
fi

# --- 4. invite school B admin (returns token in body via seed header) --------

echo "4/8  Invite school admin..."
INVITE_RESP=$(api_post "/operator/schools/$SCHOOL_ID/invite-admin" \
  "{\"email\":\"$SECOND_ADMIN_EMAIL\",\"first_name\":\"School\",\"last_name\":\"BAdmin\",\"position\":\"OGS-Büro\"}" \
  "Authorization: Bearer $OP_TOKEN" \
  "X-Phoenix-Seed-Token: true") && INVITE_OK=1 || INVITE_OK=0
INVITE_TOKEN=$(echo "$INVITE_RESP" | jq -r '.data.token // empty')

# --- 5. accept invitation (skip if already accepted) -------------------------

if [[ "$INVITE_OK" -eq 1 && -n "$INVITE_TOKEN" ]]; then
  echo "5/8  Accept invitation..."
  api_post "/auth/invitations/$INVITE_TOKEN/accept" \
    "{\"password\":\"$SECOND_ADMIN_PASSWORD\",\"confirm_password\":\"$SECOND_ADMIN_PASSWORD\"}" \
    >/dev/null && echo "      accepted" || echo "      already accepted (continuing)"
else
  echo "5/8  Skip accept (admin likely already exists from earlier run)"
fi

# --- 6. login as school B admin ----------------------------------------------

echo "6/8  Login as school B admin..."
B_LOGIN=$(curl -fsS -X POST "$API_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$SECOND_ADMIN_EMAIL\",\"password\":\"$SECOND_ADMIN_PASSWORD\",\"tenant_slug\":\"$SECOND_SLUG\"}")
B_TOKEN=$(echo "$B_LOGIN" | jq -r '.data.access_token // .access_token // empty')
require_token "$B_TOKEN" "school B admin login"

# --- 7. resolve admin role id within school B --------------------------------

echo "7/8  Resolve admin role id..."
ROLES=$(api_get "/auth/roles?name=admin" "$B_TOKEN")
ADMIN_ROLE_ID=$(echo "$ROLES" | jq -r '.data[0].id // .[0].id // empty')
if [[ -z "$ADMIN_ROLE_ID" ]]; then
  echo "error: admin role not found in school B" >&2
  echo "       roles response: $ROLES" >&2
  exit 1
fi
echo "      admin role_id=$ADMIN_ROLE_ID"

# --- 8. link demo1 into school B ---------------------------------------------

echo "8/8  Link $LINK_EMAIL to '$SECOND_SLUG'..."
LINK_RESP=$(api_post "/auth/link-to-tenant" \
  "{\"email\":\"$LINK_EMAIL\",\"role_id\":$ADMIN_ROLE_ID}" \
  "Authorization: Bearer $B_TOKEN") && LINK_OK=1 || LINK_OK=0
if [[ "$LINK_OK" -eq 1 ]]; then
  echo "      linked"
else
  # 409 here means demo1 is already mapped — fine.
  echo "      already linked (server response: $LINK_RESP)"
fi

echo
echo "Multi-tenant setup complete."
echo "  - $LINK_EMAIL now has active mappings for: demo-school, $SECOND_SLUG"
echo "  - second tenant URL: http://${SECOND_SLUG}.localtest.me:3030 (only while Playwright is running)"
echo "  - tenant switcher should now appear in the header for $LINK_EMAIL"
