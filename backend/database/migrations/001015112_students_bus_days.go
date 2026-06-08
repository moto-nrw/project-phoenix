package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsBusDaysVersion     = "1.15.112"
	studentsBusDaysDescription = "Add weekday-specific bus days to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsBusDaysVersion,
		Description: studentsBusDaysDescription,
		DependsOn: []string{
			addBusToStudentsVersion,
			studentEnrollmentSelectedWeekdaysStrictVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsBusDaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsBusDaysDown(ctx, db)
		},
	)
}

func studentsBusDaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.112: Adding bus_days to users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS bus_days JSONB NOT NULL DEFAULT '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding bus_days to users.students: %w", err)
	}

	if _, err := db.NewRaw(`
		UPDATE users.students
		SET bus_days = CASE
			WHEN COALESCE(bus, FALSE) THEN '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb
			ELSE '{}'::jsonb
		END
		WHERE bus_days = '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling users.students.bus_days: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION users.is_valid_bus_days(value JSONB)
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
		return fmt.Errorf("failed creating users.students bus_days validation function: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_bus_days,
			ADD CONSTRAINT check_students_bus_days CHECK (users.is_valid_bus_days(bus_days));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding users.students.bus_days check constraint: %w", err)
	}

	return nil
}

func studentsBusDaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.112: Removing bus_days from users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_bus_days,
			DROP COLUMN IF EXISTS bus_days;
		DROP FUNCTION IF EXISTS users.is_valid_bus_days(JSONB);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing users.students.bus_days: %w", err)
	}
	return nil
}
