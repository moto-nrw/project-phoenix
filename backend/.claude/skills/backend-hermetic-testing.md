# Backend hermetic testing

Before writing or diagnosing database integration tests, read
[backend test fixtures and lifecycle](../../../docs/agents/backend-testing.md).
It owns the fixture APIs, per-test tenants, parallel tests, shared-pool rules,
and leftover gate. This reference adds commands, not another fixture tutorial.

## Start from current examples

- [Package lifecycle](../../modules/studentpresence/internal/application/presence/main_test.go)
- [Parallel behavior tests](../../modules/studentpresence/internal/application/presence/broadcast_test.go)
- [Fixture helpers](../../test/fixtures.go)

Follow those examples for tenant ownership and `t.Parallel()`. Fixture setup
registers its own cleanup; the package owns the shared database pool. For an
error-path test that must close its database, use the closable setup described
in the canonical guide.

## Running tests

Run commands from `backend/` through the pinned toolchain. The test lifecycle
starts Postgres, prepares migration-hash templates, and creates package clones
itself. Keep Docker available; app servers and a manual schema reset are not
test prerequisites.

Use the affected package and test during diagnosis. The presence package and
test below are examples; select the package and existing test for your change.

```bash
# Focused behavior check with a fresh result, retaining compiled-package caches
../scripts/run-go-toolchain.sh go test ./modules/studentpresence/internal/application/presence \
  -run '^TestBroadcast_EndVisitRunsAfterCommit$' -count=1 -v

# Repeated execution in one binary to expose leaked test state
../scripts/run-go-toolchain.sh go test ./modules/studentpresence/internal/application/presence \
  -run '^TestBroadcast_EndVisitRunsAfterCommit$' -count=2 -v

# All tests in the affected package
../scripts/run-go-toolchain.sh go test ./modules/studentpresence/internal/application/presence

# Full suite with immediate clone sweep
../scripts/run-go-toolchain.sh ../scripts/test-backend.sh

# Full fresh-result run when the task specifically requires one
../scripts/run-go-toolchain.sh go test -count=1 ./...

# Full race check when required; Go's race detector needs CGO
CGO_ENABLED=1 ../scripts/run-go-toolchain.sh go test -race ./...

# Fixture contract gate
../scripts/run-go-toolchain.sh go test ./test/ -run '^TestHermeticTestPatterns$' -v
```

Scoped checks give feedback while editing; they do not replace the required
before-push checks or a requested full-suite/race run. From the repository root,
run `scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development`
without `--fast` before pushing code.

## Diagnosing state leaks

1. Reproduce with a focused `-count=1` or repeated `-count=2` run. These bypass
   the result cache without deleting compilation caches.
2. Check tenant ownership, fixture setup, and shared-state mutations against
   the canonical guide. Each independent subtest must own its state.
3. Use the guide's per-test leftover diagnosis to name the leaking test; add
   `t.Logf()` where the failing path needs evidence.
4. Preserve assertions and failure coverage. Apply
   [test contracts](../../../.claude/rules/no-test-modifications.md) before
   changing expectations.
