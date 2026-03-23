#!/usr/bin/env bash
# rollback-remote.sh — Runs ON the remote server during manual rollback.
# SCP'd by CI, same pattern as deploy-remote.sh to avoid heredoc stdin bugs.
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
  echo "Restoring DB from: $(basename "$BACKUP_FILE")"

  docker compose stop server frontend

  docker compose exec -T postgres psql -U postgres -d template1 -c "DROP DATABASE IF EXISTS postgres WITH (FORCE);" 2>/dev/null || true
  docker compose exec -T postgres psql -U postgres -d template1 -c "CREATE DATABASE postgres OWNER postgres TEMPLATE template0;"

  RESTORE_LOG=$(mktemp)
  if ! docker compose exec -T postgres pg_restore -U postgres -d postgres --exit-on-error < "$BACKUP_FILE" 2>"$RESTORE_LOG"; then
    echo "CRITICAL: pg_restore failed with errors:"
    cat "$RESTORE_LOG"
    rm -f "$RESTORE_LOG"
    exit 1
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
    exit 1
  fi
  echo "Database restored and verified"
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

echo "Rollback to ${TARGET_SHA} complete"
exit 0
