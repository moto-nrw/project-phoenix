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

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

BEGIN_MARKER="# project-phoenix E2E BEGIN (managed by scripts/setup-e2e-hosts.sh)"
END_MARKER="# project-phoenix E2E END (managed by scripts/setup-e2e-hosts.sh)"
LEGACY_MARKER="# project-phoenix E2E (managed by scripts/setup-e2e-hosts.sh)"

mapfile -t HOSTS < <(
  cd "$BACKEND_DIR"
  go run main.go e2e hosts
)

if [[ ${#HOSTS[@]} -eq 0 ]]; then
  echo "error: could not resolve any E2E hostnames from the Go scenario registry." >&2
  exit 1
fi

ENTRIES=()
for host in "${HOSTS[@]}"; do
  ENTRIES+=("127.0.0.1 $host")
done

if [[ $EUID -ne 0 ]]; then
  echo "error: this script must run as root (use: sudo $0)" >&2
  exit 1
fi

current_hosts="$(mktemp)"
updated_hosts="$(mktemp)"
trap 'rm -f "$current_hosts" "$updated_hosts"' EXIT

awk \
  -v begin_marker="$BEGIN_MARKER" \
  -v end_marker="$END_MARKER" \
  -v legacy_marker="$LEGACY_MARKER" '
  $0 == begin_marker {
    in_block = 1
    next
  }
  $0 == end_marker {
    in_block = 0
    next
  }
  in_block {
    next
  }
  $0 == legacy_marker {
    in_legacy = 1
    next
  }
  in_legacy && ($0 ~ /^127\.0\.0\.1[[:space:]]+/ || $0 == "") {
    next
  }
  in_legacy {
    in_legacy = 0
  }
  {
    print
  }
' /etc/hosts > "$current_hosts"

{
  cat "$current_hosts"
  echo
  echo "$BEGIN_MARKER"
  for entry in "${ENTRIES[@]}"; do echo "$entry"; done
  echo "$END_MARKER"
} > "$updated_hosts"

if cmp -s /etc/hosts "$updated_hosts"; then
  echo "/etc/hosts already matches the current Go-owned E2E scenario — nothing to do."
  exit 0
fi

cat "$updated_hosts" > /etc/hosts

echo "Added the following entries to /etc/hosts:"
printf '  %s\n' "${ENTRIES[@]}"
echo
echo "To remove later, delete the block between:"
echo "  $BEGIN_MARKER"
echo "  $END_MARKER"
