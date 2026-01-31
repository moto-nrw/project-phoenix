package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry[ActivitiesGroupsCreatedByVersion] = &Migration{
		Version:     ActivitiesGroupsCreatedByVersion,
		Description: ActivitiesGroupsCreatedByDescription,
		DependsOn:   ActivitiesGroupsCreatedByDependencies,
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return migrateActivitiesGroupsCreatedBy(ctx, db)
	}, func(ctx context.Context, db *bun.DB) error {
		return rollbackActivitiesGroupsCreatedBy(ctx, db)
	})
}

const (
	ActivitiesGroupsCreatedByVersion     = "1.8.3"
	ActivitiesGroupsCreatedByDescription = "Add created_by column to activities.groups"
)

var ActivitiesGroupsCreatedByDependencies = []string{
	"1.3.2", // activities.groups table
	"1.2.2", // users.staff table
	"1.3.4", // activities.supervisors table (for backfill query)
}

// migrateActivitiesGroupsCreatedBy adds created_by column to activities.groups
// This migration preserves all existing data by:
// 1. Adding the column as NULLABLE
// 2. Backfilling existing rows with their first supervisor (or fallback staff)
// 3. Setting NOT NULL constraint
func migrateActivitiesGroupsCreatedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.3: Adding created_by column to activities.groups...")

	// Step 0: Check if column already exists (idempotency guard)
	var columnExists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'activities'
			AND table_name = 'groups'
			AND column_name = 'created_by'
		)
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("error checking if created_by column exists: %w", err)
	}
	if columnExists {
		fmt.Println("  Column created_by already exists - skipping migration")
		return nil
	}

	// Step 1: Add created_by column as NULLABLE first
	fmt.Println("  Adding created_by column (nullable)...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD COLUMN created_by BIGINT
	`)
	if err != nil {
		return fmt.Errorf("error adding created_by column: %w", err)
	}

	// Step 2: Backfill existing rows with first assigned supervisor (prefer primary)
	fmt.Println("  Backfilling existing groups with first supervisor...")
	result, err := db.ExecContext(ctx, `
		UPDATE activities.groups g
		SET created_by = (
			SELECT s.staff_id
			FROM activities.supervisors s
			WHERE s.group_id = g.id
			ORDER BY s.is_primary DESC, s.id
			LIMIT 1
		)
		WHERE created_by IS NULL
	`)
	if err != nil {
		return fmt.Errorf("error backfilling created_by from supervisors: %w", err)
	}
	rowsUpdated, _ := result.RowsAffected()
	fmt.Printf("  Updated %d groups with their first supervisor\n", rowsUpdated)

	// Step 3: Handle any remaining NULL values (groups without supervisors)
	// Use the first active staff member as fallback
	var remainingNulls int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM activities.groups WHERE created_by IS NULL
	`).Scan(&remainingNulls)
	if err != nil {
		return fmt.Errorf("error checking remaining nulls: %w", err)
	}

	if remainingNulls > 0 {
		fmt.Printf("  Found %d groups without supervisors - using fallback staff...\n", remainingNulls)
		_, err = db.ExecContext(ctx, `
			UPDATE activities.groups
			SET created_by = (
				SELECT s.id FROM users.staff s
				WHERE s.is_active = true
				ORDER BY s.id
				LIMIT 1
			)
			WHERE created_by IS NULL
		`)
		if err != nil {
			return fmt.Errorf("error backfilling created_by with fallback staff: %w", err)
		}
	}

	// Step 4: Verify no NULLs remain before adding constraint
	var stillNull int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM activities.groups WHERE created_by IS NULL
	`).Scan(&stillNull)
	if err != nil {
		return fmt.Errorf("error verifying no nulls: %w", err)
	}
	if stillNull > 0 {
		return fmt.Errorf("cannot add NOT NULL constraint: %d groups still have NULL created_by (no staff found)", stillNull)
	}

	// Step 5: Set NOT NULL constraint
	fmt.Println("  Setting NOT NULL constraint...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ALTER COLUMN created_by SET NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error setting NOT NULL constraint: %w", err)
	}

	// Step 6: Add foreign key constraint
	fmt.Println("  Adding foreign key constraint...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD CONSTRAINT fk_activity_groups_created_by
		FOREIGN KEY (created_by) REFERENCES users.staff(id)
	`)
	if err != nil {
		return fmt.Errorf("error adding foreign key constraint: %w", err)
	}

	// Step 7: Add index for performance
	fmt.Println("  Creating index on created_by...")
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_groups_created_by
		ON activities.groups(created_by)
	`)
	if err != nil {
		return fmt.Errorf("error creating index: %w", err)
	}

	fmt.Println("Migration 1.8.3 completed successfully (zero data loss)")
	return nil
}

// rollbackActivitiesGroupsCreatedBy removes the created_by column
func rollbackActivitiesGroupsCreatedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.3: Removing created_by column from activities.groups...")

	// Drop index
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_activity_groups_created_by
	`)
	if err != nil {
		return fmt.Errorf("error dropping index: %w", err)
	}

	// Drop foreign key constraint
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		DROP CONSTRAINT IF EXISTS fk_activity_groups_created_by
	`)
	if err != nil {
		return fmt.Errorf("error dropping foreign key constraint: %w", err)
	}

	// Drop column
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		DROP COLUMN IF EXISTS created_by
	`)
	if err != nil {
		return fmt.Errorf("error dropping created_by column: %w", err)
	}

	fmt.Println("Rollback 1.8.3 completed successfully")
	return nil
}
