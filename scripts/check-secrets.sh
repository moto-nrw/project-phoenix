#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
bin="$root/.devbox/nix/profile/default/bin"
if [[ ! -x "$bin/gitleaks" ]]; then
  echo "Pinned gitleaks is missing. Run 'devbox install'." >&2
  exit 1
fi
case "${1:-}" in
  pre-commit)
    [[ $# -eq 1 ]] || exit 2
    scan=(--staged --pre-commit)
    ;;
  pre-push)
    [[ $# -eq 2 ]] || { echo 'Usage: check-secrets.sh pre-push <base-ref>' >&2; exit 2; }
    merge_base=$(git merge-base "$2" HEAD)
    # No branch commits to scan.
    if [[ "$merge_base" == "$(git rev-parse HEAD)" ]]; then
      exit 0
    fi
    scan=(--log-opts="$merge_base..HEAD")
    ;;
  worktree)
    # Branch history plus uncommitted and untracked changes: snapshot the
    # working tree into an unreferenced commit on HEAD, so path allowlists
    # still apply and committed-then-removed secrets stay in the range.
    [[ $# -eq 2 ]] || { echo 'Usage: check-secrets.sh worktree <base-ref>' >&2; exit 2; }
    merge_base=$(git merge-base "$2" HEAD)
    index=$(mktemp)
    cp "$(git rev-parse --git-path index)" "$index"
    GIT_INDEX_FILE=$index git add -A
    tree=$(GIT_INDEX_FILE=$index git write-tree)
    rm -f "$index"
    tip=$(git rev-parse HEAD)
    if [[ "$tree" != "$(git rev-parse "HEAD^{tree}")" ]]; then
      tip=$(GIT_AUTHOR_NAME=check-secrets GIT_AUTHOR_EMAIL=check-secrets@localhost \
        GIT_COMMITTER_NAME=check-secrets GIT_COMMITTER_EMAIL=check-secrets@localhost \
        git commit-tree "$tree" -p HEAD -m 'check-secrets working tree snapshot')
    fi
    # Nothing on the branch and nothing uncommitted.
    if [[ "$tip" == "$merge_base" ]]; then
      exit 0
    fi
    scan=(--log-opts="$merge_base..$tip")
    ;;
  *) echo 'Usage: check-secrets.sh pre-commit | pre-push <base-ref> | worktree <base-ref>' >&2; exit 2 ;;
esac
# The explicit config ignores GITLEAKS_CONFIG and other environment overrides.
exec "$bin/gitleaks" git "$root" --config "$root/.gitleaks.toml" --redact --no-banner --exit-code 1 "${scan[@]}"
