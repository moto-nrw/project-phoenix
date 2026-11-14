package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/uptrace/bun"
)

// publicSchema is the one schema a reset sweeps for leftover types. Everything
// else is dropped wholesale in step 1, so only public survives to be swept.
const publicSchema = "public"

// droppablePublicTypesQuery lists the types in the public schema that a reset may
// drop. It excludes the groups that PostgreSQL either refuses to drop or drops
// on its own:
//
//   - types owned by an extension (pg_depend.deptype = 'e'), such as the
//     gbtreekey* types of btree_gist or the row type of pg_stat_statements.
//     DROP TYPE on those fails with SQLSTATE 2BP01.
//   - types another object owns internally (deptype = 'i'), which are dropped
//     with their owner. Today that is only array and row types, both also
//     matched below; the predicate keeps a future range type's multirange from
//     turning a reset into an error.
//   - array types, which PostgreSQL drops together with their element type.
//   - row types of tables, views and other relations, which are dropped with
//     the relation itself. Standalone composite types (relkind 'c') stay in the
//     list because only DROP TYPE removes them.
const droppablePublicTypesQuery = `
	SELECT t.typname
	FROM pg_type t
	JOIN pg_namespace n ON n.oid = t.typnamespace
	WHERE n.nspname = 'public'
	  AND (t.typrelid = 0 OR (SELECT c.relkind FROM pg_class c WHERE c.oid = t.typrelid) = 'c')
	  AND NOT EXISTS (
	    SELECT 1 FROM pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid
	  )
	  AND NOT EXISTS (
	    SELECT 1 FROM pg_depend d
	    WHERE d.classid = 'pg_type'::regclass AND d.objid = t.oid AND d.deptype IN ('e', 'i')
	  )
	ORDER BY t.typname
`

// resetSchemas lists the schemas to drop and recreate. NOTE: keep in sync with all CREATE SCHEMA calls across migrations,
// INCLUDING a schema a later migration drops again. A reset replays the
// full history, so 1.9.1 recreates suggestions long before 1.15.315 drops
// it. Leaving a stale copy in place makes the intervening
// CREATE TABLE IF NOT EXISTS a no-op and the migration after it fails on a
// missing column (#2326).
var resetSchemas = []string{
	"auth",
	"users",
	"education",
	"schedule",
	"activities",
	"facilities",
	"iot",
	"feedback",
	"active",
	"config",
	"meta",
	"audit",          // created by migration 1.3.7
	"suggestions",    // created by 1.9.1, dropped again by 1.15.315
	"platform",       // created by migration 1.11.1
	"enrollment",     // created by migration 1.15.59
	"calendar",       // created by migration 1.15.173
	"display",        // created by migration 1.15.175
	"documents",      // created by migration 1.15.332
	"parent_billing", // created by migration 1.15.645
	"care_contract",  // created by migration 1.15.770
	"workforce",      // created by migration 20261114153024
}

// resetDatabase drops all schemas and recreates them to start fresh.
//
// The reset only issues DDL. DROP does not fire row triggers, and the schema
// carries no event triggers, so there is nothing for session_replication_role
// to disable here. Setting it over the pool was also unsound: the value applies
// to a single pooled connection, so the reset to 'origin' could land on a
// different connection than the switch to 'replica' and leave a connection in
// replica mode for the migrations that run next — with foreign keys unenforced.
func resetDatabase(ctx context.Context, db *bun.DB, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	logger = migrationLogger(logger)
	logger.InfoContext(ctx, "dropping and recreating all schemas")

	// 1. Drop all schemas with CASCADE to remove all objects inside them
	for _, schema := range resetSchemas {
		logger.DebugContext(ctx, "dropping schema", "schema", schema)
		_, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS ? CASCADE", bun.Ident(schema))
		if err != nil {
			return fmt.Errorf("failed to drop schema %s: %w", schema, err)
		}
	}

	// First drop the bun migration tables in the public schema
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS bun_migrations CASCADE;
		DROP TABLE IF EXISTS bun_migration_locks CASCADE;
	`)
	if err != nil {
		// Continue anyway: a reset that cannot drop the bookkeeping tables still
		// recreates the schemas, and migrator.Init recreates these two.
		logger.WarnContext(ctx, "failed to drop bun migration tables", "error", err)
	}

	// 2. Drop what migrations left in the public schema: functions and
	// custom types (dropPublicLeftovers).
	if err := dropPublicLeftovers(ctx, db, logger); err != nil {
		return err
	}

	// 3. Drop the extensions. The named types this step used to drop as well
	// (occupancy_status, device_status) are created unqualified, so they land
	// in public and step 2 has already removed them.
	_, err = db.ExecContext(ctx, `DROP EXTENSION IF EXISTS "uuid-ossp"`)
	if err != nil {
		// Continue anyway, this is not critical: the migration that installs the
		// extension uses IF NOT EXISTS.
		logger.WarnContext(ctx, "failed to drop extensions", "error", err)
	}

	// 4. Recreate the schemas (this will be skipped when migrations run)
	for _, schema := range resetSchemas {
		logger.DebugContext(ctx, "recreating schema", "schema", schema)
		_, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS ?", bun.Ident(schema))
		if err != nil {
			return fmt.Errorf("failed to create schema %s: %w", schema, err)
		}
	}

	// We already dropped the bun_migrations tables earlier

	logger.InfoContext(ctx, "schemas dropped and recreated", "count", len(resetSchemas))
	return nil
}

// droppablePublicRoutinesQuery lists the functions and procedures in the
// public schema a reset may drop: every one an extension does not own
// (pg_depend.deptype = 'e'), with its identity arguments.
const droppablePublicRoutinesQuery = `
	SELECT p.proname, pg_get_function_identity_arguments(p.oid)
	FROM pg_proc p
	JOIN pg_namespace n ON n.oid = p.pronamespace
	WHERE n.nspname = 'public'
	  AND p.prokind IN ('f', 'p')
	  AND NOT EXISTS (
	    SELECT 1 FROM pg_depend d
	    WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e'
	  )
	ORDER BY p.proname, 2
`

// publicRoutine is a function or procedure in public with its identity
// arguments.
type publicRoutine struct {
	name, arguments string
}

// dropPublicLeftovers drops what migrations left in the public schema,
// which step 1 keeps.
//
// Functions first (#7009): a plain CREATE FUNCTION there, such as
// f_phone_search_key, fails on the next migration of a reset database
// otherwise. Functions of extensions stay.
//
// Then the custom types. Collect the names first so no rows stay open while
// the DROP statements run.
func dropPublicLeftovers(ctx context.Context, db *bun.DB, logger *slog.Logger) error {
	if err := dropPublicRoutines(ctx, db, logger); err != nil {
		return err
	}
	typeNames, err := droppablePublicTypes(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to query custom types: %w", err)
	}
	for _, typeName := range typeNames {
		logger.DebugContext(ctx, "dropping custom type", "type", typeName)
		// Qualify the name. The query only ever returns types in public, while
		// an unqualified DROP TYPE resolves through search_path, so under a
		// non-default search_path the two could disagree: the drop would target
		// another schema's type or, with IF EXISTS, quietly do nothing and
		// leave the public type behind for the next CREATE TYPE to collide with.
		if _, err := db.ExecContext(ctx, "DROP TYPE IF EXISTS ?.? CASCADE",
			bun.Ident(publicSchema), bun.Ident(typeName)); err != nil {
			return fmt.Errorf("failed to drop type %s.%s: %w", publicSchema, typeName, err)
		}
	}
	return nil
}

// dropPublicRoutines drops every function and procedure in public that no
// extension owns.
func dropPublicRoutines(ctx context.Context, db *bun.DB, logger *slog.Logger) error {
	routines, err := droppablePublicRoutines(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to query public functions: %w", err)
	}
	for _, routine := range routines {
		logger.DebugContext(ctx, "dropping public function", "function", routine.name)
		if _, err := db.ExecContext(ctx, routine.dropStatement()); err != nil {
			return fmt.Errorf("failed to drop function %s.%s: %w", publicSchema, routine.name, err)
		}
	}
	return nil
}

// dropStatement drops the routine. The name is quoted; the arguments come
// from the catalog (pg_get_function_identity_arguments). It carries no bind
// placeholder, which bun would read in "?(".
func (r publicRoutine) dropStatement() string {
	return fmt.Sprintf("DROP ROUTINE IF EXISTS %s.%s(%s) CASCADE", quoteIdent(publicSchema), quoteIdent(r.name), r.arguments)
}

// quoteIdent quotes a PostgreSQL identifier.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// droppablePublicRoutines returns the functions and procedures in public a
// reset may drop.
func droppablePublicRoutines(ctx context.Context, db *bun.DB) ([]publicRoutine, error) {
	rows, err := db.QueryContext(ctx, droppablePublicRoutinesQuery)
	if err != nil {
		return nil, fmt.Errorf("query droppable public functions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var routines []publicRoutine
	for rows.Next() {
		var routine publicRoutine
		if err := rows.Scan(&routine.name, &routine.arguments); err != nil {
			return nil, fmt.Errorf("scan public function: %w", err)
		}
		routines = append(routines, routine)
	}
	return routines, rows.Err()
}

// droppablePublicTypes returns the names of the types in the public schema that
// a reset may drop. See droppablePublicTypesQuery for what is filtered out.
func droppablePublicTypes(ctx context.Context, db *bun.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, droppablePublicTypesQuery)
	if err != nil {
		return nil, fmt.Errorf("query droppable public types: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var typeNames []string
	for rows.Next() {
		var typeName string
		if err := rows.Scan(&typeName); err != nil {
			return nil, fmt.Errorf("scan droppable public type: %w", err)
		}
		typeNames = append(typeNames, typeName)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate droppable public types: %w", err)
	}
	return typeNames, nil
}
