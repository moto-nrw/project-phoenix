#!/usr/bin/env bash
# restore-db.sh — Restore a PostgreSQL database from a pg_dump backup.
# Shared by deploy-remote.sh (rollback path) and rollback-remote.sh.
#
# Usage: ./restore-db.sh <backup_file>
#
# Expects a running postgres container reachable via `docker compose exec`.
# Drops and recreates the database from template0, restores with
# --exit-on-error, then verifies role grants for all three app roles.
#
# Exit codes:
#   0  — Restore and verification succeeded
#   1  — Restore or verification failed

uploads_volume_name() {
  if [ -n "${DEPLOY_DIR:-}" ]; then
    echo "phoenix-${DEPLOY_DIR}-uploads"
    return 0
  fi

  local cwd_base
  cwd_base="$(basename "$(pwd)")"
  echo "phoenix-${cwd_base}-uploads"
}

restore_uploads_volume() {
  local uploads_file="$1"
  local volume_name
  volume_name="$(uploads_volume_name)"

  echo "Restoring uploads volume from: $(basename "$uploads_file")"

  if ! docker run --rm \
    -v "${volume_name}:/volume" \
    -v "$(dirname "$uploads_file"):/backup" \
    --entrypoint sh \
    postgres:17-alpine \
    -c "rm -rf /volume/* /volume/.[!.]* /volume/..?* && tar -xzf /backup/$(basename "$uploads_file") -C /volume"; then
    echo "CRITICAL: Uploads restore failed"
    return 1
  fi

  return 0
}

BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
  echo "FATAL: Usage: restore-db.sh <backup_file>"
  exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "FATAL: Backup file not found: $BACKUP_FILE"
  exit 1
fi

echo "Restoring database from: $(basename "$BACKUP_FILE")"

# ── Restore cluster globals (roles, passwords) if available ──
# Naming conventions:
#   deploy-remote.sh: backup-YYYYMMDD-HHMMSS.dump → globals-YYYYMMDD-HHMMSS.sql
#   backup.sh:        phoenix_env_YYYYMMDD_HHMMSS.dump → phoenix_env_YYYYMMDD_HHMMSS_globals.sql
BACKUP_DIR=$(dirname "$BACKUP_FILE")
BACKUP_BASE=$(basename "$BACKUP_FILE" .dump)
GLOBALS_FILE=""
UPLOADS_FILE=""

# Try deploy-remote.sh naming: backup-* → globals-*
if [[ "$BACKUP_BASE" == backup-* ]]; then
  TIMESTAMP_PART="${BACKUP_BASE#backup-}"
  GLOBALS_FILE="${BACKUP_DIR}/globals-${TIMESTAMP_PART}.sql"
  UPLOADS_FILE="${BACKUP_DIR}/uploads-${TIMESTAMP_PART}.tar.gz"
fi

# Try backup.sh naming: phoenix_env_timestamp.dump → phoenix_env_timestamp_globals.sql
if [ -z "$GLOBALS_FILE" ] || [ ! -f "$GLOBALS_FILE" ]; then
  CANDIDATE="${BACKUP_DIR}/${BACKUP_BASE}_globals.sql"
  if [ -f "$CANDIDATE" ]; then
    GLOBALS_FILE="$CANDIDATE"
  fi
fi

if [ -n "$UPLOADS_FILE" ] && [ ! -f "$UPLOADS_FILE" ]; then
  UPLOADS_FILE=""
fi

if [ -n "$GLOBALS_FILE" ] && [ -f "$GLOBALS_FILE" ]; then
  echo "Restoring cluster globals from: $(basename "$GLOBALS_FILE")"
  docker compose exec -T postgres psql -U postgres -d template1 < "$GLOBALS_FILE" 2>/dev/null || true
  echo "Cluster globals restored (roles, passwords)"
else
  echo "WARNING: No globals file found — roles must already exist on this cluster"
fi

# ── Drop and recreate database from clean template ──
docker compose exec -T postgres psql -U postgres -d template1 -c "DROP DATABASE IF EXISTS postgres WITH (FORCE);" 2>/dev/null || true
docker compose exec -T postgres psql -U postgres -d template1 -c "CREATE DATABASE postgres OWNER postgres TEMPLATE template0;"

# ── Restore with strict error handling ──
RESTORE_LOG=$(mktemp)
if ! docker compose exec -T postgres pg_restore -U postgres -d postgres --exit-on-error < "$BACKUP_FILE" 2>"$RESTORE_LOG"; then
  echo "CRITICAL: pg_restore failed with errors:"
  cat "$RESTORE_LOG"
  rm -f "$RESTORE_LOG"
  exit 1
fi
rm -f "$RESTORE_LOG"

# ── Verify restored DB has the role grants the app needs to function ──
# Checks all three roles the request path uses:
#   phoenix_auth   — connection role, pre-auth queries (login, tenant resolve, operator refresh)
#   phoenix_tenant — SET LOCAL ROLE per tenant request (tenant/tx.go:33)
#   phoenix_admin  — SET LOCAL ROLE for cross-tenant ops (tenant/tx.go:63)
if ! docker compose exec -T postgres psql -U postgres -d postgres -c "
  DO \$\$ BEGIN
    -- Role membership: phoenix_auth can SET ROLE to both
    IF NOT pg_has_role('phoenix_auth', 'phoenix_tenant', 'MEMBER') THEN
      RAISE EXCEPTION 'phoenix_auth is not a member of phoenix_tenant';
    END IF;
    IF NOT pg_has_role('phoenix_auth', 'phoenix_admin', 'MEMBER') THEN
      RAISE EXCEPTION 'phoenix_auth is not a member of phoenix_admin';
    END IF;

    -- phoenix_auth: pre-auth paths (login, token refresh, operator auth)
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

    -- phoenix_tenant: tenant-scoped request path (all CRUD after SET LOCAL ROLE)
    IF NOT has_schema_privilege('phoenix_tenant', 'auth', 'USAGE') THEN
      RAISE EXCEPTION 'phoenix_tenant lacks USAGE on schema auth';
    END IF;
    IF NOT has_schema_privilege('phoenix_tenant', 'active', 'USAGE') THEN
      RAISE EXCEPTION 'phoenix_tenant lacks USAGE on schema active';
    END IF;
    IF NOT has_table_privilege('phoenix_tenant', 'auth.account_tenants', 'SELECT') THEN
      RAISE EXCEPTION 'phoenix_tenant lacks SELECT on auth.account_tenants';
    END IF;
    IF NOT has_table_privilege('phoenix_tenant', 'active.visits', 'INSERT') THEN
      RAISE EXCEPTION 'phoenix_tenant lacks INSERT on active.visits';
    END IF;

    -- phoenix_admin: cross-tenant ops (migrations, operator routes, exports)
    IF NOT has_schema_privilege('phoenix_admin', 'platform', 'USAGE') THEN
      RAISE EXCEPTION 'phoenix_admin lacks USAGE on schema platform';
    END IF;
    IF NOT has_table_privilege('phoenix_admin', 'platform.operators', 'SELECT') THEN
      RAISE EXCEPTION 'phoenix_admin lacks SELECT on platform.operators';
    END IF;
  END \$\$;
" > /dev/null 2>&1; then
  echo "CRITICAL: Database restored but role privileges are missing — restore may have dropped ACLs"
  exit 1
fi

if [ -n "$UPLOADS_FILE" ] && [ -f "$UPLOADS_FILE" ]; then
  if ! restore_uploads_volume "$UPLOADS_FILE"; then
    exit 1
  fi
else
  echo "WARNING: No uploads backup found — uploads volume left unchanged"
fi

echo "Database restored and verified"
exit 0
