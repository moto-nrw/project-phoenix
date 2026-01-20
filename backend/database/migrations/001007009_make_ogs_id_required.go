package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	MakeOgsIdRequiredVersion     = "1.7.9"
	MakeOgsIdRequiredDescription = "Make ogs_id NOT NULL on all domain tables"
)

func init() {
	MigrationRegistry[MakeOgsIdRequiredVersion] = &Migration{
		Version:     MakeOgsIdRequiredVersion,
		Description: MakeOgsIdRequiredDescription,
		DependsOn:   []string{"1.7.8"}, // Depends on backfill completing
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return makeOgsIdRequired(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return makeOgsIdNullable(ctx, db)
		},
	)
}

// makeOgsIdRequired adds NOT NULL constraint to ogs_id on all domain tables.
// This is the final step in the multi-tenancy column setup.
// Fails fast if any table has NULL ogs_id values (backfill should have fixed this).
func makeOgsIdRequired(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.9: Making ogs_id NOT NULL on all domain tables...")

	// Tables that need NOT NULL constraint (same as previous migrations)
	tables := []string{
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

	// First pass: verify no NULL values exist in any table
	fmt.Println("  Verifying no NULL ogs_id values exist...")
	for _, table := range tables {
		var nullCount int
		err := db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s WHERE ogs_id IS NULL
		`, table)).Scan(&nullCount)
		if err != nil {
			return fmt.Errorf("error checking NULL values in %s: %w", table, err)
		}

		if nullCount > 0 {
			return fmt.Errorf("table %s has %d rows with NULL ogs_id - run backfill migration 1.7.8 first", table, nullCount)
		}
	}
	fmt.Println("  All tables verified - no NULL values found")

	// Second pass: add NOT NULL constraint
	fmt.Println("  Adding NOT NULL constraints...")
	for _, table := range tables {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s ALTER COLUMN ogs_id SET NOT NULL
		`, table))
		if err != nil {
			return fmt.Errorf("error making ogs_id NOT NULL on %s: %w", table, err)
		}

		fmt.Printf("    Made ogs_id NOT NULL on %s\n", table)
	}

	fmt.Println("Migration 1.7.9: Successfully made ogs_id NOT NULL on all domain tables")
	return nil
}

// makeOgsIdNullable removes the NOT NULL constraint from ogs_id.
// This is the rollback operation - allows ogs_id to be NULL again.
func makeOgsIdNullable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.9: Making ogs_id nullable...")

	tables := []string{
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

	for _, table := range tables {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s ALTER COLUMN ogs_id DROP NOT NULL
		`, table))
		if err != nil {
			return fmt.Errorf("error making ogs_id nullable on %s: %w", table, err)
		}

		fmt.Printf("  Made ogs_id nullable on %s\n", table)
	}

	fmt.Println("Migration 1.7.9: Rollback complete - ogs_id is now nullable")
	return nil
}
