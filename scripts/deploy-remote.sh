#!/usr/bin/env bash
# Remote maintenance deployment. 0=success, 1=pre-migration abort,
# 10=complete rollback succeeded, 11=rollback failed (application stays stopped).
set -Eeuo pipefail
(( $# <= 1 )) || { echo 'Usage: deploy-remote.sh [deployment-directory]; evidence files are no longer supported' >&2; exit 1; }
umask 077
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
case "${DEPLOY_DIR:-}" in staging|production|demo) ;; *) echo 'Invalid DEPLOY_DIR' >&2; exit 1;; esac
[[ "${DEPLOY_SHA:-}" =~ ^[a-f0-9]{7,40}$ ]] || { echo 'Invalid DEPLOY_SHA' >&2; exit 1; }
[[ "${BACKUP_RETENTION:-}" =~ ^[1-9][0-9]*$ ]] || { echo 'Invalid BACKUP_RETENTION' >&2; exit 1; }
deployment_directory=${1:-"$HOME/$DEPLOY_DIR"}
[[ "$deployment_directory" = /* ]] || { echo 'Deployment directory must be absolute' >&2; exit 1; }
cd "$deployment_directory"
mkdir .release-operation.lock || { echo 'Another release operation owns this environment' >&2; exit 1; }
cleanup_operation() {
  rmdir .release-operation.lock
}
trap cleanup_operation EXIT
[ -s .env.new ] && [ -s docker-compose.yml.new ] || { echo 'New configuration is missing' >&2; exit 1; }

# Pull before stopping the old application. Its configuration remains intact.
frontend_tag="$DEPLOY_SHA"
if [[ "$DEPLOY_DIR" = demo ]]; then frontend_tag="demo-$DEPLOY_SHA"; fi
sed "s|phoenix-server:[^ ]*|phoenix-server:${DEPLOY_SHA}|; s|phoenix-frontend:[^ ]*|phoenix-frontend:${frontend_tag}|" docker-compose.yml.new > docker-compose.yml.pinned
mv docker-compose.yml.pinned docker-compose.yml.new
docker compose --env-file .env.new -f docker-compose.yml.new pull

# Ask the new image whether the pending migrations' data preconditions hold,
# while the old release is still serving. A migration that refuses on data
# somebody has to correct would otherwise announce it after the application is
# stopped and the backup taken, making a full restore the only way out. It runs
# no migration; the only thing it writes is bun's own bookkeeping tables when
# they are absent, which the migrate below would create moments later anyway.
#
# --no-deps keeps compose from reconciling postgres against the new file while
# the old release is still connected to it; the surrounding script already
# assumes the database is up, the same way the backup below does.
if ! docker compose --env-file .env.new -f docker-compose.yml.new run --rm --no-deps migrate ./main migrate preflight; then
  echo 'Migration preflight failed; the running release was not touched' >&2
  exit 1
fi

# server and frontend, plus demo-runtime where the current Compose file has it.
# Reload after every configuration switch: the old and new file may differ.
load_app_services() {
  local listed
  listed=$(bash "$script_dir/release-backup.sh" app-services) || return 1
  read -r -a app_services <<< "$listed"
  # An empty list would make `compose stop` stop every service, postgres included.
  (( ${#app_services[@]} >= 2 ))
}

# Only server and frontend gate a release. The demo sidecar starts afterwards
# without --wait: its first start seeds a school for minutes, and its health
# must never roll back or stop a healthy application. Its healthcheck and
# journald log report a failing sidecar instead.
start_app_services() {
  docker compose up -d --wait "$@" server frontend || return 1
  [[ " ${app_services[*]} " != *' demo-runtime '* ]] || docker compose up -d --no-deps --force-recreate demo-runtime ||
    echo 'WARNING: demo-runtime did not start; the application itself is healthy' >&2
}

backup_root="${deployment_directory%/*}/backups/$DEPLOY_DIR"
mkdir -p "$backup_root"
load_app_services || { echo 'Compose service list failed; the running release was not touched' >&2; exit 1; }
if ! docker compose stop "${app_services[@]}"; then
  echo 'Application stop failed; no backup or migration attempted' >&2
  exit 1
fi

backup_id="release-$(date -u +%Y%m%dT%H%M%SZ)-${DEPLOY_SHA}"
bundle="$backup_root/$backup_id"
if ! bash "$script_dir/release-backup.sh" create "$bundle"; then
  echo 'Backup failed; restarting the unchanged version' >&2
  start_app_services || exit 11
  exit 1
fi

rollback() {
  trap - ERR
  echo "Deployment failed; restoring complete backup $backup_id" >&2
  load_app_services || exit 11
  docker compose stop "${app_services[@]}" || exit 11
  if ! bash "$script_dir/restore-db.sh" "$bundle"; then exit 11; fi
  load_app_services || exit 11
  if start_app_services --remove-orphans; then
    echo 'Previous release restored and healthy'
    exit 10
  fi
  docker compose stop "${app_services[@]}" || true
  echo 'Rollback healthcheck failed; application stopped' >&2
  exit 11
}
trap rollback ERR

previous_sha=$(awk -F= '$1 == "CURRENT_SHA" {print $2}' .deploy-state)
mv .env.new .env
mv docker-compose.yml.new docker-compose.yml
docker compose run --rm migrate
load_app_services
start_app_services --remove-orphans

{
  printf 'CURRENT_SHA=%s\nPREVIOUS_SHA=%s\n' "$DEPLOY_SHA" "$previous_sha"
  printf 'DEPLOYED_AT=%s\nBACKUP_ID=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$backup_id"
} > .deploy-state.new
mv .deploy-state.new .deploy-state
trap - ERR

# Retain whole completed sets; never delete individual components or this snapshot.
snapshots=()
for path in "$backup_root"/release-*; do
  [ -f "$path/complete" ] && snapshots+=("$path")
done
remove_count=$((${#snapshots[@]} - BACKUP_RETENTION))
for ((index=0; index<remove_count; index++)); do
  [ "${snapshots[$index]}" = "$bundle" ] || rm -rf -- "${snapshots[$index]}"
done
echo "Deployment complete; rollback backup: $backup_id"
