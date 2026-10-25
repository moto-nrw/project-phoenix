#!/usr/bin/env bash
# Sourced by the test wrappers. Large links and nested architecture builds
# must not compete for a RAM-backed system temporary directory.
PHX_GO_TEST_WORKSPACE=

prepare_go_test_workspace() {
  [ -z "${GOTMPDIR:-}" ] || return 0
  if [ -n "${TMPDIR:-}" ]; then
    export GOTMPDIR="$TMPDIR"
    return 0
  fi
  local go_cache
  go_cache=$(go env GOCACHE)
  [[ "$go_cache" = /* ]] || {
    echo 'Go tests require an absolute build-cache directory.' >&2
    return 1
  }
  mkdir -p "$go_cache"
  PHX_GO_TEST_WORKSPACE=$(mktemp -d "$go_cache/phoenix-tests.XXXXXX")
  # Go test and subprocesses must agree on the system temp root, including
  # architecture checks that confine generated output to os.TempDir().
  export GOTMPDIR="$PHX_GO_TEST_WORKSPACE" TMPDIR="$PHX_GO_TEST_WORKSPACE"
}

cleanup_go_test_workspace() {
  if [ -n "$PHX_GO_TEST_WORKSPACE" ]; then
    rm -r -- "$PHX_GO_TEST_WORKSPACE"
  fi
}
