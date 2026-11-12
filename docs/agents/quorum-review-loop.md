# quorum Review Loop

## Merge policy

When implementation and merge are authorized, that authorization includes the
implementing delegate. After self-review and a successful `quorum babysit`, the
implementer merges without waiting for a separate orchestrator approval or a
mandatory third review. Keep Quorum's review/fix loop and stopping conditions;
do not cap its rounds. Request another review only for a concrete unresolved
concern or an explicit user requirement.

Run `BASE_UPDATE=conflicts quorum babysit <pr-number>`: Quorum otherwise updates
a branch whenever it is behind, even without conflicts. This setting changes
only base-update behavior; the review/fix loop stays as configured.

Before merging, fetch the PR's base and check mergeability. Fetching updates
remote references; it does not merge or rebase the base into the PR. Update the
branch only for actual conflicts (`DIRTY`) or when the PR adds or changes
migrations, not merely because it is `BEHIND` or the base has advanced.
`BASE_UPDATE=conflicts` does not cover the migration case: merge the base
yourself, then check migration numbers and order against it and against open
PRs. For stacks, follow the
[stacked-PR workflow](../../.agents/skills/stacked-pr/SKILL.md#keep-the-stack-current).
Check relevant interactions with recent base changes; investigate concrete
integration risks without routine sync commits.

The final check still applies: the PR is ready, its current head has passed
required local checks and CI, review blockers are resolved, and it is
conflict-free. A skipped draft run is not evidence of passing CI. Unknown
mergeability or blocked checks must be resolved before merging. Bind ordinary
merges to the checked head with `gh pr merge --match-head-commit <SHA>`; for
stacks, use the [asynchronous merge endpoint](../../.agents/skills/stacked-pr/SKILL.md#merge)
with its `sha` field set to that head. If the head changes, check that revision
before retrying. Branch protection remains binding. After a
conflict fix or other code change, run affected checks and the review loop again.
Confirm the merged state and report it to the orchestrator for tracking.

## Local test gate and CI

[`.quorum/testcmd`](../../.quorum/testcmd) sets the local gate to
`scripts/run-go-toolchain.sh scripts/test-changed.sh --fast`. The wrapper pins
the Devbox Go toolchain even when Quorum runs outside a Devbox shell.
For backend changes, `--fast` selects changed packages and direct importers,
plus affected source checks from `./test` using an anchored `-run` filter.
The filesystem-input inventory is in
[`scripts/backend-source-ratchets.mjs`](../../scripts/backend-source-ratchets.mjs),
including checks without `Ratchet` in their name. Selection includes committed,
staged, unstaged and untracked paths, both sides of renames and deleted inputs.
Source checks use `-count=1`; ordinary package tests retain their cache and
receive no `-run` filter. A full `./test` selection runs once, without a filter.
The fast path does not expand transitive importers or run the full `./test`
database suite for every backend edit. Runtime schema checks stay in that suite.
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
BASE_UPDATE=conflicts quorum babysit <pr-number> --test-cmd 'scripts/run-go-toolchain.sh scripts/test-changed.sh --fast'
```

With a shared admission gate such as `~/bin/agent-heavy`, wrap only the
`--test-cmd`, never `quorum babysit` itself. The loop mostly waits for reviews
and holds an outer slot for its whole lifetime; a nested gate call inherits that
slot. On 31 October 2026 one wrapped babysit loop held one of two slots 28 % of
38 sampled minutes at about one CPU second per minute, while other agents
queued up to 490 s for test runs.

The existing package-worker budget and `-parallel 8` remain unchanged.
Concurrent sessions add their load against the same test server; the run-ID
lock prevents clone-name collisions, not concurrent DB load. A smaller gate
does not prove that DB timeouts are fixed. For a bounded before/after load
check, record concurrent runs, selected packages, flags, server port, elapsed
time and failures. Separate server I/O, SQL waits and pool waits before adding
throttling. Follow the [test-server lifecycle](operations.md#test-database)
and preserve other sessions and persistent data during cleanup.

Make an overloaded gate pass by making tests cheaper: reshape, merge or thin
them while their assertions hold
([test-contract rule](../../.claude/rules/no-test-modifications.md)). Test
deadlines, retries, dropped `t.Parallel()` and weaker assertions hide the load
instead of removing it.
