#!/usr/bin/env bash
# Validate the complete PR diff. Reuse tool caches, not custom success stamps.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
# Git hooks export checkout-local variables; child tools and fixture repos
# must resolve their own working directories, as in backend-architecture.sh.
while IFS= read -r git_variable; do
  unset "$git_variable"
done < <(git rev-parse --local-env-vars)
head=$(git rev-parse HEAD)
if [[ "${1:-}" == --hook ]]; then
  shift
  updates=false
  while read -r local_ref local_oid remote_ref remote_oid; do
    [[ "$local_oid" != 0000000000000000000000000000000000000000 ]] || continue
    updates=true
    if [[ "$remote_ref" != refs/heads/* || "$local_oid" != "$head" ]]; then
      echo 'Push one checked-out branch at a time so the hook checks the code being pushed.' >&2
      exit 1
    fi
  done
  [[ "$updates" == true ]] || exit 0
fi
base=${1:-origin/development}
# A failed fetch must not validate stale inputs.
if [[ "$base" == origin/development ]]; then
  git fetch --quiet origin development
fi
merge_base=$(git merge-base "$head" "$base")
backend=false
full_backend=false
frontend=false
context=false
env_sync=false
hooks=false
changed=$(mktemp)
trap 'rm -f "$changed"' EXIT
git diff --name-only -z "$merge_base" "$head" > "$changed"
while IFS= read -r -d '' file; do
  case "$file" in
    backend/go.mod|backend/go.sum|backend/.golangci.yml|scripts/backend-affected-packages*|scripts/backend-lint-packages.sh)
      full_backend=true ;;
  esac
  case "$file" in
    *.md) context=true ;;
    backend/*|scripts/backend-*|scripts/test-backend*|scripts/test-changed*|scripts/test-run-id.sh|scripts/check-deadcode*) backend=true ;;
    frontend/*) frontend=true ;;
    environments/*|.env.example) env_sync=true ;;
    scripts/pre-push*|scripts/check-quality.sh|scripts/check-secrets.sh|scripts/run-go-toolchain.sh|lefthook.yml|devbox.*|package.json|.github/*) backend=true; frontend=true; hooks=true ;;
    .agents/*|.claude/*|.codex/*|scripts/check-agent-context*) context=true ;;
  esac
done < "$changed"
# Uncommitted fixes must not mask broken committed code. Ignored local config
# and build caches stay usable; other changes must be committed or stashed.
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Pre-push requires a clean working tree. Commit or stash changes, then push again.' >&2
  exit 1
fi
printf 'Pre-push %s..%s: backend=%s frontend=%s context=%s hooks=%s\n' "$merge_base" "$head" "$backend" "$frontend" "$context" "$hooks"
export PATH="$root/.devbox/nix/profile/default/bin:$PATH"
require_pinned() {
  if [[ ! -x "$root/.devbox/nix/profile/default/bin/$1" ]]; then
    echo "Pinned $1 is missing. Run 'devbox install'." >&2
    exit 1
  fi
}
bash scripts/check-secrets.sh --scan
if [[ "$context" == true || "$hooks" == true ]]; then
  require_pinned node
  node scripts/check-agent-context.mjs
  node --test scripts/check-agent-context.test.mjs
fi
if [[ "$hooks" == true ]]; then
  node --test scripts/pre-push.test.mjs
fi
if [[ "$env_sync" == true ]]; then
  bash scripts/env-check.sh
fi
if [[ "$backend" == true ]]; then
  scripts/run-go-toolchain.sh scripts/check-quality.sh backend
  # Sequential whole-program checks bound memory. Lint uses CI's transitive
  # affected-package selector, falling back to full lint for config-only edits.
  if [[ "$full_backend" == true || "$hooks" == true ]]; then
    packages_output=./...
  else
    packages_output=$(scripts/run-go-toolchain.sh scripts/backend-affected-packages.sh "$base")
  fi
  packages=()
  while IFS= read -r package; do
    [[ -z "$package" ]] || packages+=("$package")
  done <<< "$packages_output"
  [[ ${#packages[@]} -gt 0 ]] || packages=(./...)
  (cd backend && ../scripts/run-go-toolchain.sh golangci-lint run --allow-serial-runners --timeout 20m "${packages[@]}")
  architecture_base=$(git rev-parse "$base")
  scripts/run-go-toolchain.sh scripts/backend-architecture.sh check --base-ref "$architecture_base"
  (cd backend && ../scripts/run-go-toolchain.sh govulncheck ./...)
fi
if [[ "$frontend" == true ]]; then
  require_pinned node
  require_pinned pnpm
  require_pinned npx
  (cd frontend && CI=true pnpm install --frozen-lockfile)
  bash scripts/check-quality.sh frontend
  bash scripts/check-quality.sh react-doctor "$base"
fi
if [[ "$backend" == true || "$frontend" == true ]]; then
  scripts/run-go-toolchain.sh scripts/test-changed.sh "$base"
fi
if [[ "$(git rev-parse HEAD)" != "$head" || -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Source changed during pre-push. Commit or stash changes and retry.' >&2
  exit 1
fi
echo "Pre-push quality checks passed in ${SECONDS}s."
