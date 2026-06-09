#!/usr/bin/env bash
# env-check.sh — Verify environment variable consistency across all config sources.
#
# Checks:
#   1. All SOPS environment files have the same keys
#   2. .env.example contains all keys from SOPS files (and vice versa)
#   3. docker-compose.example.yml services match environment compose files
#
# Works WITHOUT decryption: SOPS keeps keys in plaintext, only values are encrypted.
#
# Usage:
#   ./scripts/env-check.sh                    # Full check
#   ./scripts/env-check.sh staging production  # Compare specific environments only
#
# Exit codes:
#   0 — All checks pass
#   1 — Mismatch detected (blocks PR)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_DIR="${PROJECT_ROOT}/environments"
ENV_EXAMPLE="${PROJECT_ROOT}/.env.example"
COMPOSE_EXAMPLE="${PROJECT_ROOT}/docker-compose.example.yml"

exit_code=0

# Keys that only exist in local dev (.env.example), not in deployed environments
DEV_ONLY_KEYS="COMPOSE_BAKE COMPOSE_DOCKER_CLI_BUILD DB_DEBUG DOCKER_BUILDKIT FRONTEND_HOST_PORT MAILPIT_HOST_PORT MAILPIT_SMTP_HOST_PORT POSTGRES_HOST_PORT SERVER_HOST_PORT TEST_DB_DSN TEST_DB_PORT"

# Keys that only exist in deployed environments, not in local dev
DEPLOY_ONLY_KEYS=""

is_excluded() {
  local key="$1" list="$2"
  for k in $list; do
    [[ "$key" == "$k" ]] && return 0
  done
  return 1
}

# Extract keys from any env-style file (KEY=value)
get_keys() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "ERROR: File not found: $file" >&2
    return 1
  fi
  grep -E '^[A-Z_]+=' "$file" \
    | grep -v '^sops_' \
    | cut -d'=' -f1 \
    | sort
}

# Extract environment keys from a docker-compose file (under environment: blocks)
get_compose_env_keys() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "ERROR: File not found: $file" >&2
    return 1
  fi
  grep -E '^\s+[A-Z_]+:' "$file" \
    | grep -v '^\s*#' \
    | sed 's/:.*//' \
    | sed 's/^[[:space:]]*//' \
    | sort -u
}

# Extract service names from a docker-compose file
get_compose_services() {
  local file="$1"
  grep -E '^  [a-z]' "$file" \
    | grep -v '^\s*#' \
    | sed 's/:.*//' \
    | sed 's/^[[:space:]]*//' \
    | grep -v -E '^(driver|image|restart|depends_on|condition|command|ports|volumes|env_file|environment|logging|options|tag|healthcheck|test|interval|timeout|retries|start_period|networks|container_name)$' \
    | sort -u
}

# ============================================================
# CHECK 1: SOPS environment files have matching keys
# ============================================================
echo "=== Check 1: SOPS environment sync ==="

if [[ $# -gt 0 ]]; then
  ENVS=("$@")
else
  ENVS=()
  for f in "${ENV_DIR}"/*.sops.env; do
    [[ -f "$f" ]] || continue
    name=$(basename "$f" .sops.env)
    ENVS+=("$name")
  done
fi

if [[ ${#ENVS[@]} -ge 2 ]]; then
  REFERENCE="${ENVS[0]}"
  ref_file="${ENV_DIR}/${REFERENCE}.sops.env"
  ref_keys=$(get_keys "$ref_file")
  ref_count=$(echo "$ref_keys" | wc -l | tr -d ' ')

  echo "Reference: ${REFERENCE} (${ref_count} keys)"

  for env in "${ENVS[@]}"; do
    [[ "$env" == "$REFERENCE" ]] && continue
    env_file="${ENV_DIR}/${env}.sops.env"
    [[ -f "$env_file" ]] || { echo "❌ ${env}: file not found"; exit_code=1; continue; }

    env_keys=$(get_keys "$env_file")
    missing=$(comm -23 <(echo "$ref_keys") <(echo "$env_keys") || true)
    extra=$(comm -13 <(echo "$ref_keys") <(echo "$env_keys") || true)

    if [[ -n "$missing" ]]; then
      echo "❌ ${env} is missing keys (present in ${REFERENCE}):"
      echo "$missing" | sed 's/^/   /'
      exit_code=1
    fi
    if [[ -n "$extra" ]]; then
      echo "❌ ${env} has extra keys (not in ${REFERENCE}):"
      echo "$extra" | sed 's/^/   /'
      exit_code=1
    fi
    [[ -z "$missing" && -z "$extra" ]] && echo "✅ ${env} — keys match"
  done
else
  echo "⏭️  Skipped (need at least 2 environments)"
fi

# ============================================================
# CHECK 2: .env.example ↔ SOPS files
# ============================================================
echo ""
echo "=== Check 2: .env.example ↔ SOPS environments ==="

if [[ -f "$ENV_EXAMPLE" && ${#ENVS[@]} -ge 1 ]]; then
  example_keys=$(get_keys "$ENV_EXAMPLE")
  # Use first SOPS file as reference for deployed keys
  sops_ref="${ENV_DIR}/${ENVS[0]}.sops.env"
  sops_keys=$(get_keys "$sops_ref")

  # Keys in SOPS but not in .env.example (should be in .env.example for dev docs)
  missing_in_example=$(comm -13 <(echo "$example_keys") <(echo "$sops_keys") || true)
  if [[ -n "$missing_in_example" ]]; then
    has_real_missing=false
    while IFS= read -r key; do
      [[ -z "$key" ]] && continue
      if ! is_excluded "$key" "$DEPLOY_ONLY_KEYS"; then
        if [[ "$has_real_missing" == false ]]; then
          echo "❌ .env.example is missing keys (present in ${ENVS[0]}.sops.env):"
          has_real_missing=true
        fi
        echo "   $key"
        exit_code=1
      fi
    done <<< "$missing_in_example"
    [[ "$has_real_missing" == false ]] && echo "✅ .env.example has all deployed keys (deploy-only excluded)"
  else
    echo "✅ .env.example has all deployed keys"
  fi

  # Keys in .env.example but not in SOPS (should be in SOPS unless dev-only)
  missing_in_sops=$(comm -23 <(echo "$example_keys") <(echo "$sops_keys") || true)
  if [[ -n "$missing_in_sops" ]]; then
    has_real_missing=false
    while IFS= read -r key; do
      [[ -z "$key" ]] && continue
      if ! is_excluded "$key" "$DEV_ONLY_KEYS"; then
        if [[ "$has_real_missing" == false ]]; then
          echo "❌ ${ENVS[0]}.sops.env is missing keys (present in .env.example):"
          has_real_missing=true
        fi
        echo "   $key"
        exit_code=1
      fi
    done <<< "$missing_in_sops"
    [[ "$has_real_missing" == false ]] && echo "✅ SOPS environments have all .env.example keys (dev-only excluded)"
  else
    echo "✅ SOPS environments have all .env.example keys"
  fi
else
  echo "⏭️  Skipped (.env.example not found)"
fi

# ============================================================
# CHECK 3: docker-compose.example.yml ↔ environment compose files
# ============================================================
echo ""
echo "=== Check 3: Compose file service consistency ==="

if [[ -f "$COMPOSE_EXAMPLE" ]]; then
  # Check that core services exist in all environment compose files
  for env in "${ENVS[@]}"; do
    compose_file="${ENV_DIR}/${env}.compose.yml"
    [[ -f "$compose_file" ]] || { echo "❌ ${env}.compose.yml not found"; exit_code=1; continue; }

    # Check required services: server, frontend, postgres
    for svc in server frontend postgres; do
      if ! grep -q "^  ${svc}:" "$compose_file"; then
        echo "❌ ${env}.compose.yml is missing service: ${svc}"
        exit_code=1
      fi
    done

    # Check that compose uses env_file (not inline environment with ${VAR} interpolation)
    if grep -q '\${' "$compose_file"; then
      echo "❌ ${env}.compose.yml uses \${VAR} interpolation — should use env_file: .env"
      exit_code=1
    fi

    echo "✅ ${env}.compose.yml — services and env_file OK"
  done
else
  echo "⏭️  Skipped (docker-compose.example.yml not found)"
fi

# ============================================================
# CHECK 4: Fail-fast config guard
# ============================================================
echo ""
echo "=== Check 4: Fail-fast config guard ==="

ENV_JS="${PROJECT_ROOT}/frontend/src/env.js"
if [[ -f "$ENV_JS" ]]; then
  env_schema_violations=$(
    grep -nE '\.(default|optional)\(' "$ENV_JS" \
      | grep -v 'default("development")' \
      | grep -v 'default("info")' \
      | grep -v 'NEXT_PUBLIC_POSTHOG_KEY' \
      | grep -v 'NEXT_PUBLIC_POSTHOG_HOST' \
      | grep -v 'NEXT_PUBLIC_SENTRY_DSN' \
      | grep -v 'NEXT_PUBLIC_SENTRY_ENVIRONMENT' || true
  )
  if [[ -n "$env_schema_violations" ]]; then
    echo "❌ frontend/src/env.js contains unapproved .default()/.optional() usage:"
    echo "$env_schema_violations" | sed 's/^/   /'
    exit_code=1
  else
    echo "✅ frontend env schema has no unapproved defaults/options"
  fi
else
  echo "❌ frontend/src/env.js not found"
  exit_code=1
fi

frontend_env_fallbacks=$(grep -R -nE 'process\.env\.[A-Z0-9_]+[[:space:]]*(\?\?|\|\|)' "${PROJECT_ROOT}/frontend/src" || true)
if [[ -n "$frontend_env_fallbacks" ]]; then
  echo "❌ Frontend contains process.env fallback expressions:"
  echo "$frontend_env_fallbacks" | sed 's/^/   /'
  exit_code=1
else
  echo "✅ frontend process.env reads have no fallback expressions"
fi

backend_default_violations=$(
  grep -R -n 'viper.SetDefault' "${PROJECT_ROOT}/backend/cmd" \
    | grep -v 'log_level' || true
)
if [[ -n "$backend_default_violations" ]]; then
  echo "❌ backend/cmd contains unapproved viper.SetDefault calls:"
  echo "$backend_default_violations" | sed 's/^/   /'
  exit_code=1
else
  echo "✅ backend/cmd has no unapproved viper defaults"
fi

compose_fallbacks=$(
  grep -R -nE '\$\{[A-Z0-9_]+:-' \
    "$COMPOSE_EXAMPLE" \
    "${ENV_DIR}"/*.compose.yml \
    "${PROJECT_ROOT}/.github/workflows" 2>/dev/null || true
)
if [[ -n "$compose_fallbacks" ]]; then
  echo "❌ Compose/workflow files contain shell fallback interpolation:"
  echo "$compose_fallbacks" | sed 's/^/   /'
  exit_code=1
else
  echo "✅ Compose/workflow interpolation has no shell fallbacks"
fi

# ============================================================
# Summary
# ============================================================
echo ""
echo "---"
if [[ $exit_code -eq 0 ]]; then
  echo "✅ All checks passed"
else
  echo "❌ Issues detected — fix before merging"
fi

exit $exit_code
