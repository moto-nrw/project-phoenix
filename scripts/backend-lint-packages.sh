#!/usr/bin/env bash
# Prints golangci-lint package targets relative to backend/. Pull requests use
# affected packages; every other case fails safe to a full backend lint.
set -euo pipefail

: "${EVENT_NAME:?EVENT_NAME is required}"
: "${FULL_BACKEND:?FULL_BACKEND is required}"

packages=(./...)
if [ "$EVENT_NAME" = pull_request ] && [ "$FULL_BACKEND" != true ]; then
  : "${BASE_REF:?BASE_REF is required for partial pull-request lint}"
  repo_root=$(git rev-parse --show-toplevel)
  script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
  module=$(cd "$repo_root/backend" && go list -m)
  affected_output=$("$script_dir/backend-affected-packages.sh" "origin/$BASE_REF")

  if [ -n "$affected_output" ]; then
    packages=()
    while IFS= read -r package; do
      if [ "$package" = "$module" ]; then
        packages+=(.)
      elif [[ "$package" == "$module/"* ]]; then
        packages+=(".${package#"$module"}")
      else
        packages+=("$package")
      fi
    done <<< "$affected_output"
  fi
fi

printf '%s\n' "${packages[@]}"
