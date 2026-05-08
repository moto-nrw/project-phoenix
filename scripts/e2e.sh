#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.example.yml"
ENV_FILE="$ROOT_DIR/.env"

cleanup() {
  docker compose -f "$COMPOSE_FILE" --profile e2e down --volumes --remove-orphans >/dev/null 2>&1 || true
}

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing required file: $ENV_FILE" >&2
  echo "Create it from .env.example before running the canonical E2E command." >&2
  exit 1
fi

trap cleanup EXIT

docker compose -f "$COMPOSE_FILE" --profile e2e up -d --wait --force-recreate postgres-test server-e2e
docker compose -f "$COMPOSE_FILE" --profile e2e exec -T server-e2e go run main.go e2e prepare

cd "$ROOT_DIR/frontend"
pnpm exec playwright test "$@"
