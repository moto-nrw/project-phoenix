package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsPickupDaysVersion     = "1.15.116"
	studentsPickupDaysDescription = "Add weekday-specific pickup days to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsPickupDaysVersion,
		Description: studentsPickupDaysDescription,
		DependsOn: []string{
			addPickupStatusToStudentsVersion,
			studentsBusDaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsPickupDaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsPickupDaysDown(ctx, db)
		},
	)
}

func studentsPickupDaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.116: Adding pickup_days to users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS pickup_days JSONB NOT NULL DEFAULT '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding pickup_days to users.students: %w", err)
	}

	// Backfill from the legacy single status: "Wird abgeholt" implies all five
	// weekdays. "Geht alleine nach Hause" and unset stay empty. Only pickup_days
	// is written here — pickup_status is left untouched so existing displays
	// stay stable until a child is next edited.
	if _, err := db.NewRaw(`
		UPDATE users.students
		SET pickup_days = CASE
			WHEN pickup_status = 'Wird abgeholt' THEN '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb
			ELSE '{}'::jsonb
		END
		WHERE pickup_days = '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling users.students.pickup_days: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION users.is_valid_pickup_days(value JSONB)
		RETURNS BOOLEAN
		LANGUAGE sql
		IMMUTABLE
		AS $$
			SELECT jsonb_typeof(value) = 'object'
				AND NOT EXISTS (
					SELECT 1
					FROM jsonb_each(value) AS elem(key, value)
					WHERE elem.key NOT IN ('mon', 'tue', 'wed', 'thu', 'fri')
						OR jsonb_typeof(elem.value) <> 'boolean'
				)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.students pickup_days validation function: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_pickup_days,
			ADD CONSTRAINT check_students_pickup_days CHECK (users.is_valid_pickup_days(pickup_days));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding users.students.pickup_days check constraint: %w", err)
	}

	return nil
}

func studentsPickupDaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.116: Removing pickup_days from users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_pickup_days,
			DROP COLUMN IF EXISTS pickup_days;
		DROP FUNCTION IF EXISTS users.is_valid_pickup_days(JSONB);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing users.students.pickup_days: %w", err)
	}
	return nil
}
