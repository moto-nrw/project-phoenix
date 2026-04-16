package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createArrivalSchedulesVersion     = "1.15.32"
	createArrivalSchedulesDescription = "Create student arrival schedules, exceptions, and notes tables"
)

var CreateArrivalSchedulesDependsOn = []string{
	UsersStudentsVersion, // Depends on students table (1.3.5)
	"1.2.3",              // Depends on staff table
	enableRLSVersion,     // Depends on RLS infrastructure (1.15.1)
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createArrivalSchedulesVersion,
		Description: createArrivalSchedulesDescription,
		DependsOn:   CreateArrivalSchedulesDependsOn,
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createArrivalSchedulesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createArrivalSchedulesDown(ctx, db)
		},
	)
}

func createArrivalSchedulesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.32: Creating student arrival schedules tables...")

	// Create student_arrival_schedules table (weekly recurring)
	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_arrival_schedules (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			weekday INT NOT NULL CHECK (weekday BETWEEN 1 AND 5),
			expected_arrival TIME NOT NULL,
			notes VARCHAR(500),
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT unique_arrival_student_weekday UNIQUE (tenant_id, student_id, weekday)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_arrival_schedules table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_arrival_schedules_student_id
		ON schedule.student_arrival_schedules(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on student_id: %w", err)
	}

	// Enable RLS on arrival schedules
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_schedules ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on student_arrival_schedules: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_schedules FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to force RLS on student_arrival_schedules: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_student_arrival_schedules
		ON schedule.student_arrival_schedules FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create RLS policy on student_arrival_schedules: %w", err)
	}

	// Create student_arrival_exceptions table (date-specific overrides)
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_arrival_exceptions (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			exception_date DATE NOT NULL,
			expected_arrival TIME,
			reason VARCHAR(255),
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT unique_arrival_student_exception_date UNIQUE (tenant_id, student_id, exception_date)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_arrival_exceptions table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_arrival_exceptions_student_id
		ON schedule.student_arrival_exceptions(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on student_id: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_arrival_exceptions_date
		ON schedule.student_arrival_exceptions(exception_date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on exception_date: %w", err)
	}

	// Enable RLS on arrival exceptions
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_exceptions ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on student_arrival_exceptions: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_exceptions FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to force RLS on student_arrival_exceptions: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_student_arrival_exceptions
		ON schedule.student_arrival_exceptions FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create RLS policy on student_arrival_exceptions: %w", err)
	}

	// Create student_arrival_notes table
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_arrival_notes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			note_date DATE NOT NULL,
			content VARCHAR(500) NOT NULL,
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_arrival_notes table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_arrival_notes_student_date
		ON schedule.student_arrival_notes(student_id, note_date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating composite index on student_arrival_notes: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_arrival_notes_student_id
		ON schedule.student_arrival_notes(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_id index on student_arrival_notes: %w", err)
	}

	// Enable RLS on arrival notes
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_notes ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on student_arrival_notes: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.student_arrival_notes FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to force RLS on student_arrival_notes: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_student_arrival_notes
		ON schedule.student_arrival_notes FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create RLS policy on student_arrival_notes: %w", err)
	}

	return nil
}

func createArrivalSchedulesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.32: Dropping student arrival schedules tables...")

	_, err := db.NewRaw(`DROP TABLE IF EXISTS schedule.student_arrival_notes CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_arrival_notes table: %w", err)
	}

	_, err = db.NewRaw(`DROP TABLE IF EXISTS schedule.student_arrival_exceptions CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_arrival_exceptions table: %w", err)
	}

	_, err = db.NewRaw(`DROP TABLE IF EXISTS schedule.student_arrival_schedules CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_arrival_schedules table: %w", err)
	}

	return nil
}
