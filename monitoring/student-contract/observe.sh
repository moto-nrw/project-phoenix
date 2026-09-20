#!/usr/bin/env bash
# Read-only local Docker inspection. No API calls and no statistics resets.
set -euo pipefail
if [[ $# != 5 || ( "$1" != snapshot && "$1" != check ) ]]; then
  echo 'usage: observe.sh snapshot|check CONTAINER DATABASE BASELINE_JSON OUTPUT_PROM' >&2
  exit 2
fi
mode=$1
container=$2
database=$3
baseline=$4
output=$5
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
ok=0
observed=0
# Never forward SQL error text, which could include connection details.
if docker exec -i "$container" psql -X -qAt -U postgres --dbname="$database" \
    --set=ON_ERROR_STOP=1 < "$script_dir/observe.sql" > "$work/state" 2> "$work/error" &&
    jq -e 'type == "object" and (.database | type == "string") and
      (.system_identifier | type == "string") and (.database_oid | type == "number") and
      (.statistics_reset | type == "string") and (.statistics_dealloc | type == "number") and
      (.old_query_fingerprint | test("^[0-9a-f]{64}$")) and
      (.reads | type == "number") and (.writes | type == "number") and
      .tracking_all == true and
      (.compatibility_objects == 0 or .compatibility_objects == 2)' "$work/state" >/dev/null; then
  observed=1
  if [[ "$mode" == snapshot ]]; then
    # A baseline is explicit, exclusive, and only starts before Contract with
    # zero compatibility usage. It is never silently replaced by a timer.
    if jq -e '.reads == 0 and .writes == 0 and .compatibility_objects == 2' "$work/state" >/dev/null; then
      if (umask 077; set -o noclobber; cat "$work/state" > "$baseline") 2>/dev/null; then
        ok=1
      fi
    fi
  elif [[ -r "$baseline" ]] && jq -e -s 'length == 2 and
      .[0].compatibility_objects == 2 and .[0].reads == 0 and .[0].writes == 0 and
      (.[0] | del(.compatibility_objects)) == (.[1] | del(.compatibility_objects))' \
      "$baseline" "$work/state" >/dev/null; then
    ok=1
  fi
fi
# Atomic publication prevents partial scrapes. A failed read still publishes a
# fresh failure; a stopped collector is detected from its stale timestamp.
metric_tmp=$(mktemp "${output}.XXXXXX")
{
  echo "phoenix_student_contract_observation_ok $ok"
  echo "phoenix_student_contract_observation_read_ok $observed"
  echo "phoenix_student_contract_observation_timestamp_seconds $(date +%s)"
} > "$metric_tmp"
chmod 0644 "$metric_tmp"
mv -f -- "$metric_tmp" "$output"
if [[ "$ok" != 1 ]]; then
  echo 'student Contract observation failed; retain baseline and investigate before DDL' >&2
  exit 1
fi
