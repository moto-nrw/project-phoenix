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
	ActivitiesGroupsCreatedByDescription = "Add created_by column to activities.groups with table truncation"
)

var ActivitiesGroupsCreatedByDependencies = []string{
	"1.3.2", // activities.groups table
	"1.2.2", // users.staff table
}

// migrateActivitiesGroupsCreatedBy truncates the activities.groups table and adds created_by column
func migrateActivitiesGroupsCreatedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.2: Adding created_by column to activities.groups...")

	// Step 1: Truncate activities.groups with cascade
	// This removes all activity data and cascades to related tables:
	// - activities.supervisors_planned
	// - activities.schedules
	// - activities.student_enrollments
	// - active.groups
	fmt.Println("  Truncating activities.groups with cascade...")
	_, err := db.ExecContext(ctx, `TRUNCATE TABLE activities.groups CASCADE`)
	if err != nil {
		return fmt.Errorf("error truncating activities.groups: %w", err)
	}

	// Step 2: Add created_by column with NOT NULL constraint
	fmt.Println("  Adding created_by column...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD COLUMN created_by BIGINT NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error adding created_by column: %w", err)
	}

	// Step 3: Add foreign key constraint
	fmt.Println("  Adding foreign key constraint...")
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ADD CONSTRAINT fk_activity_groups_created_by
		FOREIGN KEY (created_by) REFERENCES users.staff(id)
	`)
	if err != nil {
		return fmt.Errorf("error adding foreign key constraint: %w", err)
	}

	// Step 4: Add index for performance
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
