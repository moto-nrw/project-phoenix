#!/bin/bash
set -euo pipefail

# Find project root (where .git directory is)
project_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "${project_root}"

# Read JSON input from stdin
input=$(</dev/stdin)

# Extract file path
file_path=$(echo "${input}" | jq -r '.tool_input.file_path // empty')

# Only process Go files
if [[ ! "${file_path}" =~ \.go$ ]]; then
  exit 0
fi

# Check if file exists
if [[ ! -f "${file_path}" ]]; then
  exit 0
fi

# Format with gofmt
if command -v gofmt &> /dev/null; then
  gofmt -w "${file_path}" 2>/dev/null || true
fi

# Organize imports with goimports (if available)
if command -v goimports &> /dev/null; then
  goimports -w "${file_path}" 2>/dev/null || true
elif [[ -f "$HOME/go/bin/goimports" ]]; then
  "$HOME/go/bin/goimports" -w "${file_path}" 2>/dev/null || true
fi

echo "✓ Formatted Go file: ${file_path}"
exit 0
