#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root/backend"

run_with_default_baseline() {
  local command=$1
  shift
  local use_default=true
  for argument in "$@"; do
    case "$argument" in
      --baseline|--baseline=*) use_default=false ;;
    esac
  done
  if "$use_default"; then
    set -- --baseline "$repo_root/backend/architecture/legacy.jsonl" "$@"
  fi
  exec go run ./internal/architecture/cmd "$command" "$@"
}

case "${1:-}" in
  check)
    shift
    run_with_default_baseline check "$@"
    ;;
  explain)
    shift
    exec go run ./internal/architecture/cmd explain "$@"
    ;;
  audit-issues)
    shift
    run_with_default_baseline audit-issues "$@"
    ;;
  diagram)
    shift
    run_with_default_baseline diagram "$@"
    ;;
  dependencies)
    shift
    run_with_default_baseline dependencies "$@"
    ;;
  *)
    echo "Usage: $0 {check [--project path] [--policy path] [--baseline path] [--base-ref sha]|explain|audit-issues --api-url url|diagram [--output dir]|dependencies --focus module-or-package [--output dir]}" >&2
    exit 2
    ;;
esac
