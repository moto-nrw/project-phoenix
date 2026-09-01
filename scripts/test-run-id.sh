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
lock_file=$(git rev-parse --git-dir)/phx-test-run.lock
reclaim_file=${lock_file}.reclaim
# shellcheck disable=SC2034 # consumed by the sourcing wrapper's EXIT trap
PHX_TEST_RUN_LOCK=

# Write the owner marker before publishing it. `ln` atomically creates the
# lock name, so a contender never sees an acquired lock without its owner.
owner_file=$(mktemp "${lock_file}.owner.XXXXXX") || owner_file=
if [ -n "$owner_file" ]; then
    printf '%s\n' "$$" >"$owner_file"
    if ln "$owner_file" "$lock_file" 2>/dev/null; then
        PHX_TEST_RUN_LOCK=$lock_file
    else
        lock_pid=
        if [ -f "$lock_file" ]; then
            IFS= read -r lock_pid <"$lock_file" || true
        fi
        # A lock is reclaimed only when its complete, numeric owner marker is
        # present and that process is gone. Invalid and legacy directory locks
        # remain contended rather than risking an overlap.
        if [[ "$lock_pid" =~ ^[1-9][0-9]*$ ]] && ! kill -0 "$lock_pid" 2>/dev/null &&
            ln "$owner_file" "$reclaim_file" 2>/dev/null; then
            candidate_pid=
            IFS= read -r candidate_pid <"$lock_file" || true
            if [ "$candidate_pid" = "$lock_pid" ] && ! kill -0 "$candidate_pid" 2>/dev/null; then
                stale_lock=${lock_file}.stale.$$
                if mv "$lock_file" "$stale_lock" 2>/dev/null; then
                    if ln "$owner_file" "$lock_file" 2>/dev/null; then
                        PHX_TEST_RUN_LOCK=$lock_file
                    fi
                    rm -f "$stale_lock"
                fi
            fi
            # Reclaim markers are deliberately never recovered. A crash in
            # this tiny window only disables the stable cache; it cannot let
            # another process rename a newly acquired run lock.
            rm -f "$reclaim_file"
        fi
    fi
    rm -f "$owner_file"
fi

if [ -n "$PHX_TEST_RUN_LOCK" ]; then
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
