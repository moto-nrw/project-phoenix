#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
bin="$root/.devbox/nix/profile/default/bin"
if [[ ! -x "$bin/ggshield" ]]; then
  echo "Pinned ggshield is missing. Run 'devbox install', then 'devbox run -- ggshield auth login'." >&2
  exit 1
fi
export PATH="$bin:$PATH"
# Environment overrides must not turn a blocking hook into a successful or
# secret-printing scan. The explicit config also excludes global CLI ignores.
export GITGUARDIAN_EXIT_ZERO=false
export GITGUARDIAN_SHOW_SECRETS=false
case "${1:-}" in
  pre-commit)
    [[ $# -eq 1 ]] || exit 2
    scan=(pre-commit)
    ;;
  pre-push)
    [[ $# -eq 2 ]] || { echo 'Usage: check-secrets.sh pre-push <base-ref>' >&2; exit 2; }
    merge_base=$(git merge-base "$2" HEAD)
    # ggshield rejects empty ranges; there are no branch commits to scan.
    if [[ "$merge_base" == "$(git rev-parse HEAD)" ]]; then
      exit 0
    fi
    scan=(commit-range "$merge_base..HEAD")
    ;;
  *) echo 'Usage: check-secrets.sh pre-commit | pre-push <base-ref>' >&2; exit 2 ;;
esac
exec "$bin/ggshield" --config-path "$root/.gitguardian.yaml" --no-check-for-updates secret scan "${scan[@]}"
