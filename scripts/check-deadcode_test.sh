#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT

mkdir -p "$fixture/bin"
cat >"$fixture/bin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == 'tool -n deadcode' ]]; then
  printf '%s/deadcode\n' "$(dirname "$0")"
  exit 0
fi
if [[ "$*" == 'tool deadcode -test ./...' ]]; then
  mode=tests
else
  mode=production
fi
exec "$(dirname "$0")/deadcode" "$mode"
SH
cat >"$fixture/bin/deadcode" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
mode=${1:-}
[[ "$mode" != './...' ]] || mode=architecture
printf 'blacksmith-gocacheprog pid=%s\n' "$$" >&2
if [[ "$mode" == "$FAIL_MODE" ]]; then
  echo 'analyzer failed' >&2
  exit 7
fi
if [[ "$mode" == "$FINDING_MODE" ]]; then
  echo 'services/example.go:12:1: unreachable func: Example'
fi
SH
chmod +x "$fixture/bin/go" "$fixture/bin/deadcode"

run_case() {
  local name=$1 finding_mode=$2 fail_mode=$3 expected_status=$4
  local status=0
  PATH="$fixture/bin:$PATH" FINDING_MODE="$finding_mode" FAIL_MODE="$fail_mode" \
    bash "$repo_root/scripts/check-deadcode.sh" >"$fixture/stdout" 2>"$fixture/stderr" || status=$?
  if [[ "$status" != "$expected_status" ]]; then
    echo "$name: expected status $expected_status, got $status" >&2
    cat "$fixture/stdout" "$fixture/stderr" >&2
    exit 1
  fi
  if [[ -n "$finding_mode" ]]; then
    grep -q 'unreachable func: Example' "$fixture/stdout"
  elif [[ -n "$fail_mode" ]]; then
    grep -q 'analyzer failed' "$fixture/stderr"
  else
    grep -q 'No unexpected dead code detected' "$fixture/stdout"
    grep -q 'blacksmith-gocacheprog' "$fixture/stderr"
    if grep -q 'blacksmith-gocacheprog' "$fixture/stdout"; then
      echo 'Cache diagnostics leaked into findings' >&2
      exit 1
    fi
  fi
  echo "PASS: $name"
}

run_case 'successful analyzer with stderr diagnostics' '' '' 0
for mode in tests production architecture; do
  run_case "$mode findings still fail" "$mode" '' 1
  run_case "$mode analyzer errors still fail" '' "$mode" 7
done
