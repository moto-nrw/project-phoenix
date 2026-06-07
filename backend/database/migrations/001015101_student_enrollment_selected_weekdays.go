package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentEnrollmentSelectedWeekdaysVersion     = "1.15.101"
	studentEnrollmentSelectedWeekdaysDescription = "Add selected weekdays to activity student enrollments"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentEnrollmentSelectedWeekdaysVersion,
		Description: studentEnrollmentSelectedWeekdaysDescription,
		DependsOn: []string{
			invitationTokensCaregiverEnabledVersion,
			ActivitiesStudentEnrollmentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentEnrollmentSelectedWeekdaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentEnrollmentSelectedWeekdaysDown(ctx, db)
		},
	)
}

func studentEnrollmentSelectedWeekdaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.101: Adding selected_weekdays to activities.student_enrollments...")

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS selected_weekdays JSONB;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding selected_weekdays to activities.student_enrollments: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS check_student_enrollments_selected_weekdays,
			ADD CONSTRAINT check_student_enrollments_selected_weekdays CHECK (
				selected_weekdays IS NULL OR (
					jsonb_typeof(selected_weekdays) = 'array'
					AND jsonb_array_length(selected_weekdays) > 0
				)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding selected_weekdays check constraint: %w", err)
	}

	return nil
}

func studentEnrollmentSelectedWeekdaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.101: Removing selected_weekdays from activities.student_enrollments...")

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS check_student_enrollments_selected_weekdays,
			DROP COLUMN IF EXISTS selected_weekdays;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing selected_weekdays from activities.student_enrollments: %w", err)
	}
	return nil
}
