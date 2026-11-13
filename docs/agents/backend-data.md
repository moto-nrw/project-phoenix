# Backend persistence and migrations

Read for BUN queries, database role/configuration changes, or schema migrations.
Paths in code spans start at `backend/` unless prefixed with `backend/`,
`.claude/`, or `docs/`. Commands run from the repository root unless stated.

## Database Configuration

DSN resolution is **fail-fast** (`modules/transactionruntime/internal/adapters/database/database_config.go`) — there are no localhost fallbacks:

1. `APP_ENV=test` — requires `TEST_DB_DSN` and never falls through to `DB_DSN` (test DB on port 5433)
2. Every other environment — requires `DB_DSN`; CLI commands connect as the `postgres` **superuser** (the seeder is API-based and opens no DB connection itself)
3. Missing config exits with an error

The HTTP server (`serve`) connects as the least-privilege **`phoenix_auth`** role instead (NOINHERIT; can `SET ROLE` to `phoenix_tenant`/`phoenix_admin` per request). `PHOENIX_AUTH_PASSWORD` is mandatory — the server refuses to start without it. This split is what makes RLS enforcement real: request queries run under the tenant role, never as superuser.

## Critical BUN ORM Patterns

### Schema-Qualified Tables (MUST USE QUOTES!)
```go
// CORRECT - Quotes around alias prevent "column not found" errors
ModelTableExpr(`users.teachers AS "teacher"`)

// WRONG - Missing quotes causes BUN mapping failures
ModelTableExpr(`users.teachers AS teacher`)
```

### Loading Nested Relationships
```go
// For Teacher → Staff → Person relationships
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    // Explicit column mapping required for each table
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".id AS "staff__id"`).
    ColumnExpr(`"person".* AS "person__*"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
    Where(`"teacher".id = ?`, id).
    Scan(ctx)
```

### Transactions and Filters
Open tenant transactions through `tenant.TransactionRunner` (or the composition
root's `tenant.UnitOfWork`). The runtime propagates the active transaction through
context (`tenant.TransactionFromContext` in `tenant/runtime.go`). Owner-private
adapters resolve that transaction through their existing bindings, not a shared
repository base. For example, People Directory's `NewDatabase` in
`modules/peopledirectory/internal/adapters/postgres/native_compose_bindings_callback_database.go`
passes the transaction and validated tenant to `ResolveDatabase` in the same
package's `scope.go`. Missing or unsupported transactions always fail. Missing school scope is allowed
only when the caller explicitly selected an administrative transaction.
Its `StudentStore.InsertRecord` and `UpdateRecord` (`student_write.go` in that
package) require a positive tenant, set the inserted row's tenant from the scope,
and constrain updates by tenant ID.

Reuse the owner's existing typed filters and query helpers before adding a
query method, as required by `.claude/rules/backend-conventions.md` Rule 2.
Keep SQL and driver types inside policy-authorized owner adapters; cross-owner
callers use public capabilities bound to consumer-owned ports.

### Soft Delete
`users.Person`, `users.Staff`, and `users.Teacher` carry `deleted_at` with bun's `soft_delete` tag: normal queries auto-filter soft-deleted rows. Staff deletion runs an offboarding service (not a bare delete). Keep this in mind when counting rows or writing raw SQL against these tables. `users.staff_documents` and `users.staff_qualifications` carry a plain `deleted_at` column instead: their workforce store filters `deleted_at IS NULL` explicitly, and the tenant role has no DELETE grant on them.

## Migration System

One file per migration, named after its **UTC timestamp**:
`20261113142530_my_feature.go`, from `date -u +%Y%m%d%H%M%S`
([ADR 0223](../adr/0223-migrationen-heissen-nach-zeitstempeln-funktionen-haben-eine-definition.md)).
Its `Version` is that prefix; `Register` panics on any other. The migrations up
to `001015963` keep their packed version (`1.15.963`) and are never renamed.

Use [migration registration](../../backend/database/migrations/00_migrations.go)
and an existing migration in `backend/database/migrations/` as the pattern.
`DependsOn` names the migrations this one needs to have run, not the newest one.

- **No number to agree on.** Two branches add migrations without renumbering.
  `scripts/check-migration-versions.mjs` rejects a name that is no timestamp
  or is already taken.
- **Changing a function.** The current definition of a function lives in one
  `repeatable_<schema>_<name>.go` file (`RegisterRepeatable`, rules in
  [`repeatable.go`](../../backend/database/migrations/repeatable.go)). Every
  migrate run installs all of them after the numbered migrations. Change the
  function there. A migration may create a new function; the check rejects one
  that defines an existing function again. To change a function that has no
  repeatable file yet, add the file with the full new definition.
- **Late migrations.** A migration that sorts below the newest one of the base
  reaches deployed databases after it; bun applies whatever is missing.
  `TestLateMigrationsYieldTheFreshSchema` replays that order and fails when
  the schema differs from a fresh database or the migrations do not run. The
  fix is a rename to a current timestamp. CI and `scripts/test-changed.sh`
  name the late migrations in `PHX_LATE_MIGRATIONS`; to run it by hand:
  `PHX_LATE_MIGRATIONS=$(node ../scripts/check-migration-versions.mjs --late) ../scripts/run-go-toolchain.sh go test -tags=testhooks -count=1 ./database/migrations -run TestLateMigrationsYieldTheFreshSchema`.
  The check compares schemas, not rows: a backfill that needs another
  migration's data names it in `DependsOn`.

The runner logs one `slog` line per applied migration — version, description and `duration_ms`, resolved from `MigrationRegistry` (#3300). A migration does not print its own progress; the `Description` it registers is what the log shows, and output that carries content (row counts, repairs) goes through `migrationLog()` in `database/migrations/migration_logging.go`, which scopes the process logger to `component=migrations`. A failed deferred rollback goes to `logRollbackFailure(ctx, err)` in the same file.

Two tests in `database/migrations/migration_output_contract_test.go` hold that contract: `TestMigrationsDoNotPrintTheirOwnProgress` rejects `fmt.Print*`/`log.Print*` in a migration file, and `TestMigrationsDoNotRestateTheRunnersLine` rejects a log line that announces the migration ("migration starting", "migration 1.15.x: …") or carries a `migration`/`version` field the runner's own line already has.

`MigrationRegistry` is a `SafeMigrationMap` — duplicate versions **panic at init**, so the binary won't start on a collision. `go run main.go migrate validate` checks the dependency graph in-memory. Migrations use a superuser connection, which bypasses RLS even with `FORCE ROW LEVEL SECURITY`. Do not add `ALTER TABLE ... DISABLE/ENABLE ROW LEVEL SECURITY`; HTTP requests stay on the least-privilege role.
