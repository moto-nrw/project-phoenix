#!/usr/bin/env bash
# Credential-regression probe. Owns one disposable Postgres on a random port;
# never loads a developer's DSN or connects to the shared server on 5433.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
probe_dir=$(mktemp -d)
container_id=
cleanup() {
  if [[ -n "$container_id" ]]; then
    docker stop "$container_id" >/dev/null
  fi
  rm -rf "$probe_dir"
}
trap cleanup EXIT

export POSTGRES_PASSWORD
POSTGRES_PASSWORD=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
container_id=$(docker run --rm -d --tmpfs /var/lib/postgresql/data \
  -p 127.0.0.1::5432 -e POSTGRES_PASSWORD postgres:17-alpine)
port=$(docker port "$container_id" 5432 | sed 's/.*://')
if [[ ! "$port" =~ ^[0-9]+$ || "$port" == 5433 ]]; then
  echo 'Refusing unsafe regression-probe port' >&2
  exit 1
fi
ready=0
for ((attempt = 0; attempt < 60; attempt++)); do
  if docker exec "$container_id" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.5
done
if [[ "$ready" != 1 ]]; then
  echo 'Isolated regression server did not become ready' >&2
  exit 1
fi

# Authenticate over TCP, not the container's trusted Unix socket. A fresh
# connection after the rejected caller must still accept the original secret.
query() {
  PGPASSWORD="$POSTGRES_PASSWORD" docker exec -e PGPASSWORD "$container_id" \
    psql --no-psqlrc -h 127.0.0.1 -U postgres -d postgres -Atqc "$1"
}
# Detect any role rewrite, including an ALTER ROLE that preserves its value.
role_state=$(query 'SELECT md5(string_agg(oid::text || xmin::text, chr(10) ORDER BY oid)) FROM pg_authid')
wrong_password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
if (
  cd "$repo_root/backend"
  TEST_DB_DSN="postgres://postgres:${wrong_password}@127.0.0.1:${port}/phoenix_test?sslmode=disable" \
    ../scripts/run-go-toolchain.sh go run ./internal/testdb/cmd/bootstrap
) >"$probe_dir/bootstrap.log" 2>&1; then
  echo 'Wrong DSN unexpectedly accepted' >&2
  exit 1
fi
if ! grep -q 'test database authentication failed.*root .env.*docs/agents/operations.md' "$probe_dir/bootstrap.log"; then
  echo 'Wrong DSN did not produce the required credential-source diagnostic' >&2
  exit 1
fi
if grep -qE "$POSTGRES_PASSWORD|$wrong_password|postgres://" "$probe_dir/bootstrap.log"; then
  echo 'Credential probe exposed connection details' >&2
  exit 1
fi
if [[ "$role_state" != "$(query 'SELECT md5(string_agg(oid::text || xmin::text, chr(10) ORDER BY oid)) FROM pg_authid')" ]]; then
  echo 'Wrong DSN changed cluster roles' >&2
  exit 1
fi
echo 'PASS: wrong DSN rejected; original TCP credentials and cluster roles unchanged'
