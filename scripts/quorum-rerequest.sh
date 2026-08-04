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
# The quorum account is taken from the author of the quorum report on the PR.
# Pin it explicitly with `git config quorum.reviewer <login>`. Only that account
# is ever re-requested; other reviewers and teams on the PR are left alone.
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

# A quorum report carries all five of its section headings. Two of them would
# also match an ordinary human comment; the full set is the stable fingerprint,
# and it excludes quorum's own "Review fix round N" notes and other bots.
# `git config quorum.reviewer <login>` pins the account on top of that.
quorum_reviewer=$(git config --get quorum.reviewer 2>/dev/null || echo "")

last_review=$(printf '%s' "$pr_json" | jq -c --arg qr "$quorum_reviewer" '
    def is_quorum_report:
        (.body | test("(^|\n)## Summary"))
        and (.body | test("(^|\n)## Blockers"))
        and (.body | test("(^|\n)## Critical"))
        and (.body | test("(^|\n)## Suggestions"))
        and (.body | test("(^|\n)## Questions"));
    .comments
    | map(select(is_quorum_report and ($qr == "" or .author.login == $qr)))
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

# --- who to re-request ------------------------------------------------------
# Exactly the quorum account, never the other reviewers on the PR: re-requesting
# a human or a team would reset a review they are in the middle of.
target="${quorum_reviewer:-$last_review_by}"

if [[ -z "$target" || "$target" == "$pr_author" ]]; then
    # GitHub refuses a review request from the PR author. quorum reviewing its
    # own operator's PR means `quorum babysit`, which re-triggers itself, so
    # there is nothing to enforce here.
    if [[ "$mode" == "apply" ]]; then
        echo "PR #$pr_number: the quorum review came from the PR author ($pr_author);"
        echo "GitHub does not accept a review request from the author. Nothing to do."
    fi
    mark_clean
    exit 0
fi

# --- did anything change since that review ----------------------------------
# committedDate is when a commit was written, not when it reached the PR, so it
# alone misses a fix that was committed before the review and pushed after it.
# The timeline is the second opinion: commit events that sit behind the review
# comment arrived after it. Either signal is enough.
last_commit_at=$(printf '%s' "$pr_json" | jq -r '[.commits[].committedDate] | max // empty')

repo_slug=$(printf '%s' "$pr_url" | sed -E 's#https://[^/]+/([^/]+/[^/]+)/pull/.*#\1#')
timeline=$(gh api "repos/$repo_slug/issues/$pr_number/timeline" --paginate --slurp 2>/dev/null |
    jq -c 'add // []' 2>/dev/null) || timeline="[]"

# A review_requested event only counts when it aimed at the quorum account -
# requesting some other reviewer must not pass as a fresh quorum round.
timeline_flags=$(printf '%s' "$timeline" | jq -c --arg rev "$last_review_at" --arg tgt "$target" '
    def target_of: (.requested_reviewer.login // .requested_team.slug // "");
    def short: ($tgt | split("/") | last);
    . as $ev
    | ([range(0; length) | select($ev[.].event == "commented" and $ev[.].created_at == $rev)] | last) as $i
    | {
        head_moved: (
            if $i == null then false
            else ($ev[($i + 1):] | map(select(.event == "committed" or .event == "head_ref_force_pushed")) | length > 0)
            end
        ),
        rerequested: (
            $ev
            | map(select(.event == "review_requested"
                and (target_of == $tgt or target_of == short)
                and (.created_at > $rev)))
            | length > 0
        )
    }
' 2>/dev/null) || timeline_flags='{"head_moved":false,"rerequested":false}'

head_moved=$(printf '%s' "$timeline_flags" | jq -r '.head_moved // false')
rerequested=$(printf '%s' "$timeline_flags" | jq -r '.rerequested // false')

pushed_after_review=0
if [[ -n "$last_commit_at" ]] && [[ "$last_commit_at" > "$last_review_at" ]]; then
    pushed_after_review=1
fi
if [[ "$head_moved" == "true" ]]; then
    pushed_after_review=1
fi

if [[ "$force" == "0" && "$pushed_after_review" == "0" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "PR #$pr_number: nothing pushed since the quorum review at $last_review_at."
        echo "Nothing to re-review. Use --force to request a fresh round anyway."
    fi
    mark_clean
    exit 0
fi

if [[ "$force" == "0" && "$rerequested" == "true" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "PR #$pr_number: $target was already re-requested after the review at $last_review_at."
        echo "Use --force to request another round."
    fi
    mark_clean
    exit 0
fi

# --- report / act -----------------------------------------------------------
if [[ "$mode" == "check" ]]; then
    echo "quorum re-request owed on PR #$pr_number (review $last_review_at): $target"
    exit 1
fi

if [[ "$mode" == "stop-hook" ]]; then
    emit_block "BLOCKED: quorum re-request owed on PR #$pr_number.

The quorum review from $last_review_at was answered with pushed commits, and
$target has not been re-requested since. quorum triggers on the review request,
not on new code, so those fixes sit there unreviewed until the reviewer is
removed and added again.

Run this, then finish:
  scripts/quorum-rerequest.sh

Reviewer to re-request: $target
If the user deliberately does not want another review round, say so instead of
running it - and touch .git/quorum-rerequest-off to stop the check for this
clone."
fi

# apply
# Removing a reviewer who is not currently requested errors; that is fine. The
# removal is what makes the following add produce a new review_requested event.
gh pr edit "$pr_number" --remove-reviewer "$target" >/dev/null 2>&1 || true
if ! gh pr edit "$pr_number" --add-reviewer "$target" >/dev/null 2>&1; then
    echo "Failed to re-request review from $target on PR #$pr_number." >&2
    exit 1
fi

echo "Re-requested review from $target on PR #$pr_number."
rm -f "$cache_file" 2>/dev/null || true
