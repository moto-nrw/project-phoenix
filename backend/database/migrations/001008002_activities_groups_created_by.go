package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 1.8.2: Add created_by column to activities.groups
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return migrateActivitiesGroupsCreatedBy(ctx, db)
	}, func(ctx context.Context, db *bun.DB) error {
		return rollbackActivitiesGroupsCreatedBy(ctx, db)
	})
}

const (
	ActivitiesGroupsCreatedByVersion     = "1.8.2"
	ActivitiesGroupsCreatedByDescription = "Add created_by column to activities.groups"
)

var ActivitiesGroupsCreatedByDependencies = []string{
	"1.3.2", // activities.groups table
	"1.2.2", // users.staff table
}

// migrateActivitiesGroupsCreatedBy adds created_by column to activities.groups
// WARNING: This migration truncates activities.groups and all dependent tables (CASCADE)
// to add a NOT NULL created_by column. Deploy during off-hours when no active sessions exist.
// Affected tables: activities.groups, activities.supervisors, activities.schedules,
// activities.student_enrollments, active.groups, active.visits, active.group_supervisors,
// active.group_mappings
func migrateActivitiesGroupsCreatedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.2: Adding created_by column to activities.groups...")

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

	// Step 1: Check for existing data and log what will be deleted
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activities.groups`).Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking activities.groups count: %w", err)
	}

	if count > 0 {
		fmt.Printf("  WARNING: Found %d existing activity groups - will be deleted (CASCADE)\n", count)
		fmt.Println("  Cascading tables: activities.supervisors, activities.schedules,")
		fmt.Println("                    activities.student_enrollments, active.groups,")
		fmt.Println("                    active.visits, active.group_supervisors, active.group_mappings")
	}

	// Step 2: Truncate table with CASCADE to clear dependent data
	fmt.Println("  Truncating activities.groups (CASCADE)...")
	_, err = db.ExecContext(ctx, `TRUNCATE TABLE activities.groups CASCADE`)
	if err != nil {
		return fmt.Errorf("error truncating activities.groups: %w", err)
	}

	// Step 3: Add created_by column with NOT NULL constraint
	fmt.Println("  Adding created_by column...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD COLUMN created_by BIGINT NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error adding created_by column: %w", err)
	}

	// Step 4: Add foreign key constraint
	fmt.Println("  Adding foreign key constraint...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD CONSTRAINT fk_activity_groups_created_by
		FOREIGN KEY (created_by) REFERENCES users.staff(id)
	`)
	if err != nil {
		return fmt.Errorf("error adding foreign key constraint: %w", err)
	}

	// Step 5: Add index for performance
	fmt.Println("  Creating index on created_by...")
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_groups_created_by
		ON activities.groups(created_by)
	`)
	if err != nil {
		return fmt.Errorf("error creating index: %w", err)
	}

	fmt.Println("Migration 1.8.2 completed successfully")
	return nil
}

// rollbackActivitiesGroupsCreatedBy removes the created_by column
func rollbackActivitiesGroupsCreatedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.2: Removing created_by column from activities.groups...")

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

	fmt.Println("Rollback 1.8.2 completed successfully")
	return nil
}
