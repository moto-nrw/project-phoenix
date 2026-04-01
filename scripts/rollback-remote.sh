#!/usr/bin/env bash
# rollback-remote.sh — Runs ON the remote server during manual rollback.
# SCP'd by CI into ~/scripts/, same pattern as deploy-remote.sh to avoid heredoc stdin bugs.
#
# Required env vars:
#   DEPLOY_DIR   — Server directory (staging/demo/production)
#   TARGET_SHA   — Short commit SHA to roll back to
#   RESTORE_DB   — "true" to restore database from latest backup
#
# Exit codes:
#   0  — Rollback succeeded
#   1  — Rollback failed

for var in DEPLOY_DIR TARGET_SHA RESTORE_DB; do
  eval "val=\${$var:-}"
  if [ -z "$val" ]; then
    echo "FATAL: $var is not set"
    exit 1
  fi
done

cd ~/"$DEPLOY_DIR"
echo "Starting manual rollback to ${TARGET_SHA}..."

# ── Database restore (if requested) ──
if [ "$RESTORE_DB" = "true" ]; then
  BACKUP_FILE=$(ls -t ~/backups/"$DEPLOY_DIR"/backup-*.dump 2>/dev/null | head -1)
  if [ -z "$BACKUP_FILE" ]; then
    echo "No backup found in ~/backups/$DEPLOY_DIR/"
    exit 1
  fi
  docker compose stop server frontend

  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  if ! "$SCRIPT_DIR/restore-db.sh" "$BACKUP_FILE"; then
    exit 1
  fi
else
  docker compose stop server frontend
fi

# ── Override image tags to SHA-pinned versions ──
sed -i "s|phoenix-server:[^ ]*|phoenix-server:${TARGET_SHA}|" docker-compose.yml
sed -i "s|phoenix-frontend:[^ ]*|phoenix-frontend:${TARGET_SHA}|" docker-compose.yml

# ── Pull and start ──
docker compose pull
if ! docker compose up -d --wait; then
  echo "CRITICAL: Rolled-back version also unhealthy!"
  exit 1
fi

# ── Update deploy state ──
PREVIOUS_SHA=""
if [ -f .deploy-state ]; then
  PREVIOUS_SHA=$(grep '^CURRENT_SHA=' .deploy-state | cut -d= -f2)
fi
echo "CURRENT_SHA=${TARGET_SHA}" > .deploy-state
echo "PREVIOUS_SHA=${PREVIOUS_SHA}" >> .deploy-state
echo "DEPLOYED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> .deploy-state
echo "BACKUP_FILE=" >> .deploy-state

rm -f ~/rollback-remote.sh ~/restore-db.sh

echo "Rollback to ${TARGET_SHA} complete"
exit 0
