#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/.." && pwd)
tool_bin="$repo_root/.devbox/nix/profile/default/bin"

if [[ ! -x "$tool_bin/go" ]]; then
  echo "Pinned Go toolchain is not installed." >&2
  echo "Run 'devbox install' in $repo_root." >&2
  exit 1
fi

expected_version=$(awk '$1 == "go" { print $2; exit }' "$repo_root/backend/go.mod")
actual_version=$("$tool_bin/go" env GOVERSION)
actual_version=${actual_version#go}
if [[ -z "$expected_version" || "$actual_version" != "$expected_version" ]]; then
  echo "Devbox Go version mismatch: found $actual_version, want $expected_version from backend/go.mod." >&2
  echo "Run 'devbox install' to refresh the pinned toolchain." >&2
  exit 1
fi

export PATH="$tool_bin:$PATH"
export GOTOOLCHAIN=local
unset GOBIN

if (($# == 0)); then
  echo "No command supplied to pinned Go toolchain runner." >&2
  exit 1
fi

requested_command=$1
shift
if [[ "$requested_command" == */* ]]; then
  exec "$requested_command" "$@"
fi

command_path="$tool_bin/$requested_command"
if [[ ! -x "$command_path" ]]; then
  echo "Pinned command is not installed: $requested_command" >&2
  echo "Run 'devbox install' in $repo_root." >&2
  exit 1
fi
exec "$command_path" "$@"
