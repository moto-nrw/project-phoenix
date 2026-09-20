#!/usr/bin/env bash
# Complete release snapshots. Run from the environment's Compose directory.
# Secrets remain in owner-only files; never source snapshot metadata as shell.
set -euo pipefail
umask 077

fail() { echo "ERROR: $*" >&2; exit 1; }
compose() { docker compose "$@"; }
required_files=(database.dump roles.sql uploads.tar.gz .env compose.yml images.tsv deploy-state environment created-at uploads-volume)

digest() {
  if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi
}

environment() {
  case "${DEPLOY_DIR:-}" in staging|production|demo) ;; *) fail 'DEPLOY_DIR must be staging, production or demo';; esac
}

volume() {
  local container mounted
  container=$(compose ps -a -q server)
  [ -n "$container" ] || fail 'Serving container is missing; cannot identify the uploads volume'
  mounted=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/app/public/uploads"}}{{if eq .Type "volume"}}{{.Name}}{{end}}{{end}}{{end}}' "$container")
  [ -n "$mounted" ] || fail 'Server uploads mount is not a named volume'
  docker volume inspect "$mounted" >/dev/null
  printf '%s\n' "$mounted"
}

# Everything that writes to the database or uploads: server and frontend
# everywhere, plus the demo stack's demo-runtime. Read from the current Compose
# file, so staging, production and older demo snapshots never name the sidecar.
app_services() {
  local listed
  listed=$(compose config --services)
  if grep -qx demo-runtime <<< "$listed"; then echo 'server frontend demo-runtime'; else echo 'server frontend'; fi
}

ensure_stopped() {
  local service container services
  services=$(app_services)
  for service in $services; do
    container=$(compose ps -a -q "$service")
    # The sidecar has no container until its first deployment has started it.
    [ -n "$container" ] || [ "$service" != demo-runtime ] || continue
    [ -n "$container" ] || fail "Missing $service container"
    [ "$(docker inspect --format '{{.State.Running}}' "$container")" = false ] || fail "$service is still running"
  done
}

postgres_image() {
  awk -F '\t' '$1 == "postgres" {print $2}' "$bundle/images.tsv"
}

verify_files() {
  environment
  [ -d "$bundle" ] && [ ! -L "$bundle" ] || fail 'Backup directory is missing or a symlink'
  [ "$(cat "$bundle/environment")" = "$DEPLOY_DIR" ] || fail 'Backup belongs to another environment'
  local file expected actual line name
  [ -f "$bundle/checksums.sha256" ] && [ ! -L "$bundle/checksums.sha256" ] || fail 'Missing checksums'
  [ "$(wc -l < "$bundle/checksums.sha256" | tr -d ' ')" = "${#required_files[@]}" ] || fail 'Invalid checksum manifest'
  for file in "${required_files[@]}"; do
    [ -s "$bundle/$file" ] && [ ! -L "$bundle/$file" ] || fail "Missing backup component: $file"
    expected=$(awk -v name="$file" '$2 == name {print $1}' "$bundle/checksums.sha256")
    [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || fail "Invalid checksum entry: $file"
    actual=$(digest "$bundle/$file")
    [ "${actual%% *}" = "$expected" ] || fail "Backup checksum mismatch: $file"
  done
  while IFS=$'\t' read -r name line; do
    [[ "$name" =~ ^[a-zA-Z0-9_-]+$ && "$line" =~ ^[a-zA-Z0-9./:_-]+@sha256:[a-f0-9]{64}$ ]] || fail 'Invalid image manifest'
    docker image inspect "$line" >/dev/null 2>&1 || docker pull "$line" >/dev/null
  done < "$bundle/images.tsv"
  [ -n "$(postgres_image)" ] || fail 'Postgres image missing from snapshot'
  docker run --rm -i "$(postgres_image)" pg_restore --list < "$bundle/database.dump" >/dev/null
  docker run --rm --mount "type=bind,src=$bundle,dst=/backup,readonly" \
    --entrypoint tar "$(postgres_image)" -tzf /backup/uploads.tar.gz >/dev/null
  # Validate Compose without printing its resolved environment.
  docker compose --env-file "$bundle/.env" -f "$bundle/compose.yml" config -q
  local uploads
  uploads=$(cat "$bundle/uploads-volume")
  [[ "$uploads" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]+$ ]] || fail 'Invalid uploads volume name'
  docker volume inspect "$uploads" >/dev/null
}

verify() {
  [ "$(cat "$bundle/complete" 2>/dev/null)" = 'moto-release-backup-v1' ] || fail 'Backup is not complete'
  verify_files
}

dedicated_cluster() {
  # The deployed Compose service owns one application database. Refuse to
  # promise a complete restore for a shared cluster or external tablespaces.
  local extra
  extra=$(compose exec -T postgres psql -X -U postgres -d template1 -At -v ON_ERROR_STOP=1 -c \
    "SELECT (SELECT count(*) FROM pg_database WHERE datname NOT IN ('postgres','template0','template1')) + (SELECT count(*) FROM pg_tablespace WHERE spcname NOT IN ('pg_default','pg_global'));")
  [ "$extra" = 0 ] || fail 'Complete release backup requires a dedicated application PostgreSQL cluster'
}

create() {
  environment
  ensure_stopped
  dedicated_cluster
  [ ! -e "$bundle" ] || fail 'Backup path already exists'
  mkdir -m 700 -p "$bundle"
  local service container image_id image_ref uploads
  uploads=$(volume)
  printf '%s\n' "$uploads" > "$bundle/uploads-volume"
  # Snapshot the actual previous containers, not mutable registry tags.
  printf 'services:\n' > "$bundle/images.yml"
  : > "$bundle/images.tsv"
  while IFS= read -r service; do
    # migrate and demo-runtime run the serving backend image by contract.
    if [ "$service" = migrate ] || [ "$service" = demo-runtime ]; then
      container=$(compose ps -a -q server)
    else
      container=$(compose ps -a -q "$service")
    fi
    [ -n "$container" ] || fail "No previous container for $service"
    image_id=$(docker inspect --format '{{.Image}}' "$container")
    image_ref=$(docker image inspect --format '{{index .RepoDigests 0}}' "$image_id")
    [[ "$image_ref" =~ @sha256:[a-f0-9]{64}$ ]] || fail "No immutable registry digest for $service"
    printf '%s\t%s\n' "$service" "$image_ref" >> "$bundle/images.tsv"
    printf '  %s:\n    image: %s\n' "$service" "$image_ref" >> "$bundle/images.yml"
  done < <(compose --profile '*' config --services)
  cp .env "$bundle/.env"
  cp .deploy-state "$bundle/deploy-state"
  compose --profile '*' -f docker-compose.yml -f "$bundle/images.yml" config --no-interpolate > "$bundle/compose.yml"
  rm "$bundle/images.yml"
  printf '%s\n' "$DEPLOY_DIR" > "$bundle/environment"
  date -u +%Y-%m-%dT%H:%M:%SZ > "$bundle/created-at"
  compose exec -T postgres pg_dumpall -U postgres --globals-only > "$bundle/roles.sql"
  compose exec -T postgres pg_dump -U postgres -d postgres -Fc > "$bundle/database.dump"
  docker run --rm --mount "type=volume,src=$uploads,dst=/volume,readonly" \
    --mount "type=bind,src=$bundle,dst=/backup" --entrypoint tar "$(postgres_image)" \
    -czf /backup/uploads.tar.gz -C /volume .
  : > "$bundle/checksums.sha256"
  local file value
  for file in "${required_files[@]}"; do
    value=$(digest "$bundle/$file")
    printf '%s  %s\n' "${value%% *}" "$file" >> "$bundle/checksums.sha256"
  done
  # A failed or interrupted verification never leaves a selectable snapshot.
  if ! bash "$0" verify-files "$bundle"; then
    fail 'Backup verification failed; migration must not start'
  fi
  printf 'moto-release-backup-v1\n' > "$bundle/complete"
  echo "Complete backup verified: $(basename "$bundle")"
}

restore() {
  verify
  dedicated_cluster
  # All preflight checks happen before stopping or deleting anything.
  local uploads services
  services=$(app_services)
  # shellcheck disable=SC2086 # service names, split on purpose
  compose stop $services
  ensure_stopped
  cp "$bundle/.env" .env
  cp "$bundle/compose.yml" docker-compose.yml
  compose up -d --wait postgres
  compose exec -T postgres psql -X -U postgres -d template1 -v ON_ERROR_STOP=1 \
    -c 'DROP DATABASE IF EXISTS postgres WITH (FORCE);' >/dev/null
  # The dedicated cluster's non-system roles must match the snapshot too:
  # replaying GRANT alone would retain privileges introduced after the backup.
  compose exec -T postgres psql -X -U postgres -d template1 -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
SELECT format('DROP ROLE %I;', rolname)
FROM pg_roles WHERE rolname !~ '^pg_' AND rolname <> current_user
ORDER BY rolname
\gexec
SQL
  # Restore grants strictly. pg_dumpall emits CREATE ROLE even for existing
  # roles; only duplicate-object on that statement is an accepted condition.
  local roles_log
  roles_log=$(mktemp)
  trap 'rm -f "$roles_log"' EXIT
  if ! awk '
    /^CREATE ROLE / {
      if (index($0, "$moto_restore$")) exit 1;
      print "DO $moto_restore$ BEGIN " $0 " EXCEPTION WHEN duplicate_object THEN NULL; END $moto_restore$;";
      next
    }
    {print}
  ' "$bundle/roles.sql" | compose exec -T postgres psql -X -U postgres -d template1 -v ON_ERROR_STOP=1 >"$roles_log" 2>&1; then
    fail 'Role restore failed; private diagnostic output was not printed'
  fi
  compose exec -T postgres psql -X -U postgres -d template1 -v ON_ERROR_STOP=1 \
    -c 'CREATE DATABASE postgres OWNER postgres TEMPLATE template0;' >/dev/null
  if ! compose exec -T postgres pg_restore -U postgres -d postgres --exit-on-error < "$bundle/database.dump" >"$roles_log" 2>&1; then
    fail 'Database restore failed; application remains stopped'
  fi
  # ACL assertions cover the login connection and both SET ROLE paths.
  compose exec -T postgres psql -X -U postgres -d postgres -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
DO $$ BEGIN
  IF NOT pg_has_role('phoenix_auth', 'phoenix_tenant', 'MEMBER')
     OR NOT pg_has_role('phoenix_auth', 'phoenix_admin', 'MEMBER')
     OR NOT has_schema_privilege('phoenix_auth', 'auth', 'USAGE')
     OR NOT has_schema_privilege('phoenix_auth', 'platform', 'USAGE')
     OR NOT has_schema_privilege('phoenix_tenant', 'auth', 'USAGE')
     OR NOT has_schema_privilege('phoenix_tenant', 'active', 'USAGE')
     OR NOT has_schema_privilege('phoenix_admin', 'platform', 'USAGE')
     OR NOT has_table_privilege('phoenix_auth', 'auth.accounts', 'SELECT')
     OR NOT has_table_privilege('phoenix_auth', 'platform.operators', 'SELECT')
     OR NOT has_table_privilege('phoenix_tenant', 'auth.account_tenants', 'SELECT')
     OR NOT has_table_privilege('phoenix_tenant', 'active.visits', 'INSERT')
     OR NOT has_table_privilege('phoenix_admin', 'platform.operators', 'SELECT') THEN
    RAISE EXCEPTION 'Restored application privileges are incomplete';
  END IF;
END $$;
SQL
  uploads=$(cat "$bundle/uploads-volume")
  docker run --rm --mount "type=volume,src=$uploads,dst=/volume" \
    --mount "type=bind,src=$bundle,dst=/backup,readonly" --entrypoint sh "$(postgres_image)" \
    -c 'find /volume -mindepth 1 -maxdepth 1 -exec rm -rf -- {} + && tar -xzf /backup/uploads.tar.gz -C /volume'
  cp "$bundle/deploy-state" .deploy-state
  rm -f "$roles_log"
  trap - EXIT
  echo 'Database, roles, uploads and previous configuration restored; application is still stopped'
}

case "${1:-}" in
  create|verify|restore|verify-files)
    [ "$#" = 2 ] || fail 'Usage: release-backup.sh {create|verify|restore} /absolute/backup-directory'
    bundle=$2
    [[ "$bundle" = /* && "$bundle" != *','* && "$bundle" != *$'\n'* ]] || fail 'Backup path must be absolute and contain no comma or newline'
    if [ "$1" = verify-files ]; then verify_files; else "$1"; fi
    ;;
  app-services) app_services;;
  *) fail 'Expected create, verify or restore';;
esac
