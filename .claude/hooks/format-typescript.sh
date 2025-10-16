#!/bin/bash
set -euo pipefail

# Read JSON input from stdin
input=$(</dev/stdin)

# Extract file path
file_path=$(echo "${input}" | jq -r '.tool_input.file_path // empty')

# Only process TypeScript/JavaScript files
if [[ ! "${file_path}" =~ \.(ts|tsx|js|jsx)$ ]]; then
  exit 0
fi

# Check if file exists
if [[ ! -f "${file_path}" ]]; then
  exit 0
fi

# Determine if we're in frontend directory
if [[ "${file_path}" == *"/frontend/"* ]]; then
  # Run prettier from frontend directory
  (cd frontend && npx prettier --write "../${file_path}" --cache 2>/dev/null) || true
  echo "✓ Formatted TypeScript file: ${file_path}"
else
  echo "⚠ Skipped: Not in frontend directory" >&2
fi

exit 0
