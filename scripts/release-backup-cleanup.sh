#!/usr/bin/env bash
# Independent retention without Docker, database access or backup credentials.
set -euo pipefail
umask 077
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
[ "$#" = 1 ] && [[ "$1" = /* ]] || { echo 'Usage: release-backup-cleanup.sh /absolute/deployment-directory' >&2; exit 1; }
case "${DEPLOY_DIR:-}" in staging|production|demo) ;; *) echo 'Invalid DEPLOY_DIR' >&2; exit 1;; esac
deployment_directory=${1%/}
cd "$deployment_directory"
mkdir .release-operation.lock || { echo 'Another release operation owns this environment; retry cleanup later' >&2; exit 1; }
trap 'rmdir .release-operation.lock' EXIT
bash "$script_dir/release-backup.sh" prune "${deployment_directory%/*}/backups/$DEPLOY_DIR" \
  "${BACKUP_RETENTION:?BACKUP_RETENTION is required}" \
  "${RELEASE_SNAPSHOT_MAX_AGE_DAYS:?RELEASE_SNAPSHOT_MAX_AGE_DAYS is required}"
echo 'Release snapshot retention complete'
