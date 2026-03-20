#!/usr/bin/env bash
# sops-setup.sh — One-time setup for SOPS + age encryption.
#
# What this script does:
#   1. Checks for sops and age binaries
#   2. Generates an age keypair (if not already present)
#   3. Updates .sops.yaml with the public key
#   4. Encrypts all *.sops.env files in-place
#   5. Prints instructions for GitHub Secret and local key storage
#
# Prerequisites:
#   brew install sops age   (macOS)
#   apt install age && curl -LO https://github.com/getsops/sops/releases/... (Linux)
#
# Usage:
#   ./scripts/sops-setup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_DIR="${PROJECT_ROOT}/environments"
SOPS_CONFIG="${PROJECT_ROOT}/.sops.yaml"

# Default age key location (SOPS looks here automatically)
case "$(uname -s)" in
  Darwin) AGE_KEY_DIR="${HOME}/Library/Application Support/sops/age" ;;
  *)      AGE_KEY_DIR="${HOME}/.config/sops/age" ;;
esac
AGE_KEY_FILE="${AGE_KEY_DIR}/keys.txt"

echo "=== SOPS + age Setup ==="
echo ""

# Step 1: Check dependencies
for cmd in sops age-keygen; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: '$cmd' not found."
    echo ""
    echo "Install with:"
    echo "  macOS:  brew install sops age"
    echo "  Linux:  sudo apt install age && see https://github.com/getsops/sops/releases"
    exit 1
  fi
done
echo "✅ sops and age found"

# Step 2: Generate age key (or use existing)
if [[ -f "$AGE_KEY_FILE" ]]; then
  echo "✅ Existing age key found at: ${AGE_KEY_FILE}"
  PUBLIC_KEY=$(grep -o 'age1[a-z0-9]*' "$AGE_KEY_FILE" | head -1)
else
  echo "Generating new age keypair..."
  mkdir -p "$AGE_KEY_DIR"
  age-keygen -o "$AGE_KEY_FILE" 2>&1
  chmod 600 "$AGE_KEY_FILE"
  PUBLIC_KEY=$(grep -o 'age1[a-z0-9]*' "$AGE_KEY_FILE" | head -1)
  echo "✅ Key generated at: ${AGE_KEY_FILE}"
fi

echo "   Public key: ${PUBLIC_KEY}"
echo ""

# Step 3: Update .sops.yaml with the public key
if grep -q 'REPLACE_WITH_AGE_PUBLIC_KEY' "$SOPS_CONFIG"; then
  # Use temp file to avoid GNU vs BSD sed -i incompatibility
  sed "s|REPLACE_WITH_AGE_PUBLIC_KEY|${PUBLIC_KEY}|" "$SOPS_CONFIG" > "${SOPS_CONFIG}.tmp" \
    && mv "${SOPS_CONFIG}.tmp" "$SOPS_CONFIG"
  echo "✅ Updated .sops.yaml with public key"
elif grep -q "$PUBLIC_KEY" "$SOPS_CONFIG"; then
  echo "✅ .sops.yaml already has this public key"
else
  echo "⚠️  .sops.yaml has a different key configured. Update manually if needed."
fi
echo ""

# Step 4: Encrypt all .sops.env files
echo "Encrypting environment files..."
encrypted=0
skipped=0

for env_file in "${ENV_DIR}"/*.sops.env; do
  [[ -f "$env_file" ]] || continue

  # Check if already encrypted (SOPS adds sops_ metadata keys)
  if grep -q '^sops_version=' "$env_file" 2>/dev/null; then
    echo "   ⏭️  $(basename "$env_file") — already encrypted"
    skipped=$((skipped + 1))
    continue
  fi

  sops encrypt --in-place "$env_file"
  echo "   🔒 $(basename "$env_file") — encrypted"
  encrypted=$((encrypted + 1))
done

echo ""
echo "✅ Encrypted: ${encrypted}, Skipped: ${skipped}"
echo ""

# Step 5: Print next steps
PRIVATE_KEY=$(grep 'AGE-SECRET-KEY' "$AGE_KEY_FILE")

echo "==========================================="
echo "  SETUP COMPLETE — Next Steps"
echo "==========================================="
echo ""
echo "1. Add the PRIVATE key as a GitHub Actions secret:"
echo "   Secret name: SOPS_AGE_KEY"
echo "   Secret value:"
echo ""
echo "   ${PRIVATE_KEY}"
echo ""
echo "   Go to: https://github.com/moto-nrw/project-phoenix/settings/secrets/actions"
echo ""
echo "2. Commit the encrypted files:"
echo "   git add environments/*.sops.env .sops.yaml"
echo "   git commit -m 'feat: add SOPS-encrypted environment configs'"
echo ""
echo "3. Share the key with team members:"
echo "   Each team member needs the private key at:"
echo "   ${AGE_KEY_FILE}"
echo ""
echo "   Share via a secure channel (1Password, Signal, etc.) — NEVER via Slack/email."
echo ""
echo "4. To edit an encrypted env file:"
echo "   sops edit environments/staging.sops.env"
echo ""
echo "5. To view decrypted values:"
echo "   sops decrypt environments/staging.sops.env"
echo "==========================================="
