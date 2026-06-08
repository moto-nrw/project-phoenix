package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentEnrollmentSelectedWeekdaysStrictVersion     = "1.15.111"
	studentEnrollmentSelectedWeekdaysStrictDescription = "Tighten selected weekdays validation on activity student enrollments"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentEnrollmentSelectedWeekdaysStrictVersion,
		Description: studentEnrollmentSelectedWeekdaysStrictDescription,
		DependsOn: []string{
			studentEnrollmentSelectedWeekdaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentEnrollmentSelectedWeekdaysStrictUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentEnrollmentSelectedWeekdaysStrictDown(ctx, db)
		},
	)
}

func studentEnrollmentSelectedWeekdaysStrictUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.111: Tightening selected_weekdays validation on activities.student_enrollments...")

	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION activities.is_valid_selected_weekdays(value JSONB)
		RETURNS BOOLEAN
		LANGUAGE sql
		IMMUTABLE
		AS $$
			SELECT value IS NULL OR (
				jsonb_typeof(value) = 'array'
				AND jsonb_array_length(value) > 0
				AND jsonb_array_length(value) = (
					SELECT COUNT(DISTINCT elem.item)
					FROM jsonb_array_elements(value) AS elem(item)
				)
				AND NOT EXISTS (
					SELECT 1
					FROM jsonb_array_elements(value) AS elem(item)
					WHERE jsonb_typeof(elem.item) <> 'number'
						OR elem.item::text !~ '^[0-9]+$'
						OR (elem.item::text)::int < 1
						OR (elem.item::text)::int > 7
				)
			)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating selected_weekdays validation function: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS check_student_enrollments_selected_weekdays,
			ADD CONSTRAINT check_student_enrollments_selected_weekdays CHECK (
				activities.is_valid_selected_weekdays(selected_weekdays)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed tightening selected_weekdays check constraint: %w", err)
	}

	return nil
}

func studentEnrollmentSelectedWeekdaysStrictDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.111: Restoring loose selected_weekdays validation...")

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS check_student_enrollments_selected_weekdays,
			ADD CONSTRAINT check_student_enrollments_selected_weekdays CHECK (
				selected_weekdays IS NULL OR (
					jsonb_typeof(selected_weekdays) = 'array'
					AND jsonb_array_length(selected_weekdays) > 0
				)
			);
		DROP FUNCTION IF EXISTS activities.is_valid_selected_weekdays(JSONB);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring loose selected_weekdays check constraint: %w", err)
	}
	return nil
}
