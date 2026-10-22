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
  *) echo 'Usage: check-secrets.sh pre-commit | pre-push <base-ref>' >&2; exit 2 ;;
esac
# The explicit config ignores GITLEAKS_CONFIG and other environment overrides.
exec "$bin/gitleaks" git "$root" --config "$root/.gitleaks.toml" --redact --no-banner --exit-code 1 "${scan[@]}"
