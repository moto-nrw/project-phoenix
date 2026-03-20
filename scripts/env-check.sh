#!/usr/bin/env bash
# env-check.sh — Verify all environments have the same env var keys.
#
# Works WITHOUT decryption: SOPS keeps keys in plaintext, only values are encrypted.
# This means the check runs in CI without needing the SOPS_AGE_KEY secret.
#
# Usage:
#   ./scripts/env-check.sh                    # Compare all environments
#   ./scripts/env-check.sh staging production  # Compare specific environments
#
# Exit codes:
#   0 — All environments have matching keys
#   1 — Missing keys detected (blocks PR)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_DIR="${SCRIPT_DIR}/../environments"

# Default: check all environments. Override with arguments.
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

if [[ ${#ENVS[@]} -lt 2 ]]; then
  echo "Need at least 2 environments to compare. Found: ${ENVS[*]:-none}"
  exit 1
fi

# Use first environment as reference
REFERENCE="${ENVS[0]}"

# Extract keys from a .sops.env file (plaintext keys, skip SOPS metadata and comments)
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

ref_file="${ENV_DIR}/${REFERENCE}.sops.env"
ref_keys=$(get_keys "$ref_file")
ref_count=$(echo "$ref_keys" | wc -l | tr -d ' ')

echo "Reference: ${REFERENCE} (${ref_count} keys)"
echo "Comparing: ${ENVS[*]}"
echo "---"

exit_code=0

for env in "${ENVS[@]}"; do
  [[ "$env" == "$REFERENCE" ]] && continue

  env_file="${ENV_DIR}/${env}.sops.env"

  if [[ ! -f "$env_file" ]]; then
    echo "❌ ${env}: file not found (${env_file})"
    exit_code=1
    continue
  fi

  env_keys=$(get_keys "$env_file")

  missing=$(comm -23 <(echo "$ref_keys") <(echo "$env_keys") || true)
  extra=$(comm -13 <(echo "$ref_keys") <(echo "$env_keys") || true)

  if [[ -n "$missing" ]]; then
    echo "❌ ${env} is missing keys (present in ${REFERENCE}):"
    echo "$missing" | sed 's/^/   /'
    exit_code=1
  fi

  if [[ -n "$extra" ]]; then
    echo "⚠️  ${env} has extra keys (not in ${REFERENCE}):"
    echo "$extra" | sed 's/^/   /'
  fi

  if [[ -z "$missing" && -z "$extra" ]]; then
    echo "✅ ${env} — keys match"
  fi
done

echo "---"
if [[ $exit_code -eq 0 ]]; then
  echo "✅ All environments have matching keys"
else
  echo "❌ Key mismatch detected — fix before merging"
fi

exit $exit_code
