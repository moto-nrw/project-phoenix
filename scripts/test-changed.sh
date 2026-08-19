#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, deren Produktions- oder
#   Test-Abhängigkeiten sie transitiv importieren — via `go list -test`.
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [base-ref]   (Default: origin/development)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

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
  changed=$(echo "$dirs" | sed "s|^|$mod/|; s|/\$||")
  # `-test` ergänzt die transitive Abhängigkeitskette der Test-Binaries.
  # Substring-Match auf Import-Pfade kann minimal über-selektieren (foo trifft
  # auch foo/bar) — zu viel testen ist ok, zu wenig nicht.
  affected=$(go list -e -test -f '{{if .ForTest}}{{.ForTest}} {{join .Deps " "}}{{end}}' ./... \
    | grep -F -f <(echo "$changed") | cut -d' ' -f1 | sort -u)
  echo "==> go test ($(echo "$affected" | wc -l | tr -d ' ') affected packages, $(echo "$dirs" | wc -l | tr -d ' ') changed)"
  # shellcheck disable=SC2086
  go test $affected
  cd ..
elif [ "$backend_tested" = false ]; then
  echo "==> backend: keine Go-Änderungen"
fi

if ! git diff --quiet "$MB" -- frontend; then
  echo "==> vitest --changed $BASE"
  (cd frontend && pnpm vitest run --changed "$BASE")
else
  echo "==> frontend: keine Änderungen"
fi
