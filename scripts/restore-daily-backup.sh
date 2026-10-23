#!/usr/bin/env bash
set +x
set -euo pipefail
command -v python3 >/dev/null || { echo 'restore_result=failed step=toolchain app_start=blocked'; exit 1; }
exec python3 "$(dirname -- "$0")/daily_backup.py" restore "$@"
