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
lock_dir=$(git rev-parse --git-dir)/phx-test-run.lock
# shellcheck disable=SC2034 # consumed by the sourcing wrapper's EXIT trap
PHX_TEST_RUN_LOCK=

# Renaming the stale directory claims it atomically. Only its winner can try
# to create the replacement lock; every other contender either acquires that
# replacement or falls back to an isolated run ID.
if ! mkdir "$lock_dir" 2>/dev/null &&
    { [ ! -f "$lock_dir/pid" ] || ! kill -0 "$(cat "$lock_dir/pid")" 2>/dev/null; }; then
    stale_lock=$lock_dir.stale.$$
    if mv "$lock_dir" "$stale_lock" 2>/dev/null; then
        mkdir "$lock_dir" 2>/dev/null || true
        rm -rf "$stale_lock"
    fi
fi

if [ -d "$lock_dir" ] &&
    (set -o noclobber; echo $$ >"$lock_dir/pid") 2>/dev/null; then
    PHX_TEST_RUN_LOCK=$lock_dir
    # git hash-object: deterministic lowercase hex without a new dependency;
    # 12 chars match testdb.SanitizeRunID's ^[a-z0-9]{1,16}$ pass-through.
    PHX_TEST_RUN_ID=$(git rev-parse --show-toplevel | git hash-object --stdin | cut -c1-12)
else
    echo "==> another test run holds this worktree's run ID; using a throwaway one (no test cache)" >&2
    # shellcheck disable=SC2034 # consumed by the sourcing wrapper's EXIT trap
    PHX_TEST_RUN_LOCK=
    PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
fi
export PHX_TEST_RUN_ID
