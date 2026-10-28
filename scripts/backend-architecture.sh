#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
# Hooks export repository-local Git variables. Clear them before child commands
# change directory, so both the backend and isolated fixtures resolve correctly.
while IFS= read -r git_variable; do
  unset "$git_variable"
done < <(git rev-parse --local-env-vars)
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
  policy-export|passthrough-snapshot|passthrough-import)
    command=$1
    shift
    if [[ "$command" == passthrough-import && "$#" -eq 1 && "$1" != /* ]]; then
      set -- "$repo_root/$1"
    fi
    cd "$repo_root/scripts/backend-architecture"
    exec go run . "$command" "$@"
    ;;
  effects)
    shift
    cd "$repo_root/scripts/backend-architecture"
    exec go run . effects "$@"
    ;;
  relocations)
    shift
    cd "$repo_root/scripts/backend-architecture"
    exec go run . relocations "$@"
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
  cycles)
    shift
    run_with_default_baseline cycles "$@"
    ;;
  *)
    echo "Usage: $0 {policy-export|passthrough-snapshot [--decision NNNN]|passthrough-import <reviewed.json>|check [--project path] [--policy path] [--baseline path] [--base-ref sha]|effects [--project path] [--policy path]|explain|audit-issues --api-url url|diagram [--output dir]|dependencies --focus module-or-package [--output dir]|cycles [--graph target|migration|both] [--json]|relocations --base-ref sha}" >&2
    exit 2
    ;;
esac
