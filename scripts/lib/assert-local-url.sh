#!/usr/bin/env bash
# ==============================================================================
# Project Phoenix — Local URL Guard
#
# Refuses any URL whose hostname is not localhost, a loopback IP, or a known
# Docker-internal hostname. Sourced by E2E seed scripts to make it impossible
# to point them at staging or production by accident — the password the
# seeder writes is well-known, so a misconfigured run would compromise a
# real environment.
#
# Mirrors:
#   - frontend/e2e/helpers/safeguard.ts (TS)
#   - backend/cmd/safeguard.go (Go)
# Keep the allowlist in sync across all three.
#
# Usage (from a script in scripts/):
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/lib/assert-local-url.sh"
#   assert_local_url "$API_URL"
# ==============================================================================

assert_local_url() {
  local url=$1
  if [[ -z "$url" ]]; then
    echo "assert_local_url: empty URL" >&2
    exit 1
  fi

  # Strip scheme, then everything after the first / or :, lowercased.
  # NOTE: Bracketed IPv6 URLs (http://[::1]:8081) are not supported here —
  # the second sed splits on the first colon and would mangle the address.
  # Currently fine because the allowlist only contains the bare ::1 form
  # (used as a Docker hostname), not the URL form. Add a dedicated IPv6
  # branch if a caller ever needs http://[::1]/.
  local host
  host=$(printf '%s' "$url" \
    | sed -E 's#^[a-zA-Z][a-zA-Z0-9+.-]*://##' \
    | sed -E 's#[/:].*##' \
    | tr '[:upper:]' '[:lower:]')

  if [[ -z "$host" ]]; then
    echo "assert_local_url: could not parse hostname from $url" >&2
    exit 1
  fi

  case "$host" in
    localhost|::1|server|server-e2e|host.docker.internal|gateway.docker.internal)
      return 0
      ;;
    *)
      # Fall through to the IPv4-loopback regex check below; non-loopback
      # hosts are rejected by the final exit 1.
      ;;
  esac

  # Any IPv4 loopback (127.0.0.0/8).
  if [[ "$host" =~ ^127\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    return 0
  fi

  echo "error: refusing to target non-local URL: $url" >&2
  echo "       only localhost / loopback / Docker-internal hostnames are allowed" >&2
  exit 1
}
