# Fixing Review Findings: Always Re-request the Reviewer

**RULE: After pushing fixes for a `quorum` review, the reviewer MUST be re-requested — removed and added again. A fix round that ends without that re-request is not finished.**

```bash
scripts/quorum-rerequest.sh        # does the remove/add round trip for this branch's PR
scripts/quorum-rerequest.sh --check   # exit 1 when a re-request is owed
```

## Why removing and re-adding, not just adding

`quorum` reviews on the **review request**, not on new code. Its own README:

> "The trigger is the review request, not the code. One request is one review, so pushing more commits does not trigger another one; re-request the review to get a fresh one."

`quorum` posts its report as a plain PR **comment**, not as a formal GitHub review. GitHub therefore never clears the pending request, the reviewer stays in `reviewRequests` — and `gh pr edit --add-reviewer` on someone already listed is a silent no-op that notifies nobody. Only removing and re-adding produces a fresh `review_requested` event, which is what the next review round hangs on.

Without it the fixes sit on the PR unreviewed, and nothing tells you: the PR looks reviewed, the findings look handled, and no one is waiting on anything.

## When it applies

- Findings from a quorum report were fixed and pushed → re-request.
- The PR has no quorum review yet, or nothing was pushed since the last one → nothing owed; the script exits without acting.
- The quorum report came from the PR author's own account → GitHub refuses that review request; `quorum babysit` re-triggers itself there, so nothing is owed.

A quorum report is recognised by all five of its headings (`## Summary`, `## Blockers`, `## Critical`, `## Suggestions`, `## Questions`); two of them would also match an ordinary comment. Pin the account explicitly with `git config quorum.reviewer <login>` when in doubt.

**Only the quorum account is re-requested.** Human reviewers and teams on the PR are never removed and re-added, that would reset a review someone is in the middle of. For the same reason a `review_requested` event only counts as a fresh round when it aimed at the quorum account.

## Enforcement

The `Stop` hook `scripts/quorum-rerequest.sh --stop-hook` (wired in `.claude/settings.json` and `.codex/hooks.json`) blocks the end of a turn while a re-request is owed. It only fires when the work looks finished (no uncommitted changes, nothing unpushed) and never twice in a row.

If a re-request is deliberately unwanted, say so — do not work around the hook. Per clone it can be switched off with `touch .git/quorum-rerequest-off`.
