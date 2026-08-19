#!/usr/bin/env bash
# Läuft nur die Tests, die von den Änderungen gegenüber BASE betroffen sind.
# Backend: geänderte Go-Packages (go test findet Abhängige über den Test-Cache
# ohnehin nicht — dafür ist der volle Lauf da; das hier ist der Inner-Loop).
# Frontend: vitest --changed (derselbe Modus, den CI für PRs benutzt).
#
# Usage: scripts/test-changed.sh [base-ref]   (Default: origin/development)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

BASE=${1:-origin/development}
# Merge-Base statt Drei-Punkt-Diff, damit auch uncommittete Änderungen zählen.
MB=$(git merge-base HEAD "$BASE")

pkgs=$(git diff --name-only "$MB" -- 'backend/**/*.go' 'backend/*.go' \
  | xargs -r -n1 dirname | sed 's|^backend|.|' | sort -u)
if [ -n "$pkgs" ]; then
  echo "==> go test ($(echo "$pkgs" | wc -l | tr -d ' ') packages)"
  # shellcheck disable=SC2086
  (cd backend && go test $pkgs)
else
  echo "==> backend: keine Go-Änderungen"
fi

if ! git diff --quiet "$MB" -- frontend; then
  echo "==> vitest --changed $BASE"
  (cd frontend && pnpm vitest run --changed "$BASE")
else
  echo "==> frontend: keine Änderungen"
fi
