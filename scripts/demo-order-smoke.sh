#!/usr/bin/env bash
# Orders one demo school on the demo server after a deploy and waits until
# the demo runtime has seeded it, including the monthly invoices of its own
# contracts. Health checks stayed green on 2026-10-27 while every visitor's
# order failed; this proves the order path itself.
# The school is hidden again afterwards, so the runtime's expiry erases it.
# 0=ready, 1=failed or not ready in time. Runs on the demo host.
set -Eeuo pipefail
[[ "${DEMO_SMOKE_TIMEOUT_SECONDS:-}" =~ ^[1-9][0-9]*$ ]] || { echo 'Invalid DEMO_SMOKE_TIMEOUT_SECONDS' >&2; exit 1; }
cd "$HOME/demo"

sql() {
  docker compose exec -T postgres psql -U postgres -d postgres -v ON_ERROR_STOP=1 -At -c "$1"
}

# The last part of a demo slug scopes the school's accounts; six characters
# of the slug alphabet, like a visitor's order. No pipe from /dev/urandom:
# under pipefail its reader's SIGPIPE ends the script with 141.
alphabet=abcdefghijkmnpqrstuvwxyz23456789
suffix=''
for _ in 1 2 3 4 5 6; do suffix+=${alphabet:RANDOM%${#alphabet}:1}; done
slug="deploy-smoke-$suffix"
sql "INSERT INTO platform.demo_school_states (name, school_name, first_name, last_name)
     VALUES ('$slug', 'Deploy-Smoke', 'Deploy', 'Smoke')" > /dev/null
echo "demo order $slug queued"

deadline=$((SECONDS + DEMO_SMOKE_TIMEOUT_SECONDS))
status=preparing
while (( SECONDS < deadline )); do
  status=$(sql "SELECT status FROM platform.demo_school_states WHERE name = '$slug'")
  [[ "$status" = preparing ]] || break
  sleep 10
done
echo "demo order $slug: $status after ${SECONDS}s"

# The open school seeds its applications, contracts and monthly invoices
# afterwards (#6621). Wait for that seed; a failed one queues the school
# again, so it is preparing until the deadline.
seeded=f
while [[ "$status" = ready ]] && (( SECONDS < deadline )); do
  seeded=$(sql "SELECT COALESCE(seed_state->>'deferred_seed_pending', 'false') = 'false' FROM platform.demo_school_states WHERE name = '$slug'")
  [[ "$seeded" = t ]] && break
  sleep 10
  status=$(sql "SELECT status FROM platform.demo_school_states WHERE name = '$slug'")
done
invoices=0
if [[ "$seeded" = t ]]; then
  invoices=$(sql "SELECT count(*) FROM parent_billing.care_invoices
                  WHERE school_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = '$slug')")
fi
echo "demo order $slug: $status, deferred seed done: $seeded, $invoices monthly invoices of its own after ${SECONDS}s"

# A failed order is retired by the runtime already; hide a ready or stuck one.
sql "UPDATE platform.schools SET deleted_at = NOW()
     WHERE deleted_at IS NULL AND id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = '$slug')" > /dev/null
if [[ "$status" != ready || "$seeded" != t || "$invoices" -eq 0 ]]; then
  docker compose logs --no-log-prefix --since 30m demo-runtime 2>&1 | grep -F "$slug" | tail -n 5 >&2 || true
  exit 1
fi
