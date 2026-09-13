#!/usr/bin/env bash
# Restore a complete release backup, never a standalone database dump.
# Caller starts the previous application only after this command succeeds.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec bash "$script_dir/release-backup.sh" restore "${1:?Pass the complete backup directory}"
