#!/usr/bin/env bash
# Keep editor processes independent of GUI shell startup and global tool versions.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tool_bin="$repo_root/.devbox/nix/profile/default/bin"
export PATH="$tool_bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=0
unset DEVELOPER_DIR
tool=${1:?Expected gopls, typescript, typescript-native, oxlint, or prettier}
shift
cd "$repo_root"
case "$tool" in
  gopls)
    # Soft heap limit: gopls settles at about 1.0-1.1 GB instead of 0.8-1.7 GB
    # with the same navigation results (docs/development-environment.md).
    export GOMEMLIMIT=${GOMEMLIMIT:-1GiB}
    exec "$repo_root/scripts/run-go-toolchain.sh" gopls "$@"
    ;;
  typescript)
    test -f frontend/node_modules/typescript/lib/tsserver.js || {
      echo 'Run devbox run bootstrap to install the workspace TypeScript SDK.' >&2
      exit 1
    }
    exec "$tool_bin/typescript-language-server" "$@"
    ;;
  typescript-native)
    # Opt-in TypeScript 7 language server: about half the memory of tsserver,
    # but without Next's TypeScript plugin (docs/development-environment.md).
    exec "$tool_bin/node" frontend/node_modules/typescript-native/bin/tsc --lsp --stdio "$@"
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
