package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

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

// ResetDatabase drops all schemas and recreates them to start fresh.
//
// The reset only issues DDL. DROP does not fire row triggers, and the schema
// carries no event triggers, so there is nothing for session_replication_role
// to disable here. Setting it over the pool was also unsound: the value applies
// to a single pooled connection, so the reset to 'origin' could land on a
// different connection than the switch to 'replica' and leave a connection in
// replica mode for the migrations that run next — with foreign keys unenforced.
func ResetDatabase(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	fmt.Println("Resetting database: Dropping and recreating all schemas...")

	// List of schemas to drop and recreate.
	// NOTE: keep in sync with all CREATE SCHEMA calls across migrations,
	// INCLUDING a schema a later migration drops again. A reset replays the
	// full history, so 1.9.1 recreates suggestions long before 1.15.315 drops
	// it. Leaving a stale copy in place makes the intervening
	// CREATE TABLE IF NOT EXISTS a no-op and the migration after it fails on a
	// missing column (#2326).
	schemas := []string{
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
		"audit",       // created by migration 1.3.7
		"suggestions", // created by 1.9.1, dropped again by 1.15.315
		"platform",    // created by migration 1.11.1
		"enrollment",  // created by migration 1.15.59
		"calendar",    // created by migration 1.15.173
		"display",     // created by migration 1.15.175
		"documents",   // created by migration 1.15.332
	}

	// 1. Drop all schemas with CASCADE to remove all objects inside them
	for _, schema := range schemas {
		fmt.Printf("Dropping schema %s...\n", schema)
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
		fmt.Printf("Warning: Failed to drop bun migration tables: %v\n", err)
		// Continue anyway
	}

	// 2. Drop the custom types left in the public schema. Collect the names
	// first so no rows stay open while the DROP statements run.
	typeNames, err := droppablePublicTypes(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to query custom types: %w", err)
	}
	for _, typeName := range typeNames {
		fmt.Printf("Dropping custom type %s...\n", typeName)
		_, err := db.ExecContext(ctx, "DROP TYPE IF EXISTS ? CASCADE", bun.Ident(typeName))
		if err != nil {
			return fmt.Errorf("failed to drop type %s: %w", typeName, err)
		}
	}

	// 3. Drop the extensions. The named types this step used to drop as well
	// (occupancy_status, device_status) live in public and are already gone:
	// unqualified DROP TYPE only ever resolved them through the search path,
	// which is exactly what step 2 now covers.
	_, err = db.ExecContext(ctx, `DROP EXTENSION IF EXISTS "uuid-ossp"`)
	if err != nil {
		fmt.Printf("Warning: Failed to drop extensions: %v\n", err)
		// Continue anyway, this is not critical
	}

	// 4. Recreate the schemas (this will be skipped when migrations run)
	for _, schema := range schemas {
		fmt.Printf("Recreating schema %s...\n", schema)
		_, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS ?", bun.Ident(schema))
		if err != nil {
			return fmt.Errorf("failed to create schema %s: %w", schema, err)
		}
	}

	// We already dropped the bun_migrations tables earlier

	fmt.Println("Database reset complete - all schemas dropped and recreated")
	return nil
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
