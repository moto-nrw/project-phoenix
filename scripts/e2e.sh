#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/docker-compose.example.yml"

require_file() {
  local path="$1"
  local hint="$2"

  if [[ -f "$path" ]]; then
    return 0
  fi

  echo "error: missing required file: $path" >&2
  echo "       $hint" >&2
  exit 1
}

compose_e2e() {
  docker compose -f "$COMPOSE_FILE" --profile e2e "$@"
}

playwright_args=("$@")
if [[ ${#playwright_args[@]} -gt 0 && ${playwright_args[0]} == "--" ]]; then
  playwright_args=("${playwright_args[@]:1}")
fi

require_file "$REPO_ROOT/.env" "Create it from .env.example before running the canonical E2E harness."
require_file "$COMPOSE_FILE" "The canonical E2E harness runs directly against docker-compose.example.yml."

echo "Bringing up isolated E2E backend (postgres-test + server-e2e)..."
compose_e2e up -d --wait --force-recreate postgres-test server-e2e

echo "Preparing canonical Go-owned E2E world..."
compose_e2e exec -T server-e2e go run main.go e2e prepare
compose_e2e exec -T server-e2e chmod 644 .e2e-manifest.json

echo "Running Playwright..."

cd "$REPO_ROOT/frontend"
pnpm exec playwright test "${playwright_args[@]}"
