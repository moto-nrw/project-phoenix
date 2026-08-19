#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, deren Produktions- oder
#   Test-Abhängigkeiten sie transitiv importieren — via `go list -test`.
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [base-ref]   (Default: origin/development)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Backend-Läufe stempeln ihre DB-Clones mit einer Run-ID; der Sweep am Ende
# droppt sie wieder und sammelt Clones toter Läufe ein (ADR 0004).
PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
export PHX_TEST_RUN_ID
backend_sweep() { (cd backend && go run ./internal/testdb/cmd/sweep) || true; }
trap backend_sweep EXIT

BASE=${1:-origin/development}
# Merge-Base statt Drei-Punkt-Diff, damit auch uncommittete Änderungen zählen.
MB=$(git merge-base HEAD "$BASE")

backend_changes=$(
  {
    git diff --name-only "$MB" -- backend
    git ls-files --others --exclude-standard -- backend
  } | sort -u
)

backend_tested=false
if printf '%s\n' "$backend_changes" | grep -qxE 'backend/go\.(mod|sum)'; then
  echo "==> go test (backend dependency changes)"
  (cd backend && go test ./...)
  backend_tested=true
else
  dirs=$(printf '%s\n' "$backend_changes" | awk '
    /^backend\/[^\/]+\.go$/ { print "."; next }
    /^backend\/.+\.go$/ {
      path = $0
      sub(/^backend\//, "", path)
      sub(/\/[^\/]+$/, "", path)
      print path
    }
  ' | sort -u)
fi

if [ -n "${dirs:-}" ]; then
  cd backend
  mod=$(go list -m)
  changed=$(printf '%s\n' "$dirs" | awk -v mod="$mod" '
    $0 == "." { print mod; next }
    { print mod "/" $0 }
  ')
  # `-test` ergänzt die transitive Abhängigkeitskette der Test-Binaries.
  # Substring-Match auf Import-Pfade kann minimal über-selektieren (foo trifft
  # auch foo/bar) — zu viel testen ist ok, zu wenig nicht.
  affected=$(
    {
      printf '%s\n' "$changed"
      go list -test -f '{{if .ForTest}}{{.ForTest}} {{join .Deps " "}}{{end}}' ./... \
        | { grep -F -f <(printf '%s\n' "$changed") || true; } \
        | cut -d' ' -f1
    } | sort -u
  )
  if [ -z "$affected" ]; then
    echo "==> backend: keine betroffenen Packages ermittelt" >&2
    exit 1
  fi
  echo "==> go test ($(echo "$affected" | wc -l | tr -d ' ') affected packages, $(echo "$dirs" | wc -l | tr -d ' ') changed)"
  # shellcheck disable=SC2086
  go test $affected
  cd ..
elif [ "$backend_tested" = false ]; then
  echo "==> backend: keine Go-Änderungen"
fi

frontend_untracked=$(git ls-files --others --exclude-standard -- frontend)
if [ -n "$frontend_untracked" ]; then
  echo "==> vitest (unversionierte Frontend-Dateien)"
  (cd frontend && pnpm vitest run)
elif ! git diff --quiet "$MB" -- frontend; then
  echo "==> vitest --changed $BASE"
  (cd frontend && pnpm vitest run --changed "$BASE")
else
  echo "==> frontend: keine Änderungen"
fi
