#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root/backend"

run_deadcode() {
  local mode=$1
  local output status

  set +e
  if [[ "$mode" == "tests" ]]; then
    output=$(go tool deadcode -tags=testhooks -test ./...)
  else
    output=$(go tool deadcode -tags= ./...)
  fi
  status=$?
  set -e

  if ((status != 0)); then
    printf '%s\n' "$output" >&2
    return "$status"
  fi

  printf '%s\n' "$output" | grep -v '^go: ' || true
}

test_output=$(run_deadcode tests)
if [[ -n "$test_output" ]]; then
  echo "Dead code detected with test entry points included:"
  printf '%s\n' "$test_output"
  exit 1
fi

# Production mode excludes, line for line:
# - test-support packages (test/, testutil/, testdb/, *test/): they serve tests only.
# - internal/architecture/ and internal/passthrough/: their production entry point is the architecture
#   command in scripts/backend-architecture, a module of its own that no
#   backend main reaches. The last run below checks the package from that
#   command, so it is not exempt, only measured there.
production_output=$(run_deadcode production)
if production_findings=$(printf '%s\n' "$production_output" | grep -Ev \
  '(^|/)(test|testutil|testdb|[[:alpha:]]+test)/|^internal/(architecture|passthrough)/'); then
  :
else
  production_filter_status=$?
  if ((production_filter_status != 1)); then
    exit "$production_filter_status"
  fi
  production_findings=
fi
if [[ -n "$production_findings" ]]; then
  echo "Production code reachable only from tests or from no entry point:"
  printf '%s\n' "$production_findings"
  exit 1
fi

# The architecture command has its own module. Build the backend-pinned tool
# to an owned path: `go tool -n` may name a temporary executable that is removed
# when the Go command exits, particularly with an external cache plugin.
tool_directory=$(mktemp -d)
trap 'rm -rf -- "$tool_directory"' EXIT
deadcode_binary="$tool_directory/deadcode"
go build -o "$deadcode_binary" golang.org/x/tools/cmd/deadcode
# The filter reports both tooling packages as well as the command itself.
tool_output=$(
  cd "$repo_root/scripts/backend-architecture"
  "$deadcode_binary" -filter '^github\.com/moto-nrw/project-phoenix/(internal/architecture|internal/passthrough|tools/architecturecmd)($|/)' ./...
)
tool_findings=$(printf '%s\n' "$tool_output" | grep -v '^go: ' || true)
if [[ -n "$tool_findings" ]]; then
  echo "Dead code detected in the architecture command:"
  printf '%s\n' "$tool_findings"
  exit 1
fi

echo "No unexpected dead code detected"
