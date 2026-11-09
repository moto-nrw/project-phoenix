# quorum Review Loop

## Devbox bootstrap and temporary profiles

Before its first Devbox call, `.envrc` sources
[`scripts/devbox-tmpdir.sh`](../../scripts/devbox-tmpdir.sh). With no non-empty
caller `TMPDIR`, this creates private (0700) `moto-nix-tmp` inside the
repository's Git common directory (`.git` in the main checkout). All worktrees
share this temp namespace, so new reviews also sweep older review leftovers. Keep checkouts on disk; Linux tmpfs/ramfs checkouts fail with an
instruction to supply a disk-backed `TMPDIR`. macOS keeps its existing system
`TMPDIR`; an unset value uses the same repo-local fallback. Explicit values,
including `/tmp`, are respected. Local dotenv overrides are loaded afterwards;
set a custom bootstrap `TMPDIR` in the invoking environment, before direnv.
No shell profile or machine configuration is changed.

This protects against [NixOS/nix#10404](https://github.com/NixOS/nix/issues/10404):
a fresh Quorum worktree needs its own Devbox profile even when packages already
exist in the shared Nix store. Keep automatic installation enabled. After
bootstrap, Node from the installed profile runs [`scripts/cleanup-devbox-tmp.mjs`](../../scripts/cleanup-devbox-tmp.mjs).
It only sweeps the repository's private default temp namespace, never
`/tmp`, a caller's custom `TMPDIR`, symlinked directories or foreign-owned trees.
Only `nix-<pid>-<number>` directories with a dead creator PID are candidates.
Linux `/proc` or macOS `lsof` checks open descriptors, working directories and
mapped files. Known references always retain a tree.

When reference inspection is incomplete, trees are retained. The bounded Linux
exception requires `/proc/<pid>` to be absent and **every entry** in the tree to
have unchanged mtime and ctime for over 60 minutes. This covers hosts where
`ptrace_scope` hides unrelated processes. It is a grace-period policy, not proof
that inaccessible processes hold no references. Cleanup runs on the next
successful bootstrap (or explicit `node scripts/cleanup-devbox-tmp.mjs` invocation), including after an earlier
error or interruption; it is not a background service. Concurrent sweeps tolerate
already removed trees. Persistent Nix-store and Devbox caches remain intact.

`--no-direnv` bypasses Quorum's bootstrap entirely. Use it only with a separately
prepared worktree profile and complete toolchain/environment. Changing only the
local test command does not protect Quorum's earlier direnv prewarm. Quorum
reads repo configuration from its base branch, so validate this bootstrap on
the PR head explicitly before it merges.

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
