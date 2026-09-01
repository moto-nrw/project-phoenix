#!/usr/bin/env bash
# Sourced by the test wrappers. Exports PHX_TEST_RUN_ID, stable per worktree:
# every DB test binary reads this variable, so a fresh random ID per
# invocation lands in every DB package's Go test cache key and misses every
# single time (measured: 114s -> 3.1s for an unchanged full-suite rerun).
#
# A stable ID means clone NAMES are stable too, so two overlapping runs in
# this worktree would fight over the same databases: CreateClone drops and
# recreates WITH (FORCE) what the other run is querying, and the exit sweep
# drops the other run's clones. The lock prevents the overlap; a run that
# cannot take it falls back to a throwaway random ID and only loses the
# cache, never correctness.
#
# The caller owns the cleanup: bash allows one EXIT trap and both wrappers
# already use theirs for the sweep, so they remove "$PHX_TEST_RUN_LOCK"
# there (guarded, since the fallback path leaves it empty).
PHX_TEST_RUN_LOCK=$(git rev-parse --git-dir)/phx-test-run.lock
if mkdir "$PHX_TEST_RUN_LOCK" 2>/dev/null ||
    { [ -f "$PHX_TEST_RUN_LOCK/pid" ] &&
        ! kill -0 "$(cat "$PHX_TEST_RUN_LOCK/pid")" 2>/dev/null; }; then
    echo $$ >"$PHX_TEST_RUN_LOCK/pid"
    # git hash-object: deterministic lowercase hex without a new dependency;
    # 12 chars match testdb.SanitizeRunID's ^[a-z0-9]{1,16}$ pass-through.
    PHX_TEST_RUN_ID=$(git rev-parse --show-toplevel | git hash-object --stdin | cut -c1-12)
else
    echo "==> another test run holds this worktree's run ID; using a throwaway one (no test cache)" >&2
    PHX_TEST_RUN_LOCK=
    PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
fi
export PHX_TEST_RUN_ID
