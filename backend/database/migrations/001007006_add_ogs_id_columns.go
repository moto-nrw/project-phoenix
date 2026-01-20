package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	AddOgsIdColumnsVersion     = "1.7.6"
	AddOgsIdColumnsDescription = "Add ogs_id columns to domain tables for multi-tenancy"
)

// Tables that need ogs_id column for multi-tenancy partitioning.
// These are the primary domain tables that hold tenant-specific data.
var ogsIdTables = []string{
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
	MigrationRegistry[AddOgsIdColumnsVersion] = &Migration{
		Version:     AddOgsIdColumnsVersion,
		Description: AddOgsIdColumnsDescription,
		DependsOn:   []string{"1.7.5"}, // Depends on previous migration
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addOgsIdColumns(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeOgsIdColumns(ctx, db)
		},
	)
}

// addOgsIdColumns adds the ogs_id column to all domain tables.
// This column will reference public.organization.id (TEXT type from BetterAuth).
// Initially nullable to allow backfilling existing data in a later migration.
func addOgsIdColumns(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.6: Adding ogs_id columns for multi-tenancy...")

	for _, table := range ogsIdTables {
		// Add column (nullable initially to allow backfilling)
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
			ADD COLUMN IF NOT EXISTS ogs_id TEXT
		`, table))
		if err != nil {
			return fmt.Errorf("error adding ogs_id to %s: %w", table, err)
		}

		// Add index for query performance (crucial for RLS policies)
		// Convert schema.table to schema_table for index name
		indexName := fmt.Sprintf("idx_%s_ogs_id", strings.ReplaceAll(table, ".", "_"))
		_, err = db.ExecContext(ctx, fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s(ogs_id)
		`, indexName, table))
		if err != nil {
			return fmt.Errorf("error creating index on %s: %w", table, err)
		}

		fmt.Printf("  Added ogs_id column and index to %s\n", table)
	}

	fmt.Println("Migration 1.7.6: Successfully added ogs_id columns to all domain tables")
	return nil
}

// removeOgsIdColumns removes the ogs_id columns from all domain tables.
// CASCADE ensures any dependent indexes are also dropped.
func removeOgsIdColumns(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.6: Removing ogs_id columns...")

	for _, table := range ogsIdTables {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s DROP COLUMN IF EXISTS ogs_id CASCADE
		`, table))
		if err != nil {
			return fmt.Errorf("error dropping ogs_id from %s: %w", table, err)
		}

		fmt.Printf("  Removed ogs_id column from %s\n", table)
	}

	fmt.Println("Migration 1.7.6: Successfully removed ogs_id columns")
	return nil
}
