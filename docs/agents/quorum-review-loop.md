# quorum Review Loop

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

Do not increase test deadlines, add retries, remove parallel tests or weaken
assertions to make an overloaded gate pass. Follow the
[test-contract rule](../../.claude/rules/no-test-modifications.md).
