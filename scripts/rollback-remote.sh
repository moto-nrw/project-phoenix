#!/usr/bin/env bash
# Manual full-state rollback. Images and configuration come from the selected
# snapshot, not from an independently selected tag or the newest loose dump.
set -Eeuo pipefail
umask 077
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
case "${DEPLOY_DIR:-}" in staging|production|demo) ;; *) echo 'Invalid DEPLOY_DIR' >&2; exit 1;; esac
deployment_directory=${1:-"$HOME/$DEPLOY_DIR"}
[[ "$deployment_directory" = /* ]] || { echo 'Deployment directory must be absolute' >&2; exit 1; }
cd "$deployment_directory"
mkdir .release-operation.lock || { echo 'Another release operation owns this environment' >&2; exit 1; }
trap 'rmdir .release-operation.lock' EXIT
backup_id=${BACKUP_ID:-}
if [ -z "$backup_id" ]; then
  backup_id=$(awk -F= '$1 == "BACKUP_ID" {print $2}' .deploy-state)
fi
[[ "$backup_id" =~ ^release-[0-9]{8}T[0-9]{6}Z-[a-f0-9]{7,40}$ ]] || {
  echo 'Select a complete release backup ID; legacy loose dumps cannot be restored automatically' >&2
  exit 1
}
bundle="${deployment_directory%/*}/backups/$DEPLOY_DIR/$backup_id"
bash "$script_dir/restore-db.sh" "$bundle"
# The restored Compose file decides: a demo snapshot brings demo-runtime along.
listed=$(bash "$script_dir/release-backup.sh" app-services)
if ! docker compose up -d --wait --remove-orphans server frontend; then
  docker compose stop server frontend || true
  echo 'Restored application failed healthchecks and was stopped' >&2
  exit 1
fi
# Started without --wait: the sidecar's health never gates or stops the application.
[[ " $listed " != *' demo-runtime '* ]] || docker compose up -d --no-deps --force-recreate demo-runtime ||
  echo 'WARNING: demo-runtime did not start; the application itself is healthy' >&2
echo "Complete release restored from $backup_id"
