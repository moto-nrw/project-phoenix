#!/bin/bash
set -euo pipefail

# Read JSON input from stdin
input=$(</dev/stdin)

# Extract command - use empty string as default
command=$(echo "${input}" | jq -r '.tool_input.command // empty')

# Check if this is a git commit command, also after `cd … &&`
commit_re='(^|[^[:alnum:]_-])git[[:space:]]+commit([[:space:]]|$)'
if [[ ! "${command}" =~ ${commit_re} ]]; then
  exit 0
fi

# Check for Claude attribution (CRITICAL - MUST NOT APPEAR). The whole command
# is searched, so later -m paragraphs and heredoc messages count too. Exit 2
# blocks the tool call; any other non-zero status only prints a notice.
if echo "${command}" | grep -qiE "co-authored-by:.*claude"; then
  echo "❌ ERROR: Commit message contains Claude attribution!" >&2
  echo "  Remove 'Co-Authored-By: Claude' from commit message" >&2
  exit 2
fi

# Extract the first quoted message (-m or --message). Bash patterns only:
# the sed shipped with macOS has no \+ or \x27.
message=""
double_quoted_re='(-m|--message)[=[:space:]]+"([^"]*)"'
single_quoted_re="(-m|--message)[=[:space:]]+'([^']*)'"
if [[ "${command}" =~ ${double_quoted_re} ]]; then
  message=${BASH_REMATCH[2]}
elif [[ "${command}" =~ ${single_quoted_re} ]]; then
  message=${BASH_REMATCH[2]}
fi

if [[ -z "${message}" ]]; then
  exit 0  # No inline message found, skip the format check
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
