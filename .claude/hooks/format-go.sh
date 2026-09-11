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

if [[ "${file_path}" != /* ]]; then
  file_path="${project_root}/${file_path}"
fi

# Format with the repository-pinned Go toolchain. goimports also applies gofmt.
if ! (
  cd "${project_root}/backend"
  "${project_root}/scripts/run-go-toolchain.sh" go tool goimports -w "${file_path}"
); then
  echo "Failed to format Go file: ${file_path}" >&2
  exit 1
fi

echo "✓ Formatted Go file: ${file_path}"
exit 0
