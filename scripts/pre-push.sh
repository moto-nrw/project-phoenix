#!/usr/bin/env bash
# Fast local gate for the complete PR diff: secrets, guidance, lint, types and
# unused code (knip).
# Expensive whole-repo analysis (build, ratchets, deadcode, architecture,
# react-doctor) and tests run in CI; license and vulnerability scans nightly.
# PHX_PREPUSH=skip skips the gate; CI still checks the push.
# --worktree checks uncommitted and untracked changes too, without result
# caching (scripts/check-changed.sh uses it while an agent iterates).
set -euo pipefail
if [[ "${PHX_PREPUSH:-}" == skip ]]; then
  echo 'Pre-push skipped (PHX_PREPUSH=skip). CI still checks this push.'
  exit 0
fi
root=$(git rev-parse --show-toplevel)
cd "$root"
# Git hooks export checkout-local variables; child tools and fixture repos
# must resolve their own working directories, as in backend-architecture.sh.
while IFS= read -r git_variable; do
  unset "$git_variable"
done < <(git rev-parse --local-env-vars)
source "$root/scripts/quality-timing.sh"
head=$(git rev-parse HEAD)
worktree=false
if [[ "${1:-}" == --worktree ]]; then
  worktree=true
  shift
elif [[ "${1:-}" == --hook ]]; then
  shift
  updates=false
  pushed_remote=
  while read -r local_ref local_oid remote_ref remote_oid; do
    [[ "$local_oid" != 0000000000000000000000000000000000000000 ]] || continue
    updates=true
    pushed_remote=$remote_oid
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
# A conflict-free merge of the base onto the already pushed branch adds only
# code that CI checked on the base; re-validating it locally buys nothing.
if [[ -n "${pushed_remote:-}" && "$pushed_remote" != 0000000000000000000000000000000000000000 ]] &&
  git rev-parse -q --verify "$head^2" >/dev/null &&
  [[ "$(git rev-parse "$head^1")" == "$pushed_remote" ]] &&
  git merge-base --is-ancestor "$head^2" "$base" &&
  [[ -z "$(git diff-tree --cc --no-commit-id -p "$head")" ]]; then
  echo 'Pre-push skipped: conflict-free merge of the base. CI still checks this push.'
  exit 0
fi
backend=false
frontend=false
context=false
env_sync=false
hooks=false
# Keep the diff and child-tool scratch files off a quota-limited system /tmp.
# Git metadata also works in linked worktrees without dirtying the checkout.
workspace_parent=$(git rev-parse --path-format=absolute --git-path pre-push-cache)
mkdir -p "$workspace_parent"
workspace=$(mktemp -d "$workspace_parent/work.XXXXXX")
trap 'rm -r -- "$workspace"' EXIT
export TMPDIR="${TMPDIR:-$workspace}"
changed="$workspace/changed"
if [[ "$worktree" == true ]]; then
  { git diff --name-only -z "$merge_base"; git ls-files --others --exclude-standard -z; } > "$changed"
else
  git diff --name-only -z "$merge_base" "$head" > "$changed"
fi
while IFS= read -r -d '' file; do
  case "$file" in
    *.md) context=true ;;
    backend/*|scripts/backend-*|scripts/check-backend-quality.sh|scripts/check-deadcode*) backend=true ;;
    frontend/*|scripts/check-frontend-quality.sh) frontend=true ;;
    environments/*|.env.example) env_sync=true ;;
    scripts/check-quality.sh|scripts/quality-timing.sh|devbox.*|package.json) backend=true; frontend=true; hooks=true ;;
    scripts/pre-push*|scripts/check-secrets*|.gitleaks*|scripts/run-go-toolchain.sh|scripts/dev-native*|scripts/go-package-workers.sh|scripts/total-memory-gb.sh|lefthook.yml|.github/*|scripts/test-*) hooks=true ;;
    .agents/*|.claude/*|.codex/*|scripts/check-agent-context*) context=true ;;
  esac
done < "$changed"
if [[ "$worktree" == false && -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Pre-push requires a clean working tree. Commit or stash changes, then push again.' >&2
  exit 1
fi
printf 'Pre-push %s..%s: backend=%s frontend=%s context=%s hooks=%s\n' "$merge_base" "$head" "$backend" "$frontend" "$context" "$hooks"
export PATH="$root/.devbox/nix/profile/default/bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=${CGO_ENABLED:-0}
# Cache inputs must see the same budget that the pinned runner exports.
export GOFLAGS="${GOFLAGS--p=4}"
export GOMAXPROCS="${GOMAXPROCS-4}"
require_pinned() {
  if [[ ! -x "$root/.devbox/nix/profile/default/bin/$1" ]]; then
    echo "Pinned $1 is missing. Run 'devbox install'." >&2
    exit 1
  fi
}
cached() {
  local stage=$1
  shift
  # Keyed by the merge-base, so new base commits keep results for the same diff.
  if [[ "$worktree" == true ]]; then
    quality_step "$stage" "$@"
  else
    node scripts/pre-push-cache.mjs "$stage" "$merge_base" "$@"
  fi
}
quality_step secrets bash scripts/check-secrets.sh pre-push "$base"
require_pinned node
if [[ "$context" == true || "$hooks" == true ]]; then
  quality_step context node scripts/check-agent-context.mjs
  quality_step context-tests node --test scripts/check-agent-context.test.mjs
fi
if [[ "$hooks" == true ]]; then
  cached hook-tests node --test scripts/pre-push.test.mjs scripts/check-secrets.test.mjs
fi
if [[ "$env_sync" == true ]]; then
  quality_step env-sync bash scripts/env-check.sh
fi
if [[ "$backend" == true ]]; then
  # Validate installed tools even on cache hits. Keep vulnerability results
  # fresh; the external vulnerability database is not a static cache input.
  scripts/run-go-toolchain.sh go version >/dev/null
  # golangci-lint type-checks the affected packages, so it also catches build errors.
  require_pinned golangci-lint
  cached backend-lint scripts/run-go-toolchain.sh scripts/check-backend-quality.sh lint "$base"
fi
if [[ "$frontend" == true ]]; then
  require_pinned pnpm
  require_pinned npx
  # Frozen installation verifies the dependency state before looking up results.
  quality_step frontend-install bash -c 'cd frontend && CI=true pnpm install --frozen-lockfile'
  cached frontend-quality bash scripts/check-quality.sh frontend-fast
fi
# Integration/affected tests run in CI, not on every push. Explicit pre-push
# verification by contributors still uses scripts/test-changed.sh.
if [[ "$worktree" == false ]] && [[ "$(git rev-parse HEAD)" != "$head" || -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo 'Source changed during pre-push. Commit or stash changes and retry.' >&2
  exit 1
fi
echo "Pre-push quality checks passed in ${SECONDS}s."
