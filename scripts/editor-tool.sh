#!/usr/bin/env bash
# Keep editor processes independent of GUI shell startup and global tool versions.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tool_bin="$repo_root/.devbox/nix/profile/default/bin"
export PATH="$tool_bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=0
unset DEVELOPER_DIR
tool=${1:?Expected gopls, typescript, oxlint, or prettier}
shift
cd "$repo_root"
case "$tool" in
  gopls) exec "$repo_root/scripts/run-go-toolchain.sh" gopls "$@" ;;
  typescript)
    test -f frontend/node_modules/typescript/lib/tsserver.js || {
      echo 'Run devbox run bootstrap to install the workspace TypeScript SDK.' >&2
      exit 1
    }
    exec "$tool_bin/typescript-language-server" "$@"
    ;;
  oxlint)
    cd frontend
    exec "$tool_bin/node" node_modules/oxlint/bin/oxlint "$@"
    ;;
  prettier)
    cd frontend
    exec "$tool_bin/node" node_modules/prettier/bin/prettier.cjs "$@"
    ;;
  *) echo "Unknown editor tool: $tool" >&2; exit 64 ;;
esac
