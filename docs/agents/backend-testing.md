# Backend test fixtures and lifecycle

Read before adding, changing, or diagnosing backend tests. Paths in code spans
start at the repository root unless stated; commands below run from `backend/`.
Current rules here supersede historical test-lifecycle examples in ADR 0004.

## Hermetic fixtures

All backend tests use real database fixtures, never hardcoded IDs. The CI gate `TestHermeticTestPatterns` (`backend/test/hermetic_verification_test.go`) fails on `int64(1)`-style IDs; mock-based test files must be added to its `skipPatterns` allowlist.

Use the real fixture setup and behavior tests as examples:
[package lifecycle](../../backend/services/active/main_test.go),
[parallel behavior tests](../../backend/services/active/broadcast_test.go),
and [fixture helpers](../../backend/test/fixtures.go).
Copy their tenant/parallel setup, not literal IDs or legacy factory composition.

- **One pool per package (#2419)**: `SetupTestDB` returns the same `*bun.DB` for every test in the binary. Never `db.Close()` it (gate: `no_shared_pool_close`). Tests that close their DB on purpose to force error paths use `testpkg.SetupClosableTestDB(t)`.
- **No explicit `Cleanup*` calls**: `cleanupCallBaseline` is empty and the
  AST-based gate rejects fixture cleanup through any import alias. The package
  clone owns tenant rows. Tenantless fixture builders register their lifecycle
  internally; schema-migration tests use `testpkg.OwnTenantRows`; subtests that
  need isolated state use `testpkg.OwnTenant` / `testpkg.OwnCtx`. If a missing
  row is the test arrangement, use a production delete operation or reserve an
  unused sequence ID through a fixture helper.
- **Every test owns its tenant (#2419)**: a package opts in once, from `TestMain`, with `testpkg.PerTestTenants()`. From then on each top-level test gets its own tenant, every `CreateTest*` fixture it creates lands there, and JWT claims minted through `api/testutil` follow it — so no fixture call and no claims helper needs a tenant argument. Inside a test, `testpkg.Ctx(t)` is the context (the replacement for `TenantContext(1)`) and `testpkg.Tenant(t)` the ID. Subtests share their parent's tenant — which is right when the parent builds the fixtures they read, and wrong for a table of subtests that each create the same kind of row and then assert something tenant-wide about it. Those call `testpkg.OwnTenant(t)` / `testpkg.OwnCtx(t)` as their first line and get a tenant of their own. One edge to know: the rebase happens when claims are *used* (`MintTestJWT`, `WithClaims`), so reading `claims.TenantID` straight off the struct still yields the bootstrap value — inside a test, take the tenant from `testpkg.Tenant(t)`, never from the claims you just built. Two gates hold the line: `db_packages_opt_into_per_test_tenants` fails any package that opens the test database without opting in, and `bootstrap_tenant_ratchet` counts every remaining spelling (`TenantContext(1)`, `WithTenantID(ctx, 1)`, `TenantID: 1`, `SetTenantID(1)`, `…ForTenant(…, 1, …)`, and literal `tenant_id` filters in raw SQL) per package, shrink-only.
- **Every top-level test is parallel (#2851)**: start it with `t.Parallel()`.
  The `tests_run_in_parallel` gate has an empty baseline and rejects every
  exception. Inject process configuration and output; use per-test database
  clones for schema changes, sweeps, query measurements, and lock tests.
- **Concurrency is pinned, not inherited**: `scripts/test-backend.sh` runs
  `-p 10 -parallel 8` (local postgres-test has `max_connections=300`);
  post-merge CI runs `-p 6 -parallel 8`, changed-only PRs `-p 4 -parallel 8`
  (CI's service container keeps the stock 100 connections). `-parallel` stays
  at 8 everywhere on purpose: `-test.parallel` is part of the Go test cache
  key and sizes the per-binary pool.
  The pool per binary is derived from `-test.parallel` plus
  headroom, because a test holding a tenant transaction that opens a second one
  needs two connections at once — without headroom those tests deadlock and
  every one of them fails on its own 5s deadline, which looks nothing like a
  pool problem.
- **Leftovers are a gate, not a report (#2419)**: every test binary compares its
  clone against the start state it recorded for itself and fails the PACKAGE
  when rows are left in SHARED state — rows outside the tenants its own tests
  created. Rows in a test's own tenant are not leftovers. The gate runs from
  `TestMain` via `testpkg.Run(m)` (gate: `db_packages_run_the_leftover_gate`),
  so `../scripts/run-go-toolchain.sh go test ./...` is gated exactly like a
  full wrapper run; it costs the package one query at exit (~30-70ms measured).
  `PHX_TEST_LEFTOVERS=1 ../scripts/run-go-toolchain.sh go test -v` also prints
  the pairs `testdb.LeftoverAllowlist` still tolerates;
  `PHX_TEST_LEFTOVERS=test ../scripts/run-go-toolchain.sh go test -parallel 1 ./pkg` checks after every test
  and names the culprit instead of the package.
- **Parallel + bootstrap tenant is the combination to avoid.** Tests sharing tenant 1 may run in parallel only while every assertion is scoped to IDs the test created; the moment one asserts something tenant-wide (a count, a "list all"), it becomes order-dependent. The remaining files where both meet are frozen by the `parallel_on_bootstrap_tenant_ratchet` gate — do not add a new one, opt the package into per-test tenants instead.
- The fixture catalog lives in `backend/test/fixtures.go` (`CreateTest*` helpers, including `*ForTenant` variants for multi-tenant tests and auth chains like `CreateTestTeacherWithAccount`). Search it before writing a new fixture.
- Tests hitting the DB go in external test packages (`package active_test`); pure model tests stay internal.
- From `backend/`, run the gate before pushing: `../scripts/run-go-toolchain.sh go test ./test/ -run TestHermeticTestPatterns -v`
- Preserve test contracts; use `.claude/rules/no-test-modifications.md` to distinguish regressions from authorized expectation changes.

## Test commands

```bash
# Testing — self-initializing lifecycle (ADR 0004): SetupTestDB starts the
# postgres-test container if needed, builds the template for this branch's
# migrations hash (phoenix_test_<hash>, so parallel worktrees never share
# one), and gives each package binary a run-stamped clone.
../scripts/run-go-toolchain.sh ../scripts/test-backend.sh  # Full suite via gotestsum + immediate clone sweep (preferred full run)
../scripts/run-go-toolchain.sh go test ./...                # All tests (works standalone; each binary drops its own clone at exit)
PHX_TEST_LEFTOVERS=1 ../scripts/run-go-toolchain.sh go test -v ./services/active  # Also print tolerated leftovers
PHX_TEST_LEFTOVERS=test ../scripts/run-go-toolchain.sh go test -parallel 1 ./services/active  # Name the leaking test
PHX_TEST_KEEP_CLONE=1 ../scripts/run-go-toolchain.sh go test ./services/active  # Keep the clone for a post-mortem
../scripts/run-go-toolchain.sh go run ./internal/testdb/cmd/sweep  # Drop this/dead runs' clones manually
../scripts/run-go-toolchain.sh go test -short ./...             # Fast inner loop; NEVER in CI — guts coverage.
../scripts/run-go-toolchain.sh go test ./services/active/... -v # Specific package
../scripts/run-go-toolchain.sh go test -race ./...              # Race detection
../scripts/run-go-toolchain.sh go test ./api/auth -run TestLogin # Specific test
```

The Go result cache can replay an earlier green result after the calendar day
changes. Add `-count=1` for date- or clock-dependent diagnosis.
