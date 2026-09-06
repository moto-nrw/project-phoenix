#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root/frontend"
pnpm install --frozen-lockfile
# Type generation reads next.config but does not start services or execute routes.
# This existing build-only switch permits a fresh worktree without runtime secrets.
SKIP_ENV_VALIDATION=true pnpm exec next typegen
agent-browser install
