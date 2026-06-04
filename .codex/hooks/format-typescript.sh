#!/bin/bash
set -euo pipefail

# Find project root (where .git directory is)
project_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "${project_root}"

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
# Match both absolute ("/frontend/") and relative ("frontend/") paths
if [[ "${file_path}" == *"/frontend/"* ]] || [[ "${file_path}" == frontend/* ]]; then
  # Run prettier from frontend directory
  # Extract the relative path from project root
  relative_path="${file_path#${project_root}/}"
  (cd frontend && npx prettier --write "../${relative_path}" --cache 2>/dev/null) || true
  echo "✓ Formatted TypeScript file: ${file_path}"
else
  echo "⚠ Skipped: Not in frontend directory" >&2
fi

exit 0
