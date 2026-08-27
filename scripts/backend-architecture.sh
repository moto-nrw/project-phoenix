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
  legacy-check)
    go_arch_lint=$(go -C tools tool -n go-arch-lint)
    exec "$go_arch_lint" check \
      --project-path . \
      --arch-file .go-arch-lint.yml
    ;;
  diagram)
    output=${2:-/tmp/phoenix-backend-architecture.svg}
    go_arch_lint=$(go -C tools tool -n go-arch-lint)
    "$go_arch_lint" graph \
      --project-path . \
      --arch-file .go-arch-lint.yml \
      --focus handlers \
      --type flow \
      --out "$output"
    echo "$output"
    ;;
  dependencies)
    command -v dot >/dev/null || {
      echo "Graphviz is required; enter the Devbox shell first" >&2
      exit 1
    }
    output=${2:-/tmp/phoenix-backend-dependencies.svg}
    expression=${3:-'goos=linux(goarch=amd64(./...:module))'}
    goda=$(go -C tools tool -n goda)
    "$goda" graph "$expression" | dot -Tsvg -o "$output"
    echo "$output"
    ;;
  *)
    echo "Usage: $0 {check [--project path] [--policy path] [--baseline path] [--base-ref sha]|explain|audit-issues --baseline path|legacy-check|diagram [output.svg]|dependencies [output.svg] [goda-expression]}" >&2
    exit 2
    ;;
esac
