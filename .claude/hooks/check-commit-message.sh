#!/bin/bash
set -euo pipefail

# Read JSON input from stdin
input=$(</dev/stdin)

# Extract command - use empty string as default
command=$(echo "${input}" | jq -r '.tool_input.command // empty')

# Check if this is a git commit command
if [[ ! "${command}" =~ ^git[[:space:]]+commit ]]; then
  exit 0
fi

# Extract commit message (handle both -m and --message)
message=$(echo "${command}" | sed -n 's/.*-m[= ]\+["\x27]\([^"\x27]*\)["\x27].*/\1/p')

if [[ -z "${message}" ]]; then
  # Try alternative format
  message=$(echo "${command}" | sed -n 's/.*--message[= ]\+["\x27]\([^"\x27]*\)["\x27].*/\1/p')
fi

if [[ -z "${message}" ]]; then
  exit 0  # No message found, skip validation
fi

# Check for Claude attribution (CRITICAL - MUST NOT APPEAR)
if echo "${message}" | grep -qi "co-authored-by.*claude"; then
  echo "❌ ERROR: Commit message contains Claude attribution!" >&2
  echo "  Message: ${message}" >&2
  echo "  Remove 'Co-Authored-By: Claude' from commit message" >&2
  exit 1
fi

# Check conventional commits format
if ! [[ "${message}" =~ ^(feat|fix|docs|style|refactor|test|chore)(\([a-z0-9-]+\))?:[[:space:]].+ ]]; then
  echo "⚠ Warning: Commit message doesn't follow conventional commits" >&2
  echo "  Expected: type(scope): description" >&2
  echo "  Types: feat, fix, docs, style, refactor, test, chore" >&2
  echo "  Message: ${message}" >&2
  # Don't fail, just warn
fi

exit 0
