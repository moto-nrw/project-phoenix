#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root/backend"
export PHOENIX_ARCHITECTURE_PROJECT="$repo_root/backend"

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
  cd "$repo_root/scripts/backend-architecture"
  exec go run . "$command" "$@"
}

case "${1:-}" in
  check)
    shift
    has_check_options=false
    for argument in "$@"; do
      case "$argument" in
        --base-ref|--base-ref=*|--project|--project=*|--baseline|--baseline=*) has_check_options=true ;;
      esac
    done
    if ! "$has_check_options"; then
      base_sha=$(git merge-base HEAD origin/development)
      set -- --base-ref "$base_sha" "$@"
    fi
    run_with_default_baseline check "$@"
    ;;
  explain)
    shift
    cd "$repo_root/scripts/backend-architecture"
    exec go run . explain "$@"
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
  validate-ticket)
    shift
    cd "$repo_root/scripts/backend-architecture"
    exec go run . validate-ticket "$@"
    ;;
  *)
    echo "Usage: $0 {check [--project path] [--policy path] [--baseline path] [--base-ref sha]|explain|audit-issues --api-url url|diagram [--output dir]|dependencies --focus module-or-package [--output dir]|validate-ticket --ticket path}" >&2
    exit 2
    ;;
esac
