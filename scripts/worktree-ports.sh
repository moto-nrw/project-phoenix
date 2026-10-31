#!/usr/bin/env bash
# Gives every linked worktree its own host ports, as `wt add` does for managed
# ones. Worktrees created by other tools (T3, agents, `git worktree add`) copy
# the main checkout's .env and all listen on 3000/8080/5432/8025/1025, so their
# dev servers, Compose services and E2E runs collide.
#
# Idempotent; run by scripts/dev-native.sh and scripts/bootstrap-dev.sh.
#   1. Linked worktree without .wt.env and still on the default ports: pick a
#      port slot from a hash of the worktree path, shifting to the next slot
#      while any of its ports is in use, and write the ports into .env. The
#      slot then stays fixed for this worktree.
#   2. Any linked worktree: rewrite localhost URLs in .env, backend/dev.env and
#      frontend/.env.local that still name a default port to this worktree's
#      port (wt leaves CATERER_URL and TRAEGER_URL on 3000).
# The main checkout and the shared test database (TEST_DB_PORT) stay unchanged.
# Prints key names and port numbers only, never other values.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"
[ "$(git rev-parse --path-format=absolute --git-dir)" != "$(git rev-parse --path-format=absolute --git-common-dir)" ] || exit 0
[ -f .env ] || exit 0

keys=(FRONTEND_HOST_PORT SERVER_HOST_PORT POSTGRES_HOST_PORT MAILPIT_HOST_PORT MAILPIT_SMTP_HOST_PORT)
defaults=(3000 8080 5432 8025 1025)
# wt's lattice: slot k uses 3000+10k, 8080+10k, 5432+20k, 8025+10k, 1025+10k.
# The ports of one slot differ from every other slot's ports, also across
# keys. Derived slots start at 50, above the low slots wt leases.
steps=(10 10 20 10 10)
first_slot=50
slot_count=200

value_of() { # key file
  sed -n "s/^$1=//p" "$2" | tail -n 1
}
port_free() {
  ! lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

current=()
for i in "${!keys[@]}"; do
  value=
  [ -f .wt.env ] && value=$(value_of "${keys[$i]}" .wt.env)
  [ -n "$value" ] || value=$(value_of "${keys[$i]}" .env)
  current+=("$value")
done

if [ ! -f .wt.env ] && [ "${current[*]}" = "${defaults[*]}" ]; then
  hash=$(printf '%s' "$(cd "$root" && pwd -P)" | cksum | cut -d' ' -f1)
  for ((try = 0; try < slot_count; try++)); do
    slot=$((first_slot + (hash + try) % slot_count))
    candidate=()
    for i in "${!keys[@]}"; do candidate+=($((defaults[i] + steps[i] * slot))); done
    busy=false
    for port in "${candidate[@]}"; do port_free "$port" || busy=true; done
    [ "$busy" = false ] && break
  done
  [ "$busy" = false ] || { echo "worktree-ports: no free port slot" >&2; exit 1; }
  # mktemp files are private (0600), which also suits the env files.
  tmp=$(mktemp .env.ports.XXXXXX)
  cat .env >"$tmp"
  for i in "${!keys[@]}"; do
    if grep -q "^${keys[$i]}=" "$tmp"; then
      sed "s/^${keys[$i]}=.*/${keys[$i]}=${candidate[$i]}/" "$tmp" >"$tmp.next" && mv "$tmp.next" "$tmp"
    else
      printf '%s=%s\n' "${keys[$i]}" "${candidate[$i]}" >>"$tmp"
    fi
  done
  mv "$tmp" .env
  current=("${candidate[@]}")
  echo "worktree-ports: own ports for this worktree (slot $slot): ${keys[*]} = ${candidate[*]}"
fi

# Rewrite localhost:<default> (also *.localhost) when this worktree uses
# another port; the digit guard keeps localhost:30001 and similar intact.
script=
stale=()
for i in 0 1 2 3; do # SMTP is not addressed by URL.
  [ "${current[$i]}" = "${defaults[$i]}" ] && continue
  script+="s/localhost:${defaults[$i]}([^0-9]|\$)/localhost:${current[$i]}\\1/g;"
  stale+=("${defaults[$i]}")
done
[ -n "$script" ] || exit 0
for file in .env backend/dev.env frontend/.env.local; do
  [ -f "$file" ] || continue
  changed=$(grep -cE "localhost:($(IFS='|'; echo "${stale[*]}"))([^0-9]|$)" "$file" || true)
  [ "$changed" -gt 0 ] || continue
  tmp=$(mktemp "$file.ports.XXXXXX")
  sed -E "$script" "$file" >"$tmp" && mv "$tmp" "$file"
  echo "worktree-ports: $file: $changed line(s) now use this worktree's ports"
done
