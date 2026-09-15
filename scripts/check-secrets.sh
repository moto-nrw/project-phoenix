#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
bin="$root/.devbox/nix/profile/default/bin"
if [[ ! -x "$bin/git-secrets" ]]; then
  echo "Pinned git-secrets is missing. Run 'devbox install'." >&2
  exit 1
fi
export PATH="$bin:$PATH"
exec "$bin/git-secrets" "$@"
