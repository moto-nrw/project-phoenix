#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages PLUS alle Packages, die sie (transitiv)
#   importieren — via `go list`. Bekannte Lücke: test-only-Importketten über
#   zwei Ebenen (Testdatei -> Helper-Package -> geändertes Package) werden
#   nicht erkannt; der exakte Mechanismus bleibt Gos Test-Cache beim vollen
#   `go test ./...`. Das hier ist der Inner-Loop, kein Pre-Merge-Ersatz.
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [base-ref]   (Default: origin/development)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

BASE=${1:-origin/development}
# Merge-Base statt Drei-Punkt-Diff, damit auch uncommittete Änderungen zählen.
MB=$(git merge-base HEAD "$BASE")

dirs=$(git diff --name-only "$MB" -- 'backend/**/*.go' 'backend/*.go' \
  | xargs -r -n1 dirname | sed 's|^backend/\{0,1\}||' | sort -u)
if [ -n "$dirs" ]; then
  cd backend
  mod=$(go list -m)
  changed=$(echo "$dirs" | sed "s|^|$mod/|; s|/\$||")
  # Betroffen = geändertes Package selbst ODER eines seiner (transitiven)
  # Deps bzw. direkten Test-Imports ist geändert. Substring-Match auf
  # Import-Pfade kann minimal über-selektieren (foo trifft auch foo/bar) —
  # zu viel testen ist ok, zu wenig nicht.
  affected=$(go list -e -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \
    | grep -F -f <(echo "$changed") | cut -d' ' -f1 | sort -u)
  echo "==> go test ($(echo "$affected" | wc -l | tr -d ' ') affected packages, $(echo "$dirs" | wc -l | tr -d ' ') changed)"
  # shellcheck disable=SC2086
  go test $affected
  cd ..
else
  echo "==> backend: keine Go-Änderungen"
fi

if ! git diff --quiet "$MB" -- frontend; then
  echo "==> vitest --changed $BASE"
  (cd frontend && pnpm vitest run --changed "$BASE")
else
  echo "==> frontend: keine Änderungen"
fi
