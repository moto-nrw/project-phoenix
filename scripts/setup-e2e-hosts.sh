#!/usr/bin/env bash
# ==============================================================================
# Project Phoenix — Add E2E test domains to /etc/hosts
#
# We use *.localtest.me as the dev tenant domain (instead of *.localhost) so
# NextAuth can set domain-scoped cookies and the tenant switch flow works
# locally just like in production. localtest.me is a public DNS wildcard,
# but some DNS resolvers (corporate, ISP, privacy filters) refuse to return
# it. This script makes resolution explicit by adding entries to /etc/hosts.
#
# Idempotent: running it twice does nothing the second time.
# Requires sudo because /etc/hosts is root-owned.
#
# Usage:  sudo ./scripts/setup-e2e-hosts.sh
# ==============================================================================

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "error: this script must run as root (use: sudo $0)" >&2
  exit 1
fi

MARKER="# project-phoenix E2E (managed by scripts/setup-e2e-hosts.sh)"
ENTRIES=(
  "127.0.0.1 demo-school.localtest.me"
  "127.0.0.1 second-school.localtest.me"
  "127.0.0.1 operator.localtest.me"
  "127.0.0.1 localtest.me"
)

if grep -qF "$MARKER" /etc/hosts; then
  echo "/etc/hosts already contains the project-phoenix block — nothing to do."
  exit 0
fi

{
  echo ""
  echo "$MARKER"
  for entry in "${ENTRIES[@]}"; do echo "$entry"; done
} >> /etc/hosts

echo "Added the following entries to /etc/hosts:"
printf '  %s\n' "${ENTRIES[@]}"
echo
echo "To remove later, delete the block marked with: $MARKER"
