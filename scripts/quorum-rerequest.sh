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
# The quorum account is declared, not guessed: `git config quorum.reviewer
# <login>` per clone, otherwise the tracked `.github/quorum-reviewer`. Only that
# account is ever re-requested; other reviewers and teams are left alone.
#
# Exit codes: 0 nothing owed / re-request performed, 1 re-request owed
# (--check) or the remove/add round trip failed, 2 Stop-hook block, 3 unable
# to determine (missing tool, API failure). --stop-hook never exits 3: it
# stays silent on environment problems and blocks on an inconclusive check.
#
# The opt-outs (`touch .git/quorum-rerequest-off` per clone, QUORUM_RERQ_OFF=1
# per invocation) silence only the Stop hook; manual and --check runs always
# answer.
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
            sed -n '2,33p' "$0" | sed 's/^# \{0,1\}//'
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

# The Stop hook must never wedge a session over a missing tool or a broken
# lookup - but a manual or --check run reporting success for a skipped action
# is exactly the silent failure this script exists to prevent. Environment
# problems are therefore silent (exit 0) only in stop-hook mode and loud
# (exit 3) everywhere else.
soft_exit() { exit 0; }
bail() {
    if [[ "$mode" == "stop-hook" ]]; then exit 0; fi
    echo "quorum-rerequest: $1" >&2
    exit 3
}

command -v gh >/dev/null 2>&1 || bail "gh is not installed"
command -v jq >/dev/null 2>&1 || bail "jq is not installed"
project_root=$(git rev-parse --show-toplevel 2>/dev/null) || bail "not inside a git repository"
cd "$project_root"
git_dir=$(git rev-parse --git-dir 2>/dev/null) || bail "cannot resolve the git directory"

# Deliberate opt-outs silence only the enforcement path.
if [[ "$mode" == "stop-hook" ]]; then
    if [[ -n "${QUORUM_RERQ_OFF:-}" ]]; then soft_exit; fi
    if [[ -f "$git_dir/quorum-rerequest-off" ]]; then soft_exit; fi
fi

cache_file="$git_dir/quorum-rerequest.cache"

# Set when the local push state cannot be settled from local refs alone; the
# answer then comes from comparing the PR head with the local HEAD below.
check_pr_head=0

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

    # A branch can be fully pushed without tracking information (git push
    # origin HEAD without -u), so a missing upstream must not switch the
    # enforcement off. Fall back to the matching origin ref; when neither
    # exists, the push state is settled against the PR head further down.
    upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null) || upstream=""
    if [[ -z "$upstream" ]]; then
        branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null) || branch=""
        if [[ -n "$branch" && "$branch" != "HEAD" ]] &&
            git rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null 2>&1; then
            upstream="origin/$branch"
        fi
    fi
    if [[ -n "$upstream" ]]; then
        if [[ -n "$(git log --oneline "$upstream..HEAD" 2>/dev/null)" ]]; then soft_exit; fi
    else
        check_pr_head=1
    fi

    # Negative results are cached per HEAD sha, so the API round trip only
    # happens on turns that actually moved the branch.
    head_sha=$(git rev-parse HEAD 2>/dev/null) || soft_exit
    if [[ -f "$cache_file" ]] && [[ "$(cat "$cache_file" 2>/dev/null)" == "clean $head_sha" ]]; then
        soft_exit
    fi
fi

# can_cache drops to 0 as soon as a lookup was inconclusive, so an incomplete
# answer is never frozen into the per-HEAD cache.
can_cache=1

mark_clean() {
    if [[ "$mode" == "stop-hook" && "$can_cache" == "1" ]]; then
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

# --- who runs quorum here ---------------------------------------------------
# The account is declared, never guessed: this script removes and re-adds a
# review request, and inferring the account from comment shape alone would let a
# human who writes the same headings have their pending review reset.
# Per clone `git config quorum.reviewer` wins, otherwise the tracked
# .github/quorum-reviewer applies. Resolved before any API call, so repos
# without quorum pay nothing per turn.
target=$(git config --get quorum.reviewer 2>/dev/null || echo "")
if [[ -z "$target" && -f "$project_root/.github/quorum-reviewer" ]]; then
    target=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' "$project_root/.github/quorum-reviewer" |
        sed '/^$/d' | head -n 1)
fi

if [[ -z "$target" ]]; then
    # Without a declared account there is nothing safe to re-request.
    bail "no quorum account configured - set 'git config quorum.reviewer <login>' or commit .github/quorum-reviewer"
fi

# --- PR state ---------------------------------------------------------------
pr_fields="number,state,url,author,reviewRequests,headRefOid"
if ! pr_json=$(gh pr view ${pr_arg:+"$pr_arg"} --json "$pr_fields" 2>&1); then
    case "$pr_json" in
        *"no pull requests found"* | *"Could not resolve"* | *"no default remote"*)
            # An explicitly named PR that cannot be resolved is an error; a
            # branch without an open PR is genuinely "nothing owed" - in every
            # mode, not just --check.
            if [[ -n "$pr_arg" ]]; then
                bail "no open PR found for '$pr_arg'"
            fi
            if [[ "$mode" != "stop-hook" ]]; then
                echo "No open PR here - nothing owed."
            fi
            exit 0
            ;;
        *) bail "gh pr view failed: $pr_json" ;;
    esac
fi
[[ -n "$pr_json" ]] || bail "gh pr view returned nothing"

pr_number=$(printf '%s' "$pr_json" | jq -r '.number')
pr_url=$(printf '%s' "$pr_json" | jq -r '.url')
pr_author=$(printf '%s' "$pr_json" | jq -r '.author.login // empty')
pr_state=$(printf '%s' "$pr_json" | jq -r '.state // empty')

if [[ "$pr_state" != "OPEN" ]]; then
    if [[ "$mode" != "stop-hook" ]]; then
        echo "PR #$pr_number is $pr_state - nothing to re-request."
    fi
    exit 0
fi

# Without a local tracking ref, "is everything pushed" is answered by the PR
# itself: a head that does not match the local HEAD (or cannot be read) means
# the fix round is still in flight - not a finished turn. Never cached: the
# next push changes the answer.
if [[ "$mode" == "stop-hook" && "$check_pr_head" == "1" ]]; then
    pr_head=$(printf '%s' "$pr_json" | jq -r '.headRefOid // empty')
    if [[ -z "$pr_head" || "$pr_head" != "$(git rev-parse HEAD 2>/dev/null)" ]]; then
        soft_exit
    fi
fi

repo_slug=$(printf '%s' "$pr_url" | sed -E 's#https://[^/]+/([^/]+/[^/]+)/pull/.*#\1#')

# Comments come from the REST endpoint with explicit pagination - provably
# complete on any PR size. An unanswerable lookup is inconclusive, not clean.
comments="[]"
comments_ok=0
for _ in 1 2 3; do
    if comments_raw=$(gh api "repos/$repo_slug/issues/$pr_number/comments" --paginate --slurp 2>/dev/null) &&
        comments=$(printf '%s' "$comments_raw" | jq -c 'add // []' 2>/dev/null); then
        comments_ok=1
        break
    fi
    sleep 1
done
if [[ "$comments_ok" == "0" ]]; then
    can_cache=0
    if [[ "$mode" == "stop-hook" ]]; then
        emit_block "BLOCKED: quorum re-request status on PR #$pr_number is INCONCLUSIVE.

The PR's comments could not be fetched, so it is unknown whether a quorum
review exists that was answered without a re-request. Do not report this work
as finished. Retry:
  scripts/quorum-rerequest.sh --check
If GitHub is unreachable and the user wants to finish anyway, they can opt out
with QUORUM_RERQ_OFF=1 or: touch .git/quorum-rerequest-off"
    fi
    bail "could not fetch comments for PR #$pr_number"
fi

# A quorum report comes from that account and carries all five of its section
# headings, which excludes quorum's own "Review fix round N" notes.
last_review=$(printf '%s' "$comments" | jq -c --arg qr "$target" '
    def is_quorum_report:
        (.body | test("(^|\n)## Summary"))
        and (.body | test("(^|\n)## Blockers"))
        and (.body | test("(^|\n)## Critical"))
        and (.body | test("(^|\n)## Suggestions"))
        and (.body | test("(^|\n)## Questions"));
    map(select(.user.login == $qr and is_quorum_report))
    | last // {}
')
last_review_at=$(printf '%s' "$last_review" | jq -r '.created_at // empty')
last_review_id=$(printf '%s' "$last_review" | jq -r '.id // empty')

# Nothing to re-trigger without a review to answer.
if [[ -z "$last_review_at" ]]; then
    if [[ "$mode" == "apply" ]]; then
        echo "No quorum review found on PR #$pr_number - nothing to re-request."
    fi
    mark_clean
    exit 0
fi

if [[ "$target" == "$pr_author" ]]; then
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
# The timeline is the only authority on "was something pushed after the
# review": commit TIMESTAMPS (committedDate) are useless in both directions -
# a fix committed before the review and pushed after it carries an old date
# (missed re-request), a preserved or future-dated commit pushed before the
# review carries a newer one (false alarm). Only the POSITION of head-movement
# events relative to the report comment is trustworthy. The report is located
# by its comment ID, which the timeline "commented" event carries verbatim -
# created_at is NOT unique (two comments can share a second, and `last` would
# then anchor on the wrong one).

# A failed timeline call must not read as "nothing changed" and must never be
# cached as clean, otherwise one flaky request silences the check for this HEAD.
# Retried, because a transient blip downgrading the whole check to
# "inconclusive" (below) is worth two seconds.
timeline="[]"
timeline_ok=0
for _ in 1 2 3; do
    if timeline_raw=$(gh api "repos/$repo_slug/issues/$pr_number/timeline" --paginate --slurp 2>/dev/null) &&
        timeline=$(printf '%s' "$timeline_raw" | jq -c 'add // []' 2>/dev/null); then
        timeline_ok=1
        break
    fi
    sleep 1
done
if [[ "$timeline_ok" == "0" ]]; then
    timeline="[]"
    can_cache=0
fi

# head_moved is three-valued: true / false / unknown. Unknown covers a dead
# timeline AND a timeline in which the report comment cannot be located - in
# both cases "nothing changed" would be a guess, and a guess must not end a
# turn cleanly.
# A review_requested event only counts when it aimed at the quorum account and
# sits behind the last head movement: a re-request followed by further commits
# leaves those commits unreviewed, so it does not settle the loop.
timeline_flags=$(printf '%s' "$timeline" | jq -c \
    --arg rid "$last_review_id" --arg tgt "$target" '
    def target_of: (.requested_reviewer.login // .requested_team.slug // "");
    def short: ($tgt | split("/") | last);
    def is_head_move: (.event == "committed" or .event == "head_ref_force_pushed");
    . as $ev
    | ([range(0; length)
        | select($ev[.].event == "commented" and (($ev[.].id | tostring) == $rid))] | last) as $i
    | ([range(0; length) | select($ev[.] | is_head_move)] | last) as $h
    | {
        head_moved: (
            if $i == null then "unknown"
            elif ($ev[($i + 1):] | map(select(is_head_move)) | length > 0) then "true"
            else "false"
            end
        ),
        rerequested: (
            [range(0; length)
             | select(
                 ($ev[.].event == "review_requested")
                 and ((($ev[.] | target_of) == $tgt) or (($ev[.] | target_of) == short))
                 and ($i != null and . > $i)
                 and ($h == null or . > $h))]
            | length > 0
        )
    }
' 2>/dev/null) || timeline_flags='{"head_moved":"unknown","rerequested":false}'

head_moved=$(printf '%s' "$timeline_flags" | jq -r '.head_moved // "unknown"')
rerequested=$(printf '%s' "$timeline_flags" | jq -r '.rerequested // false')

if [[ "$timeline_ok" == "0" ]]; then
    head_moved="unknown"
fi

# Unknown is not clean - the Stop hook blocks and says so, the other modes
# exit 3. --force skips the question entirely.
if [[ "$head_moved" == "unknown" && "$force" == "0" ]]; then
    can_cache=0
    if [[ "$mode" == "stop-hook" ]]; then
        emit_block "BLOCKED: quorum re-request status on PR #$pr_number is INCONCLUSIVE.

A quorum review from $last_review_at exists, but the PR timeline could not be
read (or the report could not be located in it), so it is unknown whether
fixes were pushed since and whether $target was re-requested. Do not report
this work as finished. Retry:
  scripts/quorum-rerequest.sh --check
If GitHub is unreachable and the user wants to finish anyway, they can opt out
with QUORUM_RERQ_OFF=1 or: touch .git/quorum-rerequest-off"
    fi
    bail "cannot tell whether a re-request is owed on PR #$pr_number - timeline unreadable or report not found in it (retry, or --force to re-request regardless)"
fi

if [[ "$force" == "0" && "$head_moved" == "false" ]]; then
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
# The removal is what makes the following add produce a fresh review_requested
# event. Every pending-request check below asks GitHub FRESH - the pr_json
# snapshot from the top of the script is stale by now and a stale answer here
# turns the add into a silent no-op that would be reported as success.
pending_now() {
    # Prints true / false, or fails (non-zero) when GitHub cannot be asked.
    gh pr view "$pr_number" --json reviewRequests 2>/dev/null |
        jq -r --arg t "$target" '
            [.reviewRequests[]? | (.login // .slug // "")]
            | any(. == $t or . == ($t | split("/") | last))' 2>/dev/null
    return "${PIPESTATUS[0]}"
}

if ! rm_out=$(gh pr edit "$pr_number" --remove-reviewer "$target" 2>&1); then
    # A failed removal is only tolerable when the target is verifiably not
    # requested (removing an absent reviewer errors by design). Decide on a
    # fresh lookup; if that lookup fails too, the state is unknown - abort.
    still_pending=$(pending_now) || still_pending="unknown"
    if [[ "$still_pending" != "false" ]]; then
        echo "Failed to remove the pending request for $target on PR #$pr_number:" >&2
        echo "  $rm_out" >&2
        echo "Pending state now: $still_pending. Without a confirmed removal the add" >&2
        echo "would be a silent no-op. Nothing was changed." >&2
        exit 1
    fi
fi

added=0
add_out=""
for _ in 1 2 3; do
    if add_out=$(gh pr edit "$pr_number" --add-reviewer "$target" 2>&1); then
        added=1
        break
    fi
    sleep 2
done
if [[ "$added" == "0" ]]; then
    echo "Failed to re-request review from $target on PR #$pr_number:" >&2
    echo "  $add_out" >&2
    echo "The pending request may already have been removed. Check and repair with:" >&2
    echo "  gh pr view $pr_number --json reviewRequests" >&2
    echo "  gh pr edit $pr_number --add-reviewer $target" >&2
    exit 1
fi

# Trust, but verify: the add must verifiably have landed as a pending request.
# An unanswerable verification is a failure, not a pass - the cache stays
# untouched so the next hook run re-checks.
verify=$(pending_now) || verify="unknown"
if [[ "$verify" != "true" ]]; then
    echo "gh reported success, but the pending request for $target could not be" >&2
    echo "confirmed (state: $verify). Verify manually:" >&2
    echo "  gh pr view $pr_number --json reviewRequests" >&2
    exit 1
fi

echo "Re-requested review from $target on PR #$pr_number."
rm -f "$cache_file" 2>/dev/null || true
