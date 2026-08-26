#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, deren Produktions- oder
#   Test-Abhängigkeiten sie transitiv importieren.
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [base-ref]   (Default: origin/development)
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

BASE=${1:-origin/development}
# Merge-Base statt Drei-Punkt-Diff, damit auch uncommittete Änderungen zählen.
MB=$(git merge-base HEAD "$BASE")

affected=()
affected_output=$(scripts/backend-affected-packages.sh "$BASE")
while IFS= read -r package; do
  [ -n "$package" ] && affected+=("$package")
done <<< "$affected_output"

if [ "${#affected[@]}" -gt 0 ]; then
  cpu_count=$(getconf _NPROCESSORS_ONLN)
  if ! [[ "$cpu_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "getconf returned an invalid CPU count: $cpu_count" >&2
    exit 1
  fi
  package_workers=$((cpu_count / 2))
  if [ "$package_workers" -lt 1 ]; then
    package_workers=1
  elif [ "$package_workers" -gt 4 ]; then
    package_workers=4
  fi
  binary_cpus=$((cpu_count / (2 * package_workers)))
  if [ "$binary_cpus" -lt 1 ]; then
    binary_cpus=1
  fi
  test_workers=$cpu_count
  if [ "$test_workers" -gt 8 ]; then
    test_workers=8
  fi

  # Half the machine (at most four package binaries) keeps the changed-test
  # loop usable on weak hardware. Each DB-backed binary sizes its own pool from
  # -parallel, so this also bounds local PostgreSQL connection pressure.
  echo "==> go test (${#affected[@]} affected packages; -p $package_workers, GOMAXPROCS $binary_cpus, -parallel $test_workers)"
  PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
  export PHX_TEST_RUN_ID
  backend_go_phase=1
  backend_go_log=$(mktemp "${TMPDIR:-/tmp}/phoenix-test-changed-go.XXXXXX")
  summarize_backend_go_failure() {
    echo "==> go test failure summary" >&2
    summary=$(grep -E '^(FAIL$|FAIL[[:space:]]|--- FAIL:)|panic:|Error Trace:|Received unexpected error|Not equal:|Should be' "$backend_go_log" | tail -200 || true)
    if [ -n "$summary" ]; then
      printf '%s\n' "$summary" >&2
    else
      tail -120 "$backend_go_log" >&2
    fi
  }
  backend_sweep() {
    status=$?
    if [ "$status" -ne 0 ] && [ "$backend_go_phase" -eq 1 ] && [ -f "$backend_go_log" ]; then
      summarize_backend_go_failure
    fi
    if ! (cd "$repo_root/backend" && go run ./internal/testdb/cmd/sweep) &&
      [ "$status" -eq 0 ]; then
      status=1
    fi
    rm -f "$backend_go_log"
    return "$status"
  }
  trap backend_sweep EXIT
  (cd backend && GOMAXPROCS="$binary_cpus" go test \
    -p "$package_workers" -parallel "$test_workers" "${affected[@]}") 2>&1 | tee "$backend_go_log"
  backend_go_phase=0
else
  echo "==> backend: keine Go-Änderungen"
fi

frontend_changes=$(
  {
    git diff --name-only "$MB" -- frontend
    git ls-files --others --exclude-standard -- frontend
  } | awk '!/\.md$/' | sort -u
)
if printf '%s\n' "$frontend_changes" | grep -qxE \
  'frontend/(package\.json|pnpm-lock\.yaml|pnpm-workspace\.yaml|tsconfig\.json|vitest\.config\.ts|src/test/setup(-common)?\.ts)'; then
  echo "==> vitest (frontend test infrastructure changed)"
  (cd frontend && pnpm vitest run)
elif [ -n "$frontend_changes" ]; then
  # Vitest 4 includes committed, staged, unstaged, and untracked files in its
  # dependency traversal. A separate full-suite fallback only duplicates work
  # for new test files.
  echo "==> vitest --changed $BASE"
  (cd frontend && pnpm vitest run --changed "$BASE")
else
  echo "==> frontend: keine Änderungen"
fi
