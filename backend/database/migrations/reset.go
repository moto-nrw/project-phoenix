package migrations

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database"
)

// ResetDatabase drops all schemas and recreates them to start fresh
func ResetDatabase() error {
	db, err := database.DBConn()
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Resetting database: Dropping and recreating all schemas...")

	// First disable all triggers
	_, err = db.ExecContext(context.Background(), "SET session_replication_role = 'replica'")
	if err != nil {
		return fmt.Errorf("failed to disable triggers: %w", err)
	}

	// List of schemas to drop and recreate
	schemas := []string{
		"auth",
		"users",
		"education",
		"schedule",
		"activities",
		"facilities",
		"iot",
		"feedback",
		"config",
		"meta",
	}

	// 1. Drop all schemas with CASCADE to remove all objects inside them
	for _, schema := range schemas {
		fmt.Printf("Dropping schema %s...\n", schema)
		_, err := db.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
		if err != nil {
			return fmt.Errorf("failed to drop schema %s: %w", schema, err)
		}
	}

	// First drop the bun migration tables in the public schema
	_, err = db.ExecContext(context.Background(), `
		DROP TABLE IF EXISTS bun_migrations CASCADE;
		DROP TABLE IF EXISTS bun_migration_locks CASCADE;
	`)
	if err != nil {
		fmt.Printf("Warning: Failed to drop bun migration tables: %v\n", err)
		// Continue anyway
	}

	// 2. Look for and drop all custom types
	rows, err := db.QueryContext(context.Background(), `
		SELECT typname FROM pg_type t 
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace 
		WHERE n.nspname = 'public'
	`)
	if err != nil {
		fmt.Printf("Warning: Failed to query custom types: %v\n", err)
	} else {
		defer rows.Close()

		// Process each custom type
		for rows.Next() {
			var typeName string
			if err := rows.Scan(&typeName); err != nil {
				fmt.Printf("Warning: Failed to scan type name: %v\n", err)
				continue
			}

			// Skip standard PostgreSQL types
			if typeName == "bool" || typeName == "int" || typeName == "text" {
				continue
			}

			fmt.Printf("Dropping custom type %s...\n", typeName)
			_, err := db.ExecContext(context.Background(), fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", typeName))
			if err != nil {
				fmt.Printf("Warning: Failed to drop type %s: %v\n", typeName, err)
			}
		}
	}

	// 3. Drop specific known types that might be in any schema
	_, err = db.ExecContext(context.Background(), `
		DROP TYPE IF EXISTS occupancy_status CASCADE;
		DROP TYPE IF EXISTS device_status CASCADE;
		
		-- Drop extensions
		DROP EXTENSION IF EXISTS "uuid-ossp";
	`)
	if err != nil {
		fmt.Printf("Warning: Failed to drop specific types and extensions: %v\n", err)
		// Continue anyway, this is not critical
	}

	// 3. Recreate the schemas (this will be skipped when migrations run)
	for _, schema := range schemas {
		fmt.Printf("Recreating schema %s...\n", schema)
		_, err := db.ExecContext(context.Background(), fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
		if err != nil {
			return fmt.Errorf("failed to create schema %s: %w", schema, err)
		}
	}

	// We already dropped the bun_migrations tables earlier

	// Re-enable triggers
	_, err = db.ExecContext(context.Background(), "SET session_replication_role = 'origin'")
	if err != nil {
		return fmt.Errorf("failed to re-enable triggers: %w", err)
	}

	fmt.Println("Database reset complete - all schemas dropped and recreated")
	return nil
}
