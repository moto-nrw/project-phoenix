#!/usr/bin/env bash
set +x
set -euo pipefail
command -v python3 >/dev/null || { echo 'backup_result=failed step=toolchain'; exit 1; }
exec python3 "$(dirname -- "$0")/daily_backup.py" backup "$@"
