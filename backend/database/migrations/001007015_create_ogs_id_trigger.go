package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	CreateOgsIdTriggerVersion     = "1.8.5"
	CreateOgsIdTriggerDescription = "Create trigger to auto-populate ogs_id from tenant context"
)

// Tables that need the trigger (same as RLS tables)
var triggerTables = []string{
	"users.persons",
	"users.students",
	"users.staff",
	"users.teachers",
	"education.groups",
	"facilities.rooms",
	"iot.devices",
	"active.visits",
	"active.groups",
	"activities.groups",
	"activities.categories",
}

func init() {
	MigrationRegistry[CreateOgsIdTriggerVersion] = &Migration{
		Version:     CreateOgsIdTriggerVersion,
		Description: CreateOgsIdTriggerDescription,
		DependsOn:   []string{"1.8.4"}, // After bypass role
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createOgsIdTrigger(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropOgsIdTrigger(ctx, db)
		},
	)
}

// createOgsIdTrigger creates triggers on all domain tables that automatically
// populate ogs_id from the current tenant context (app.ogs_id session variable).
//
// This allows the application code to insert records without explicitly setting ogs_id,
// as long as the tenant context has been set via SET LOCAL app.ogs_id = '...'.
//
// The trigger:
// - Only fires on INSERT when ogs_id is NULL
// - Uses current_ogs_id() to get the tenant ID
// - Raises an error if no tenant context is set (prevents data leakage)
func createOgsIdTrigger(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.5: Creating ogs_id auto-population triggers...")

	// First, create the trigger function
	_, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION set_ogs_id_from_context()
		RETURNS TRIGGER AS $$
		DECLARE
			ctx_ogs_id TEXT;
		BEGIN
			-- Only set ogs_id if it's NULL (allow explicit values)
			IF NEW.ogs_id IS NULL THEN
				ctx_ogs_id := current_ogs_id();

				-- If current_ogs_id() returns the "no context" UUID, raise error
				-- This prevents inserting data without proper tenant context
				IF ctx_ogs_id = '00000000-0000-0000-0000-000000000000' THEN
					RAISE EXCEPTION 'No tenant context set. Call SET LOCAL app.ogs_id before INSERT.';
				END IF;

				NEW.ogs_id := ctx_ogs_id;
			END IF;

			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		COMMENT ON FUNCTION set_ogs_id_from_context() IS
			'Trigger function that auto-populates ogs_id from app.ogs_id session variable.
			 Raises error if no tenant context is set.';
	`)
	if err != nil {
		return fmt.Errorf("error creating set_ogs_id_from_context function: %w", err)
	}
	fmt.Println("  Created set_ogs_id_from_context() trigger function")

	// Create triggers on each table
	for _, table := range triggerTables {
		// Generate trigger name from table name
		// e.g., "users.students" -> "students_set_ogs_id"
		parts := strings.Split(table, ".")
		tableName := parts[len(parts)-1]
		triggerName := tableName + "_set_ogs_id"

		// Drop existing trigger if exists
		dropSQL := fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, triggerName, table)
		_, _ = db.ExecContext(ctx, dropSQL)

		// Create the trigger
		createTriggerSQL := fmt.Sprintf(`
			CREATE TRIGGER %s
				BEFORE INSERT ON %s
				FOR EACH ROW
				EXECUTE FUNCTION set_ogs_id_from_context()
		`, triggerName, table)
		_, err = db.ExecContext(ctx, createTriggerSQL)
		if err != nil {
			return fmt.Errorf("error creating trigger %s on %s: %w", triggerName, table, err)
		}

		fmt.Printf("  Created trigger %s on %s\n", triggerName, table)
	}

	fmt.Println("Migration 1.8.5: Successfully created ogs_id triggers on all domain tables")
	return nil
}

// dropOgsIdTrigger removes the triggers and trigger function.
func dropOgsIdTrigger(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.5: Dropping ogs_id triggers...")

	// Drop triggers from each table
	for _, table := range triggerTables {
		parts := strings.Split(table, ".")
		tableName := parts[len(parts)-1]
		triggerName := tableName + "_set_ogs_id"

		dropSQL := fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, triggerName, table)
		_, _ = db.ExecContext(ctx, dropSQL)

		fmt.Printf("  Dropped trigger %s from %s\n", triggerName, table)
	}

	// Drop the function
	_, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS set_ogs_id_from_context() CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping set_ogs_id_from_context function: %w", err)
	}
	fmt.Println("  Dropped set_ogs_id_from_context() function")

	fmt.Println("Migration 1.8.5: Successfully dropped ogs_id triggers")
	return nil
}
