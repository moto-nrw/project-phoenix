#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, deren Produktions- oder
#   Test-Abhängigkeiten sie transitiv importieren.
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [--fast] [base-ref]   (Default: origin/development)
#   --fast: nur direkt geänderte Packages plus ihre direkten Importer
#           (Depth 1 statt transitiver Closure, ohne das Ratchet-Package
#           ./test). Für die schnelle Fix-Iteration; vor dem Push einmal
#           ohne --fast laufen lassen.
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

FAST=false
if [ "${1:-}" = --fast ]; then
  FAST=true
  shift
fi
BASE=${1:-origin/development}
# Merge-Base statt Drei-Punkt-Diff, damit auch uncommittete Änderungen zählen.
MB=$(git merge-base HEAD "$BASE")

affected=()
if [ "$FAST" = true ]; then
  echo "==> Schnellmodus: nur direkt geänderte Packages und ihre direkten Importer. Vor dem Push ohne --fast laufen lassen."
  affected_output=$(scripts/backend-affected-packages.sh --direct "$BASE")
else
  affected_output=$(scripts/backend-affected-packages.sh "$BASE")
fi
while IFS= read -r package; do
  [ -n "$package" ] && affected+=("$package")
done <<< "$affected_output"

if [ "${#affected[@]}" -gt 0 ]; then
  cpu_count=$(getconf _NPROCESSORS_ONLN)
  if ! [[ "$cpu_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "getconf returned an invalid CPU count: $cpu_count" >&2
    exit 1
  fi
  # Half the machine (at most eight package binaries) keeps the changed-test
  # loop usable on weak hardware. Each DB-backed binary opens 13 connections
  # at -parallel 8 (backend/test/db_clone.go), so 8 x 13 = 104 stays well
  # under the local postgres-test max_connections=300. -parallel is pinned:
  # -test.parallel is part of the Go test cache key, and a CPU-derived value
  # would split the cache universe per machine. No GOMAXPROCS override for
  # the same reason as in test-backend.sh: -p bounds the load.
  package_workers=$((cpu_count / 2))
  if [ "$package_workers" -lt 1 ]; then
    package_workers=1
  elif [ "$package_workers" -gt 8 ]; then
    package_workers=8
  fi

  echo "==> go test (${#affected[@]} affected packages; -p $package_workers, -parallel 8)"
  backend_go_phase=0
  backend_go_log=
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
    [ -n "${PHX_TEST_RUN_LOCK:-}" ] && rm -f "$PHX_TEST_RUN_LOCK"
    rm -f "$backend_go_log"
    return "$status"
  }
  # Stabile Run-ID pro Worktree (Cache-Hebel) plus Overlap-Lock; Details im
  # Helper. Der Trap muss vor dem Bootstrap stehen, damit dessen Fehlschlag
  # das gerade erworbene Lock ebenfalls aufräumt.
  # shellcheck source=scripts/test-run-id.sh
  source "$repo_root/scripts/test-run-id.sh"
  trap backend_sweep EXIT
  # Handshake einmal pro Lauf statt einmal pro Binary. Jedes Binary liest
  # PHX_TEST_TEMPLATE, die Variable ist Teil des Go-Test-Cache-Keys und muss
  # deshalb zwischen beiden Wrappern uebereinstimmen (test-backend.sh setzt
  # sie genauso).
  PHX_TEST_TEMPLATE=$(cd backend && go run ./internal/testdb/cmd/bootstrap)
  export PHX_TEST_TEMPLATE
  backend_go_log=$(mktemp "${TMPDIR:-/tmp}/phoenix-test-changed-go.XXXXXX")
  backend_go_phase=1
  (cd backend && go test \
    -p "$package_workers" -parallel 8 "${affected[@]}") 2>&1 | tee "$backend_go_log"
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
run_frontend_vitest() {
  (
    cd frontend
    pnpm install --frozen-lockfile
    pnpm vitest run "$@"
  )
}

if printf '%s\n' "$frontend_changes" | grep -qxE \
  'frontend/(package\.json|pnpm-lock\.yaml|pnpm-workspace\.yaml|tsconfig\.json|vitest\.config\.ts|src/test/setup(-common)?\.ts)'; then
  echo "==> vitest (frontend test infrastructure changed)"
  run_frontend_vitest
elif [ -n "$frontend_changes" ]; then
  # Vitest 4 includes committed, staged, unstaged, and untracked files in its
  # dependency traversal. A separate full-suite fallback only duplicates work
  # for new test files.
  echo "==> vitest --changed $BASE"
  run_frontend_vitest --changed "$BASE"
else
  echo "==> frontend: keine Änderungen"
fi
