#!/usr/bin/env bash
# quorum-rerequest.sh - re-trigger a quorum review after fixing its findings.
#
# quorum reviews on a review REQUEST, not on new code (quorum README: "The
# trigger is the review request, not the code. One request is one review, so
# pushing more commits does not trigger another one; re-request the review to
# get a fresh one."). quorum posts its report as a plain PR comment, so GitHub
# never clears the pending request - which makes a bare `gh pr edit
# --add-reviewer` a no-op that fires no notification. The reviewer has to be
# removed and added again to produce a fresh review_requested event.
#
# Usage: quorum-rerequest.sh [--check|--stop-hook] [--force] [pr-number]
#
# Modes:
#   (default)     perform the remove/add round trip; without a PR number the
#                 open PR of the current branch is used
#   --force       re-request even when nothing new was pushed
#   --check       report whether a re-request is owed; exit 1 when it is
#   --stop-hook   Stop-hook protocol: exit 2 (with instructions on stderr)
#                 when a re-request is owed and the work looks finished
#
# Opt out per clone with `touch .git/quorum-rerequest-off`, per invocation
# with QUORUM_RERQ_OFF=1.
#
# Kept bash-3.2 compatible (macOS /bin/bash).

set -euo pipefail

mode="apply"
force=0
pr_arg=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --check) mode="check" ;;
        --stop-hook) mode="stop-hook" ;;
        --force) force=1 ;;
        -h | --help)
            sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        [0-9]*) pr_arg="$1" ;;
        *)
            echo "unknown option: $1" >&2
            exit 64
            ;;
    esac
    shift
done

# A hook must never break the session over a missing tool or a detached repo.
soft_exit() { exit 0; }

if ! command -v gh >/dev/null 2>&1; then soft_exit; fi
if ! command -v jq >/dev/null 2>&1; then soft_exit; fi
if ! project_root=$(git rev-parse --show-toplevel 2>/dev/null); then soft_exit; fi
cd "$project_root"
git_dir=$(git rev-parse --git-dir 2>/dev/null) || soft_exit

if [[ -n "${QUORUM_RERQ_OFF:-}" ]]; then soft_exit; fi
if [[ -f "$git_dir/quorum-rerequest-off" ]]; then soft_exit; fi

cache_file="$git_dir/quorum-rerequest.cache"

# --- stop-hook input --------------------------------------------------------
# stop_hook_active is true when the agent is already continuing because of a
# Stop hook. Blocking again there loops forever.
if [[ "$mode" == "stop-hook" ]]; then
    hook_input=""
    if [[ ! -t 0 ]]; then
        IFS= read -r -d '' -t 1 hook_input || true
    fi
    if [[ -n "$hook_input" ]]; then
        active=$(printf '%s' "$hook_input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)
        if [[ "$active" == "true" ]]; then soft_exit; fi
    fi

    # Only block when the work actually looks finished: uncommitted changes or
    # unpushed commits mean the fix round is still running, and nagging there
    # is noise. Untracked files are ignored on purpose - a stray screenshot or
    # scratch file must not be able to switch the check off by accident.
    if [[ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]]; then soft_exit; fi
    upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null) || soft_exit
    if [[ -n "$(git log --oneline "$upstream..HEAD" 2>/dev/null)" ]]; then soft_exit; fi

    # Negative results are cached per HEAD sha, so the API round trip only
    # happens on turns that actually moved the branch.
    head_sha=$(git rev-parse HEAD 2>/dev/null) || soft_exit
    if [[ -f "$cache_file" ]] && [[ "$(cat "$cache_file" 2>/dev/null)" == "clean $head_sha" ]]; then
        soft_exit
    fi
fi

mark_clean() {
    if [[ "$mode" == "stop-hook" ]]; then
        printf 'clean %s' "$(git rev-parse HEAD 2>/dev/null || echo unknown)" >"$cache_file" 2>/dev/null || true
    fi
}

# Both Claude Code and Codex accept a Stop hook blocking either through exit
# code 2 with the reason on stderr, or through {"decision":"block","reason":…}
# on stdout. Emitting both makes the block independent of which channel the
# running tool prefers.
emit_block() {
    local reason="$1"
    jq -n --arg reason "$reason" '{decision: "block", reason: $reason}'
    printf '%s\n' "$reason" >&2
    exit 2
}

# --- PR state ---------------------------------------------------------------
pr_fields="number,state,url,author,reviewRequests,comments,commits"
if [[ -n "$pr_arg" ]]; then
    pr_json=$(gh pr view "$pr_arg" --json "$pr_fields" 2>/dev/null) || soft_exit
else
    pr_json=$(gh pr view --json "$pr_fields" 2>/dev/null) || soft_exit
fi
if [[ -z "$pr_json" ]]; then soft_exit; fi

if [[ "$(printf '%s' "$pr_json" | jq -r '.state // empty')" != "OPEN" ]]; then soft_exit; fi

pr_number=$(printf '%s' "$pr_json" | jq -r '.number')
pr_url=$(printf '%s' "$pr_json" | jq -r '.url')
pr_author=$(printf '%s' "$pr_json" | jq -r '.author.login // empty')

# A quorum report is a comment carrying both the Summary and the Blockers
# heading. That excludes quorum's own "Review fix round N" progress notes and
# third-party bot comments.
last_review=$(printf '%s' "$pr_json" | jq -c '
    .comments
    | map(select((.body | test("(^|\n)## Summary")) and (.body | test("(^|\n)## Blockers"))))
    | last // {}
')
last_review_at=$(printf '%s' "$last_review" | jq -r '.createdAt // empty')
last_review_by=$(printf '%s' "$last_review" | jq -r '.author.login // empty')

# Nothing to re-trigger without a review to answer.
if [[ -z "$last_review_at" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "No quorum review found on PR #$pr_number - nothing to re-request."
    fi
    mark_clean
    exit 0
fi

# ISO-8601 Z timestamps compare correctly as strings.
last_commit_at=$(printf '%s' "$pr_json" | jq -r '[.commits[].committedDate] | max // empty')

pushed_after_review=1
if [[ -z "$last_commit_at" ]] || [[ ! "$last_commit_at" > "$last_review_at" ]]; then
    pushed_after_review=0
fi

if [[ "$force" == "0" && "$pushed_after_review" == "0" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "PR #$pr_number: no commits pushed since the quorum review at $last_review_at."
        echo "Nothing to re-review. Use --force to request a fresh round anyway."
    fi
    mark_clean
    exit 0
fi

# Only now, when a re-request is plausible, pay for the timeline call.
repo_slug=$(printf '%s' "$pr_url" | sed -E 's#https://[^/]+/([^/]+/[^/]+)/pull/.*#\1#')
last_request_at=$(gh api "repos/$repo_slug/issues/$pr_number/timeline" --paginate \
    --jq '[.[] | select(.event == "review_requested") | .created_at] | max // empty' 2>/dev/null || echo "")

if [[ "$force" == "0" && -n "$last_request_at" ]] && [[ "$last_request_at" > "$last_review_at" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "PR #$pr_number was already re-requested at $last_request_at (review: $last_review_at)."
        echo "Use --force to request another round."
    fi
    mark_clean
    exit 0
fi

# --- who to re-request ------------------------------------------------------
# Whoever is on the PR right now, else the account that posted the review.
# GitHub refuses a review request from the PR author, so that case is reported
# rather than failed on.
targets=$(printf '%s' "$pr_json" |
    jq -r '.reviewRequests[]? | (.login // .slug // empty)' | sed '/^$/d')

if [[ -z "$targets" && -n "$last_review_by" && "$last_review_by" != "$pr_author" ]]; then
    targets="$last_review_by"
fi

if [[ -z "$targets" ]]; then
    no_target_msg="quorum re-request owed on PR #$pr_number, but no reviewer could be determined:
nobody is requested on the PR, and the review came from the PR author, whom
GitHub refuses as a reviewer.

Ask the user who should be re-requested, then run:
  gh pr edit $pr_number --add-reviewer <login-or-org/team>"
    # Blocking here too: silently dropping this case is the exact failure the
    # hook exists to prevent.
    if [[ "$mode" == "stop-hook" ]]; then emit_block "$no_target_msg"; fi
    printf '%s\n' "$no_target_msg" >&2
    exit 1
fi

targets_flat=$(printf '%s' "$targets" | tr '\n' ' ' | sed 's/ *$//')

# --- report / act -----------------------------------------------------------
if [[ "$mode" == "check" ]]; then
    echo "quorum re-request owed on PR #$pr_number (review $last_review_at, head $last_commit_at): $targets_flat"
    exit 1
fi

if [[ "$mode" == "stop-hook" ]]; then
    emit_block "BLOCKED: quorum re-request owed on PR #$pr_number.

The quorum review from $last_review_at was answered with commits pushed
afterwards ($last_commit_at), and no review has been re-requested since. quorum
triggers on the review request, not on new code, so those fixes sit there
unreviewed until the reviewer is removed and added again.

Run this, then finish:
  scripts/quorum-rerequest.sh

Reviewers to re-request: $targets_flat
If the user deliberately does not want another review round, say so instead of
running it - and touch .git/quorum-rerequest-off to stop the check for this
clone."
fi

# apply
exit_code=0
for target in $targets; do
    # Removing a reviewer who is not currently requested errors; that is fine.
    gh pr edit "$pr_number" --remove-reviewer "$target" >/dev/null 2>&1 || true
    if gh pr edit "$pr_number" --add-reviewer "$target" >/dev/null 2>&1; then
        echo "Re-requested review from $target on PR #$pr_number."
    else
        echo "Failed to re-request review from $target on PR #$pr_number." >&2
        exit_code=1
    fi
done

if [[ "$exit_code" == "0" ]]; then
    rm -f "$cache_file" 2>/dev/null || true
fi
exit "$exit_code"
