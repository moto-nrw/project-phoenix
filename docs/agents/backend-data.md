# Backend persistence and migrations

Read for BUN queries, database role/configuration changes, or schema migrations.
Paths in code spans start at `backend/` unless prefixed with `backend/`,
`.claude/`, or `docs/`. Commands run from the repository root unless stated.

## Database Configuration

DSN resolution is **fail-fast** (`database/database_config.go`) — there are no localhost fallbacks:

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
Open tenant transactions through `tenant.TransactionRunner` (or the composition root's `tenant.UnitOfWork`). The runtime propagates the active transaction through context; repositories pick it up through `base.GetDB(ctx, db)`. For query filters and the generic repository API (`Repository[T]`, `base.Filter` with `Equal`/`ILike`/`In`/pagination), see `.claude/rules/backend-conventions.md` Rule 2 — don't invent per-field finder methods.

### Soft Delete
`users.Person`, `users.Staff`, and `users.Teacher` carry `deleted_at` with bun's `soft_delete` tag: normal queries auto-filter soft-deleted rows. Staff deletion runs an offboarding service (not a bare delete). Keep this in mind when counting rows or writing raw SQL against these tables.

## Migration System

One file per migration, named with the **zero-padded numeric version prefix** — `001015124_my_feature.go` for version `1.15.124` (the collision scanner in `00_migrations.go` only recognizes `000`/`001`-prefixed filenames; never use the dotted version in the filename):

Use [migration registration](../../backend/database/migrations/00_migrations.go)
and an existing migration in `backend/database/migrations/` as the pattern.
Choose an unused version and explicit dependencies; do not copy example versions.

`MigrationRegistry` is a `SafeMigrationMap` — duplicate versions **panic at init**, so the binary won't start on a collision. `go run main.go migrate validate` checks the dependency graph in-memory. Migrations use a superuser connection, which bypasses RLS even with `FORCE ROW LEVEL SECURITY`. Do not add `ALTER TABLE ... DISABLE/ENABLE ROW LEVEL SECURITY`; HTTP requests stay on the least-privilege role.
