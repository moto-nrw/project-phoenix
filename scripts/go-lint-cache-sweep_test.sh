#!/usr/bin/env bash
# Run with the pinned Go and golangci-lint toolchain. Exercise the warmer's
# cache directories: warm issue results must not cost dependency facts when
# a later source change requires analysis again.
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
sweep="$repo_root/scripts/go-cache-sweep.sh"
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"; [ ! -e "$fixture" ]' EXIT
export GOCACHE="$fixture/go-build" GOLANGCI_LINT_CACHE="$fixture/golangci-lint"
export GOFLAGS='' CGO_ENABLED=0 GOTOOLCHAIN=local
export GL_DEBUG=goanalysis/issues/cache,goanalysis/facts/cache
version=$(awk '$1 == "go" {print $2; exit}' "$repo_root/backend/go.mod")
mkdir -p "$fixture/mod/dependency"
printf 'module lintprobe\n\ngo %s\n\nrequire dependency v0.0.0\nreplace dependency => ./dependency\n' "$version" >"$fixture/mod/go.mod"
printf 'module dependency\n\ngo %s\n' "$version" >"$fixture/mod/dependency/go.mod"
printf 'package dependency\nfunc Value() *int { return new(int) }\n' >"$fixture/mod/dependency/value.go"
printf 'package lintprobe\nimport "dependency"\nfunc Value() int { return *dependency.Value() }\n' >"$fixture/mod/value.go"

fail() { cat "$fixture/lint.log" >&2; echo "FAIL: $*" >&2; exit 1; }
lint() {
  (cd "$fixture/mod" && golangci-lint run --no-config --enable-only staticcheck ./...) >"$fixture/lint.log" 2>&1 || fail "lint failed"
}
workflow_sweep() {
  local command=$1 dir
  local dirs=()
  # Read the real warm-lint commands, not a second copy of their directory list.
  while IFS= read -r dir; do
    case "$dir" in
      \~/.cache/go-build) dirs+=("$GOCACHE") ;;
      \~/.cache/golangci-lint) dirs+=("$GOLANGCI_LINT_CACHE") ;;
      *) fail "unexpected cache directory: $dir" ;;
    esac
  done < <(awk -v command="$command" '
    /^  warm-lint:/ { lint = 1 }
    lint && $1 == "run:" && $2 == "scripts/go-cache-sweep.sh" && $3 == command {
      for (i = 5; i <= NF; i++) print $i
    }
  ' "$repo_root/.github/workflows/backend-build-cache.yml")
  [ "${#dirs[@]}" -gt 0 ] || fail "missing warm-lint $command command"
  "$sweep" "$command" "$fixture/marker" "${dirs[@]}"
}

lint
workflow_sweep mark
lint
grep -q 'analyzing 0/1 packages' "$fixture/lint.log" || fail "warm lint did not reuse issue results"
workflow_sweep sweep
printf 'package lintprobe\nimport "dependency"\nfunc Value() int { return *dependency.Value() + 1 }\n' >"$fixture/mod/value.go"
lint
grep -q 'analyzing 1/1 packages' "$fixture/lint.log" || fail "source change did not invalidate issue results"
grep -q 'Loaded [1-9][0-9]* cached facts for package .*dependency' "$fixture/lint.log" || fail "sweep removed reusable dependency facts"
if grep -q 'No cached facts for package .*dependency' "$fixture/lint.log"; then
  fail "dependency analysis had to rebuild cached facts"
fi
echo "go-lint-cache-sweep: ok"
