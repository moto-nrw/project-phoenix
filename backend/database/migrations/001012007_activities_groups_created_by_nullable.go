package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activitiesGroupsCreatedByNullableVersion     = "1.12.7"
	activitiesGroupsCreatedByNullableDescription = "Make created_by nullable in activities.groups for system-created activities"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activitiesGroupsCreatedByNullableVersion,
		Description: activitiesGroupsCreatedByNullableDescription,
		DependsOn:   []string{"1.8.3"}, // migration that added the created_by NOT NULL column
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return makeActivitiesGroupsCreatedByNullable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackActivitiesGroupsCreatedByNullable(ctx, db)
		},
	)
}

func makeActivitiesGroupsCreatedByNullable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.7: Making activities.groups.created_by nullable...")

	// Idempotency: skip if already nullable
	var isNullable string
	err := db.QueryRowContext(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = 'activities'
		AND table_name = 'groups'
		AND column_name = 'created_by'
	`).Scan(&isNullable)
	if err != nil {
		return fmt.Errorf("error checking created_by nullable status: %w", err)
	}
	if isNullable == "YES" {
		fmt.Println("  created_by is already nullable - skipping migration")
		return nil
	}

	// Drop NOT NULL constraint (FK is retained, nullable FK is valid in Postgres)
	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ALTER COLUMN created_by DROP NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error dropping NOT NULL from created_by: %w", err)
	}

	fmt.Println("Migration 1.12.7 completed: activities.groups.created_by is now nullable")
	return nil
}

func rollbackActivitiesGroupsCreatedByNullable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.7: Restoring NOT NULL on activities.groups.created_by...")

	// Backfill any NULL values before re-adding NOT NULL constraint
	result, err := db.ExecContext(ctx, `
		UPDATE activities.groups g
		SET created_by = (
			SELECT s.staff_id
			FROM activities.supervisors s
			WHERE s.group_id = g.id
			ORDER BY s.is_primary DESC, s.id
			LIMIT 1
		)
		WHERE g.created_by IS NULL
	`)
	if err != nil {
		return fmt.Errorf("error backfilling NULL created_by from supervisors: %w", err)
	}
	rowsUpdated, _ := result.RowsAffected()
	if rowsUpdated > 0 {
		fmt.Printf("  Backfilled %d rows from supervisors\n", rowsUpdated)
	}

	// Fallback: any still-NULL rows get the first available staff member
	_, err = db.ExecContext(ctx, `
		UPDATE activities.groups
		SET created_by = (
			SELECT s.id FROM users.staff s ORDER BY s.id LIMIT 1
		)
		WHERE created_by IS NULL
	`)
	if err != nil {
		return fmt.Errorf("error backfilling NULL created_by with fallback staff: %w", err)
	}

	// Verify no NULLs remain
	var nullCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM activities.groups WHERE created_by IS NULL
	`).Scan(&nullCount)
	if err != nil {
		return fmt.Errorf("error verifying no nulls: %w", err)
	}
	if nullCount > 0 {
		return fmt.Errorf("cannot restore NOT NULL: %d groups still have NULL created_by (no staff found)", nullCount)
	}

	_, err = db.ExecContext(ctx, `
		ALTER TABLE activities.groups
		ALTER COLUMN created_by SET NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error restoring NOT NULL on created_by: %w", err)
	}

	fmt.Println("Rollback 1.12.7 completed: activities.groups.created_by is NOT NULL again")
	return nil
}
