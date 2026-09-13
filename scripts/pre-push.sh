#!/usr/bin/env bash
# Validate the complete PR diff with input-keyed static checks and stage timings.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
# Git hooks export checkout-local variables; child tools and fixture repos
# must resolve their own working directories, as in backend-architecture.sh.
while IFS= read -r git_variable; do
  unset "$git_variable"
done < <(git rev-parse --local-env-vars)
source "$root/scripts/quality-timing.sh"
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
  quality_step fetch-base git fetch --quiet origin development
fi
merge_base=$(git merge-base "$head" "$base")
backend=false
frontend=false
context=false
env_sync=false
hooks=false
changed=$(mktemp)
trap 'rm -f "$changed"' EXIT
git diff --name-only -z "$merge_base" "$head" > "$changed"
while IFS= read -r -d '' file; do
  case "$file" in
    *.md) context=true ;;
    backend/*|scripts/backend-*|scripts/check-backend-quality.sh|scripts/check-deadcode*) backend=true ;;
    frontend/*|scripts/check-frontend-quality.sh) frontend=true ;;
    environments/*|.env.example) env_sync=true ;;
    scripts/check-quality.sh|scripts/quality-timing.sh|devbox.*|package.json) backend=true; frontend=true; hooks=true ;;
    scripts/pre-push*|scripts/check-secrets.sh|scripts/run-go-toolchain.sh|lefthook.yml|.github/*|scripts/test-*) hooks=true ;;
    .agents/*|.claude/*|.codex/*|scripts/check-agent-context*) context=true ;;
  esac
done < "$changed"
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Pre-push requires a clean working tree. Commit or stash changes, then push again.' >&2
  exit 1
fi
printf 'Pre-push %s..%s: backend=%s frontend=%s context=%s hooks=%s\n' "$merge_base" "$head" "$backend" "$frontend" "$context" "$hooks"
export PATH="$root/.devbox/nix/profile/default/bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=${CGO_ENABLED:-0}
require_pinned() {
  if [[ ! -x "$root/.devbox/nix/profile/default/bin/$1" ]]; then
    echo "Pinned $1 is missing. Run 'devbox install'." >&2
    exit 1
  fi
}
cached() {
  local stage=$1
  shift
  node scripts/pre-push-cache.mjs "$stage" "$base" "$@"
}
quality_step secrets bash scripts/check-secrets.sh --scan
require_pinned node
if [[ "$context" == true || "$hooks" == true ]]; then
  quality_step context node scripts/check-agent-context.mjs
  quality_step context-tests node --test scripts/check-agent-context.test.mjs
fi
if [[ "$hooks" == true ]]; then
  cached hook-tests node --test scripts/pre-push.test.mjs
fi
if [[ "$env_sync" == true ]]; then
  quality_step env-sync bash scripts/env-check.sh
fi
if [[ "$backend" == true ]]; then
  # Validate installed tools even on cache hits. Keep vulnerability results
  # fresh; the external vulnerability database is not a static cache input.
  scripts/run-go-toolchain.sh go version >/dev/null
  require_pinned golangci-lint
  require_pinned govulncheck
  cached backend-quality scripts/run-go-toolchain.sh scripts/check-quality.sh backend
  cached backend-lint scripts/run-go-toolchain.sh scripts/check-backend-quality.sh lint "$base"
  architecture_base=$(git rev-parse "$base")
  cached architecture scripts/run-go-toolchain.sh scripts/backend-architecture.sh check --base-ref "$architecture_base"
  quality_step vulnerabilities scripts/run-go-toolchain.sh scripts/check-backend-quality.sh vulnerabilities
fi
if [[ "$frontend" == true ]]; then
  require_pinned pnpm
  require_pinned npx
  # Frozen installation verifies the dependency state before looking up results.
  quality_step frontend-install bash -c 'cd frontend && CI=true pnpm install --frozen-lockfile'
  cached frontend-quality bash scripts/check-quality.sh frontend
  cached react-doctor bash scripts/check-quality.sh react-doctor "$base"
fi
# Integration/affected tests run in CI, not on every push. Explicit pre-push
# verification by contributors still uses scripts/test-changed.sh.
if [[ "$(git rev-parse HEAD)" != "$head" || -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Source changed during pre-push. Commit or stash changes and retry.' >&2
  exit 1
fi
echo "Pre-push quality checks passed in ${SECONDS}s."
