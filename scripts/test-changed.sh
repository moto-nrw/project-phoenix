#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, deren Produktions- oder
#   Test-Abhängigkeiten sie transitiv importieren.
# Frontend: Dateisystem-Guards plus vitest --changed (wie im PR-Check).
#
# Usage: scripts/test-changed.sh [--fast] [base-ref]   (Default: origin/development)
#   --fast: nur direkt geänderte Packages plus ihre direkten Importer
#           (Depth 1 statt transitiver Closure) und gezielt betroffene
#           Quelltext-Ratchets aus ./test. CI läuft die volle Closure.
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
# shellcheck source=scripts/go-test-workspace.sh
source "$repo_root/scripts/go-test-workspace.sh"

FAST=false
if [ "${1:-}" = --fast ]; then
  FAST=true
  shift
fi
BASE=${1:-origin/development}

affected=()
if [ "$FAST" = true ]; then
  echo "==> Schnellmodus: geänderte Packages, direkte Importer und betroffene Quelltext-Ratchets. CI läuft die volle Closure."
  affected_output=$(scripts/backend-affected-packages.sh --direct "$BASE")
else
  affected_output=$(scripts/backend-affected-packages.sh "$BASE")
fi
while IFS= read -r package; do
  [ -n "$package" ] && affected+=("$package")
done <<< "$affected_output"

# Go imports do not describe repository files read by tests. Keep the full CI
# selection unchanged; the fast loop adds only the source checks that read a
# changed path. A full ./test selection already covers them without a second run.
if [ "$FAST" = true ]; then
  test_package=$(awk '$1 == "module" { print $2 "/test"; exit }' backend/go.mod)
  if [[ "$affected_output" != './...' ]] &&
    ! printf '%s\n' "$affected_output" | grep -qxF -e './test' -e "$test_package"; then
    source_ratchets=$(node "$repo_root/scripts/backend-source-ratchets.mjs" "$BASE")
    if [ -n "$source_ratchets" ]; then
      echo "==> backend source ratchets: $source_ratchets"
      (
        trap cleanup_go_test_workspace EXIT
        prepare_go_test_workspace
        cd backend
        # Walked/untracked/deleted inputs are not Go import dependencies. Always
        # execute the scanners instead of trusting a cached test result.
        go test -p "$("$repo_root/scripts/go-package-workers.sh")" -parallel 8 -tags=testhooks -count=1 ./test -run "$source_ratchets"
      )
    fi
  fi
fi

# A new migration that sorts below the newest one of the base reaches deployed
# databases after it. TestLateMigrationsYieldTheFreshSchema replays that order
# for the migrations named here (ADR 0223); without one it has nothing to do.
if [ "${#affected[@]}" -gt 0 ] && [ -z "${PHX_LATE_MIGRATIONS+set}" ] &&
  [ -n "$(git diff --no-renames --diff-filter=A --name-only "$BASE" -- backend/database/migrations
    git ls-files --others --exclude-standard -- backend/database/migrations)" ]; then
  PHX_LATE_MIGRATIONS=$(node "$repo_root/scripts/check-migration-versions.mjs" --late "$BASE")
  export PHX_LATE_MIGRATIONS
  if [ -n "$PHX_LATE_MIGRATIONS" ]; then
    echo "==> late migrations, replayed in deployment order: $PHX_LATE_MIGRATIONS"
    # The replay runs the role migrations, which refuse an empty password.
    # The roles exist on the test server, so this fixture value is not applied.
    export PHOENIX_AUTH_PASSWORD="${PHOENIX_AUTH_PASSWORD:-phoenix_auth_test}"
  fi
fi

if [ "${#affected[@]}" -gt 0 ]; then
  # Match the full suite's CPU/RAM/I/O budget. -parallel remains a stable
  # test-cache and DB-pool input rather than following the machine's CPUs.
  package_workers=$("$repo_root/scripts/go-package-workers.sh")

  echo "==> go test (${#affected[@]} affected packages; -p $package_workers, -parallel 8)"
  backend_go_phase=0
  backend_go_log=
  summarize_backend_go_failure() {
    echo "==> go test failure summary" >&2
    # Indented file_test.go:N lines carry t.Error/t.Fatal messages; go test
    # prints them only for failed tests.
    summary=$(grep -E '^(FAIL$|FAIL[[:space:]]|--- FAIL:)|^[[:space:]]+(--- FAIL:|[^[:space:]]+_test\.go:[0-9]+: )|panic:|Error Trace:|Received unexpected error|Not equal:|Should be' "$backend_go_log" | tail -200 || true)
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
      # Publish atomically: an overlapping run (throwaway run ID) may fail too.
      if gzip -c "$backend_go_log" >"$backend_go_published.$$" &&
        mv -f "$backend_go_published.$$" "$backend_go_published"; then
        echo "==> full go test output: $backend_go_published ($(du -h "$backend_go_published" | cut -f1))" >&2
      fi
    fi
    if ! (cd "$repo_root/backend" && go run ./internal/testdb/cmd/sweep) &&
      [ "$status" -eq 0 ]; then
      status=1
    fi
    [ -n "${PHX_TEST_RUN_LOCK:-}" ] && rm -f "$PHX_TEST_RUN_LOCK"
    rm -f "$backend_go_log"
    if ! cleanup_go_test_workspace && [ "$status" -eq 0 ]; then status=1; fi
    return "$status"
  }
  # Stabile Run-ID pro Worktree (Cache-Hebel) plus Overlap-Lock; Details im
  # Helper. Der Trap muss vor dem Bootstrap stehen, damit dessen Fehlschlag
  # das gerade erworbene Lock ebenfalls aufräumt.
  # shellcheck source=scripts/test-run-id.sh
  source "$repo_root/scripts/test-run-id.sh"
  trap backend_sweep EXIT
  prepare_go_test_workspace
  # Handshake einmal pro Lauf statt einmal pro Binary. Jedes Binary liest
  # PHX_TEST_TEMPLATE, die Variable ist Teil des Go-Test-Cache-Keys und muss
  # deshalb zwischen beiden Wrappern uebereinstimmen (test-backend.sh setzt
  # sie genauso).
  PHX_TEST_TEMPLATE=$(cd backend && go run ./internal/testdb/cmd/bootstrap)
  export PHX_TEST_TEMPLATE
  # A failed package prints everything its binary wrote, once 494 MB from one
  # api run. Show only package results and build errors live; a failure adds
  # the summary above and keeps the last failed run's log, compressed, per worktree.
  backend_go_log=$(mktemp "${TMPDIR:-/tmp}/phoenix-test-changed-go.XXXXXX")
  backend_go_published=$(git rev-parse --path-format=absolute --git-path test-changed-go.log.gz)
  backend_go_phase=1
  (cd backend && go test \
    -p "$package_workers" -parallel 8 -tags=testhooks "${affected[@]}") 2>&1 | tee "$backend_go_log" |
    awk '/^(ok|FAIL|\?|--- FAIL|panic:|# )/ || /^[^[:space:]]+\.go:[0-9]+:[0-9]+: / { print; fflush() }'
  backend_go_phase=0
else
  echo "==> backend: keine Go-Änderungen"
fi

# One fail-closed policy owns local and CI selection, including untracked files.
frontend_mode=$(node scripts/test-frontend-changed.mjs --mode "$BASE")
if [ "$frontend_mode" != none ]; then
  (
    cd frontend
    CI=true pnpm install --frozen-lockfile
    node ../scripts/test-frontend-changed.mjs "$BASE"
  )
else
  echo "==> frontend: keine Änderungen"
fi
