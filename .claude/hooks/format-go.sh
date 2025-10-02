#!/bin/bash

# Read JSON input from stdin
input=$(cat)

# Extract file path
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only process Go files
if [[ ! "$file_path" =~ \.go$ ]]; then
  exit 0
fi

# Check if file exists
if [ ! -f "$file_path" ]; then
  exit 0
fi

# Format with gofmt
if command -v gofmt &> /dev/null; then
  gofmt -w "$file_path" 2>/dev/null
fi

# Organize imports with goimports (if available)
if command -v goimports &> /dev/null; then
  goimports -w "$file_path" 2>/dev/null
elif [ -f "/Users/yonnock/go/bin/goimports" ]; then
  /Users/yonnock/go/bin/goimports -w "$file_path" 2>/dev/null
fi

echo "✓ Formatted Go file: $file_path"
