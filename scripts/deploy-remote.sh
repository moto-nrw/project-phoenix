#!/usr/bin/env bash
# deploy-remote.sh — Runs ON the remote server during CI deployment.
# SCP'd by CI into ~/scripts/ alongside restore-db.sh.
#
# Required env vars:
#   DEPLOY_DIR        — Server directory (staging/demo/production)
#   DEPLOY_SHA        — Short commit SHA for deploy-state tracking
#   BACKUP_RETENTION  — Number of backups to keep (e.g. 3 or 7)
#
# Exit codes:
#   0  — Deploy succeeded
#   1  — Deploy aborted before migration (pull/backup failure), services restored
#   10 — Deploy failed, automatic rollback succeeded
#   11 — Deploy failed AND rollback failed (CRITICAL)

uploads_volume_name() {
  echo "phoenix-${DEPLOY_DIR}-uploads"
}

backup_uploads_volume() {
  local uploads_file="$1"
  local volume_name
  volume_name="$(uploads_volume_name)"

  mkdir -p "$(dirname "$uploads_file")"

  if ! docker run --rm \
    -v "${volume_name}:/volume" \
    -v "$(dirname "$uploads_file"):/backup" \
    --entrypoint sh \
    postgres:17-alpine \
    -c "tar -czf /backup/$(basename "$uploads_file") -C /volume ."; then
    return 1
  fi

  return 0
}

for var in DEPLOY_DIR DEPLOY_SHA BACKUP_RETENTION; do
  eval "val=\${$var:-}"
  if [ -z "$val" ]; then
    echo "FATAL: $var is not set"
    exit 1
  fi
done

cd ~/"$DEPLOY_DIR"
echo "Starting $DEPLOY_DIR deployment ($DEPLOY_SHA)..."

# ── Save rollback copies of current config ──
cp .env .env.rollback 2>/dev/null || true
cp docker-compose.yml docker-compose.yml.rollback 2>/dev/null || true

# ── Verify new config files were delivered by CI ──
if [ ! -f .env.new ]; then
  echo "FATAL: .env.new not found — SCP delivery failed or concurrent deploy consumed it"
  exit 1
fi
if [ ! -f docker-compose.yml.new ]; then
  echo "FATAL: docker-compose.yml.new not found — SCP delivery failed or concurrent deploy consumed it"
  exit 1
fi

# ── Swap in new config ──
mv .env.new .env
mv docker-compose.yml.new docker-compose.yml

# ── Pin images to exact SHA (immutable tags for rollback safety) ──
sed -i "s|phoenix-server:[^ ]*|phoenix-server:${DEPLOY_SHA}|" docker-compose.yml
sed -i "s|phoenix-frontend:[^ ]*|phoenix-frontend:${DEPLOY_SHA}|" docker-compose.yml

# ── Pull new images ──
if ! docker compose pull; then
  echo "Pull failed, restoring old config"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi

# ── Stop app containers (postgres stays for backup) ──
docker compose stop server frontend

# ── Backup database (DB frozen — no app writing) ──
mkdir -p ~/backups/"$DEPLOY_DIR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE=~/backups/$DEPLOY_DIR/backup-${TIMESTAMP}.dump
GLOBALS_FILE=~/backups/$DEPLOY_DIR/globals-${TIMESTAMP}.sql
UPLOADS_FILE=~/backups/$DEPLOY_DIR/uploads-${TIMESTAMP}.tar.gz
# Dump cluster globals (roles, passwords, tablespaces) — needed for full restore
if ! docker compose exec -T postgres pg_dumpall -U postgres --globals-only > "$GLOBALS_FILE"; then
  echo "pg_dumpall failed, aborting deploy"
  rm -f "$GLOBALS_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi
chmod 600 "$GLOBALS_FILE"
# Dump database data + schema in custom format
if ! docker compose exec -T postgres pg_dump -U postgres -d postgres -Fc > "$BACKUP_FILE"; then
  echo "pg_dump failed, aborting deploy"
  rm -f "$BACKUP_FILE" "$GLOBALS_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi
if [ ! -s "$BACKUP_FILE" ] || [ ! -s "$GLOBALS_FILE" ]; then
  echo "Backup files empty, aborting deploy"
  rm -f "$BACKUP_FILE" "$GLOBALS_FILE" "$UPLOADS_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi

if ! backup_uploads_volume "$UPLOADS_FILE"; then
  echo "Uploads backup failed, aborting deploy"
  rm -f "$BACKUP_FILE" "$GLOBALS_FILE" "$UPLOADS_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi

if [ ! -s "$UPLOADS_FILE" ]; then
  echo "Uploads backup empty, aborting deploy"
  rm -f "$BACKUP_FILE" "$GLOBALS_FILE" "$UPLOADS_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi

echo "Backup: $(basename "$BACKUP_FILE") ($(du -h "$BACKUP_FILE" | cut -f1))"
echo "Uploads backup: $(basename "$UPLOADS_FILE") ($(du -h "$UPLOADS_FILE" | cut -f1))"

# ── Read previous deploy state ──
PREVIOUS_SHA=""
if [ -f .deploy-state ]; then
  PREVIOUS_SHA=$(grep '^CURRENT_SHA=' .deploy-state | cut -d= -f2)
fi

# ── Run migration ──
DEPLOY_FAILED=false
if ! docker compose run --rm migrate; then
  echo "Migration FAILED"
  DEPLOY_FAILED=true
fi

# ── Start services (skip if migration failed) ──
if [ "$DEPLOY_FAILED" = "false" ]; then
  if ! docker compose up -d --wait --remove-orphans; then
    echo "Healthcheck FAILED"
    DEPLOY_FAILED=true
  fi
fi

# ── SUCCESS ──
if [ "$DEPLOY_FAILED" = "false" ]; then
  echo "CURRENT_SHA=${DEPLOY_SHA}" > .deploy-state
  echo "PREVIOUS_SHA=${PREVIOUS_SHA}" >> .deploy-state
  echo "DEPLOYED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> .deploy-state
  echo "BACKUP_FILE=${BACKUP_FILE}" >> .deploy-state
  echo "UPLOADS_FILE=${UPLOADS_FILE}" >> .deploy-state
  rm -f .env.rollback docker-compose.yml.rollback
  rm -f ~/deploy-remote.sh ~/restore-db.sh
  RETENTION_CUTOFF=$((BACKUP_RETENTION + 1))
  ls -t ~/backups/"$DEPLOY_DIR"/backup-*.dump 2>/dev/null | tail -n +"$RETENTION_CUTOFF" | xargs -r rm -v
  ls -t ~/backups/"$DEPLOY_DIR"/globals-*.sql 2>/dev/null | tail -n +"$RETENTION_CUTOFF" | xargs -r rm -v
  ls -t ~/backups/"$DEPLOY_DIR"/uploads-*.tar.gz 2>/dev/null | tail -n +"$RETENTION_CUTOFF" | xargs -r rm -v
  echo "$DEPLOY_DIR deployment complete"
  exit 0
fi

# ── ROLLBACK ──
echo "Deploy failed, initiating rollback..."
docker compose stop server frontend 2>/dev/null || true

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if ! "$SCRIPT_DIR/restore-db.sh" "$BACKUP_FILE"; then
  exit 11
fi

cp .env.rollback .env 2>/dev/null || true
cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true

if docker compose pull && docker compose up -d --wait --remove-orphans; then
  echo "Rollback successful — restored previous version"
  exit 10
else
  echo "CRITICAL: Rollback also failed!"
  exit 11
fi
