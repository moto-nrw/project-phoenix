package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	periodScopedRosterUniquenessVersion     = "1.15.52"
	periodScopedRosterUniquenessDescription = "Make active template roster uniqueness period-scoped"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     periodScopedRosterUniquenessVersion,
		Description: periodScopedRosterUniquenessDescription,
		DependsOn: []string{
			templateExtensionsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return periodScopedRosterUniquenessUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return periodScopedRosterUniquenessDown(ctx, db)
		},
	)
}

func periodScopedRosterUniquenessUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.52: Making active template roster uniqueness period-scoped...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_student_enrollments_active;
		CREATE UNIQUE INDEX idx_student_enrollments_active
		ON activities.student_enrollments (
			tenant_id,
			student_id,
			activity_group_id,
			COALESCE(calendar_period_id, 0)
		)
		WHERE valid_until IS NULL;

		DROP INDEX IF EXISTS activities.idx_supervisors_active;
		CREATE UNIQUE INDEX idx_supervisors_active
		ON activities.supervisors (
			tenant_id,
			staff_id,
			group_id,
			COALESCE(calendar_period_id, 0)
		)
		WHERE valid_until IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed recreating period-scoped roster unique indexes: %w", err)
	}

	return nil
}

func periodScopedRosterUniquenessDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.52: Restoring global active roster uniqueness...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_student_enrollments_active;
		CREATE UNIQUE INDEX idx_student_enrollments_active
		ON activities.student_enrollments (tenant_id, student_id, activity_group_id)
		WHERE valid_until IS NULL;

		DROP INDEX IF EXISTS activities.idx_supervisors_active;
		CREATE UNIQUE INDEX idx_supervisors_active
		ON activities.supervisors (tenant_id, staff_id, group_id)
		WHERE valid_until IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring global roster unique indexes: %w", err)
	}

	return nil
}
