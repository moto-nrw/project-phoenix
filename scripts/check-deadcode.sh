#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root/backend"

run_deadcode() {
  local mode=$1
  local output status

  set +e
  if [[ "$mode" == "tests" ]]; then
    output=$(go tool deadcode -test ./... 2>&1)
  else
    output=$(go tool deadcode ./... 2>&1)
  fi
  status=$?
  set -e

  if ((status != 0)); then
    printf '%s\n' "$output" >&2
    return "$status"
  fi

  printf '%s\n' "$output" | grep -v '^go: ' || true
}

test_findings=$(run_deadcode tests)
if [[ -n "$test_findings" ]]; then
  echo "Dead code detected with test entry points included:"
  printf '%s\n' "$test_findings"
  exit 1
fi

production_findings=$(run_deadcode production | grep -Ev \
  '(^|/)(test|testutil|testdb|[[:alpha:]]+test)/|_test_helpers\.go|models/config/registry\.go:.*unreachable func: ResetRegistry$' || true)
if [[ -n "$production_findings" ]]; then
  echo "Production code reachable only from tests or from no entry point:"
  printf '%s\n' "$production_findings"
  exit 1
fi

echo "No unexpected dead code detected"
