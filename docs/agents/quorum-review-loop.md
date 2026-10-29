# Fixing Review Findings: Always Re-request the Reviewer

## Local test gate and CI

[`.quorum/testcmd`](../../.quorum/testcmd) sets the local gate to
`scripts/run-go-toolchain.sh scripts/test-changed.sh --fast`. The wrapper pins
the Devbox Go toolchain even when Quorum runs outside a Devbox shell.
For backend changes, `--fast` selects changed packages and direct importers,
not transitive importers or the automatically appended `./test` ratchets.
Changes to `go.mod` or `go.sum` still select all backend packages. Frontend
selection is unchanged. CI keeps the full affected closure, including test
importers and ratchets; the manual full run remains available:

```bash
scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development
```

Quorum resolves `--test-cmd` first, then the user-local
`~/.config/quorum/testcmd/<owner>/<repo>`, then the repo file **from the base
branch**, not the PR head. Check for a local override before relying on this
gate. To verify a gate change before it merges, pass the command explicitly:

```bash
quorum babysit <pr-number> --test-cmd 'scripts/run-go-toolchain.sh scripts/test-changed.sh --fast'
```

The existing package-worker budget and `-parallel 8` remain unchanged.
Concurrent sessions add their load against the same test server; the run-ID
lock prevents clone-name collisions, not concurrent DB load. A smaller gate
does not prove that DB timeouts are fixed. For a bounded before/after load
check, record concurrent runs, selected packages, flags, server port, elapsed
time and failures. Separate server I/O, SQL waits and pool waits before adding
throttling. Follow the [test-server lifecycle](operations.md#test-database)
and preserve other sessions and persistent data during cleanup.

Do not increase test deadlines, add retries, remove parallel tests or weaken
assertions to make an overloaded gate pass. Follow the
[test-contract rule](../../.claude/rules/no-test-modifications.md).

## Re-request after pushed fixes

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

The quorum account is declared, never guessed: `git config quorum.reviewer <login>` per clone, otherwise the tracked `.github/quorum-reviewer`. Without a declared account no review request is touched at all. A quorum report is a comment from that account carrying all five headings (`## Summary`, `## Blockers`, `## Critical`, `## Suggestions`, `## Questions`).

**Only the quorum account is re-requested.** Human reviewers and teams on the PR are never removed and re-added, that would reset a review someone is in the middle of. For the same reason a `review_requested` event only counts as a fresh round when it aimed at the quorum account **and** came after the last push. Re-requesting and then pushing more commits leaves those commits unreviewed, so the check stays open.

## Enforcement

The `Stop` hook `scripts/quorum-rerequest.sh --stop-hook` (wired in `.claude/settings.json` and `.codex/hooks.json`) blocks the end of a turn while a re-request is owed. It only fires when the work looks finished (no uncommitted changes, nothing unpushed) and never twice in a row.

Codex loads `.codex/hooks.json` only behind its experimental hooks feature flag. The repo config (`.codex/config.toml`) switches it on (`[features] hooks = true`, plus the pre-rename alias `codex_hooks` for older CLI versions); if the Stop hook still does not fire there, set the same flag in `~/.codex/config.toml`.

If a re-request is deliberately unwanted, say so — do not work around the hook. Per clone it can be switched off with `touch .git/quorum-rerequest-off`.
