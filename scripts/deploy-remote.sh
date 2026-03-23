#!/usr/bin/env bash
# deploy-remote.sh — Runs ON the remote server during CI deployment.
# SCP'd by CI alongside .env.new and docker-compose.yml.new.
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
docker compose exec -T postgres pg_dump -U postgres -d postgres -Fc > "$BACKUP_FILE"
if [ ! -s "$BACKUP_FILE" ]; then
  echo "Backup failed or empty, aborting deploy"
  rm -f "$BACKUP_FILE"
  cp .env.rollback .env 2>/dev/null || true
  cp docker-compose.yml.rollback docker-compose.yml 2>/dev/null || true
  docker compose pull 2>/dev/null || true
  docker compose up -d --wait --remove-orphans 2>/dev/null || true
  exit 1
fi
echo "Backup: $(basename "$BACKUP_FILE") ($(du -h "$BACKUP_FILE" | cut -f1))"

# ── Read previous deploy state ──
PREVIOUS_SHA=""
if [ -f .deploy-state ]; then
  PREVIOUS_SHA=$(grep '^CURRENT_SHA=' .deploy-state | cut -d= -f2)
fi

# ── Run migration ──
DEPLOY_FAILED=false
if ! docker compose run --rm server ./main migrate; then
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
  rm -f .env.rollback docker-compose.yml.rollback
  RETENTION_CUTOFF=$((BACKUP_RETENTION + 1))
  ls -t ~/backups/"$DEPLOY_DIR"/backup-*.dump 2>/dev/null | tail -n +"$RETENTION_CUTOFF" | xargs -r rm -v
  echo "$DEPLOY_DIR deployment complete"
  exit 0
fi

# ── ROLLBACK ──
echo "Deploy failed, initiating rollback..."
docker compose stop server frontend 2>/dev/null || true

echo "Restoring database from backup..."
docker compose exec -T postgres psql -U postgres -d template1 -c "DROP DATABASE IF EXISTS postgres WITH (FORCE);" 2>/dev/null || true
docker compose exec -T postgres psql -U postgres -d template1 -c "CREATE DATABASE postgres OWNER postgres TEMPLATE template0;"

RESTORE_LOG=$(mktemp)
if ! docker compose exec -T postgres pg_restore -U postgres -d postgres --exit-on-error < "$BACKUP_FILE" 2>"$RESTORE_LOG"; then
  echo "CRITICAL: pg_restore failed with errors:"
  cat "$RESTORE_LOG"
  rm -f "$RESTORE_LOG"
  exit 11
fi
rm -f "$RESTORE_LOG"

# Verify restored DB has the role grants the app needs to function.
# SELECT 1 only proves connectivity; this checks actual privileges.
if ! docker compose exec -T postgres psql -U postgres -d postgres -c "
  DO \$\$ BEGIN
    IF NOT pg_has_role('phoenix_auth', 'phoenix_tenant', 'MEMBER') THEN
      RAISE EXCEPTION 'phoenix_auth is not a member of phoenix_tenant';
    END IF;
    IF NOT pg_has_role('phoenix_auth', 'phoenix_admin', 'MEMBER') THEN
      RAISE EXCEPTION 'phoenix_auth is not a member of phoenix_admin';
    END IF;
    IF NOT has_schema_privilege('phoenix_auth', 'auth', 'USAGE') THEN
      RAISE EXCEPTION 'phoenix_auth lacks USAGE on schema auth';
    END IF;
    IF NOT has_schema_privilege('phoenix_auth', 'platform', 'USAGE') THEN
      RAISE EXCEPTION 'phoenix_auth lacks USAGE on schema platform';
    END IF;
    IF NOT has_table_privilege('phoenix_auth', 'auth.accounts', 'SELECT') THEN
      RAISE EXCEPTION 'phoenix_auth lacks SELECT on auth.accounts';
    END IF;
    IF NOT has_table_privilege('phoenix_auth', 'platform.operators', 'SELECT') THEN
      RAISE EXCEPTION 'phoenix_auth lacks SELECT on platform.operators';
    END IF;
  END \$\$;
" > /dev/null 2>&1; then
  echo "CRITICAL: Database restored but role privileges are missing — restore may have dropped ACLs"
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
