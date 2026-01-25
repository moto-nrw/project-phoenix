package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createPickupSchedulesVersion     = "1.8.1"
	createPickupSchedulesDescription = "Create student pickup schedules and exceptions tables"
)

// PickupSchedulesDependsOn defines migration dependencies
var PickupSchedulesDependsOn = []string{
	UsersStudentsVersion, // Depends on students table (1.3.5)
	"1.2.3",              // Depends on staff table
}

func init() {
	MigrationRegistry[createPickupSchedulesVersion] = &Migration{
		Version:     createPickupSchedulesVersion,
		Description: createPickupSchedulesDescription,
		DependsOn:   PickupSchedulesDependsOn,
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createPickupSchedulesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createPickupSchedulesDown(ctx, db)
		},
	)
}

func createPickupSchedulesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.1: Creating student pickup schedules tables...")

	// Create student_pickup_schedules table (weekly recurring)
	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_pickup_schedules (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			weekday INT NOT NULL CHECK (weekday BETWEEN 1 AND 5),
			pickup_time TIME NOT NULL,
			notes VARCHAR(500),
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT unique_student_weekday UNIQUE (student_id, weekday)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_pickup_schedules table: %w", err)
	}

	// Create index on student_id for faster lookups
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_pickup_schedules_student_id
		ON schedule.student_pickup_schedules(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on student_id: %w", err)
	}

	// Create student_pickup_exceptions table (date-specific overrides)
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_pickup_exceptions (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			exception_date DATE NOT NULL,
			pickup_time TIME,
			reason VARCHAR(255) NOT NULL,
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT unique_student_exception_date UNIQUE (student_id, exception_date)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_pickup_exceptions table: %w", err)
	}

	// Create index on student_id for faster lookups
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_pickup_exceptions_student_id
		ON schedule.student_pickup_exceptions(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on student_id: %w", err)
	}

	// Create index on exception_date for filtering upcoming exceptions
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_pickup_exceptions_date
		ON schedule.student_pickup_exceptions(exception_date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on exception_date: %w", err)
	}

	return nil
}

func createPickupSchedulesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.1: Dropping student pickup schedules tables...")

	// Drop exceptions table first (no dependencies)
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS schedule.student_pickup_exceptions CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_pickup_exceptions table: %w", err)
	}

	// Drop schedules table
	_, err = db.NewRaw(`
		DROP TABLE IF EXISTS schedule.student_pickup_schedules CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_pickup_schedules table: %w", err)
	}

	return nil
}
