#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root/backend"

case "${1:-}" in
  check)
    shift
    exec go run ./internal/architecture/cmd check "$@"
    ;;
  explain)
    shift
    exec go run ./internal/architecture/cmd explain "$@"
    ;;
  audit-issues)
    shift
    exec go run ./internal/architecture/cmd audit-issues "$@"
    ;;
  diagram)
    shift
    exec go run ./internal/architecture/cmd diagram "$@"
    ;;
  dependencies)
    shift
    exec go run ./internal/architecture/cmd dependencies "$@"
    ;;
  legacy-check)
    go_arch_lint=$(go -C tools tool -n go-arch-lint)
    exec "$go_arch_lint" check \
      --project-path . \
      --arch-file .go-arch-lint.yml
    ;;
  *)
    echo "Usage: $0 {check [--project path] [--policy path] [--baseline path] [--base-ref sha]|explain|audit-issues --baseline path --api-url url|legacy-check|diagram [--output dir]|dependencies --focus module-or-package [--output dir]}" >&2
    exit 2
    ;;
esac
