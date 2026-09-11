#!/usr/bin/env bash
set -euo pipefail
# Prefer Anthropic's native installer over stale Homebrew/npm installations.
if [[ -x "$HOME/.local/bin/claude" ]]; then
  claude_bin="$HOME/.local/bin/claude"
else
  claude_bin=$(command -v claude) || {
    echo 'Install Claude Code using the official native installer first.' >&2
    exit 1
  }
fi
exec "$claude_bin" "$@"
