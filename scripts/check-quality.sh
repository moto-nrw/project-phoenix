#!/usr/bin/env bash
# Shared CI/local entry point; stack-specific commands live separately so a
# frontend quality edit does not select the backend, or vice versa.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
case "${1:-}" in
  backend) exec bash "$root/scripts/check-backend-quality.sh" quality ;;
  frontend) exec bash "$root/scripts/check-frontend-quality.sh" quality ;;
  react-doctor) exec bash "$root/scripts/check-frontend-quality.sh" react-doctor "${2:?base revision is required}" ;;
  *) echo 'Usage: check-quality.sh backend|frontend|react-doctor [base]' >&2; exit 2 ;;
esac
