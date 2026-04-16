package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createCalendarPeriodsVersion     = "1.15.33"
	createCalendarPeriodsDescription = "Create calendar periods table for timetable system"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createCalendarPeriodsVersion,
		Description: createCalendarPeriodsDescription,
		DependsOn: []string{
			enableRLSVersion, // Depends on RLS infrastructure (1.15.1)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createCalendarPeriodsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createCalendarPeriodsDown(ctx, db)
		},
	)
}

func createCalendarPeriodsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.33: Creating calendar periods table...")

	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.calendar_periods (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			name VARCHAR(255) NOT NULL,
			period_type TEXT NOT NULL CHECK (period_type IN ('school_year', 'semester', 'holiday', 'custom')),
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			week_cycle_length SMALLINT NOT NULL DEFAULT 1,
			week_cycle_anchor DATE,
			is_active BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			CONSTRAINT unique_calendar_period_name UNIQUE (tenant_id, name),
			CONSTRAINT check_calendar_period_dates CHECK (start_date < end_date),
			CONSTRAINT check_week_cycle_length CHECK (week_cycle_length >= 1)
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating calendar_periods table: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_calendar_periods_tenant_id
		ON schedule.calendar_periods(tenant_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on tenant_id: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_calendar_periods_active
		ON schedule.calendar_periods(tenant_id, is_active) WHERE is_active = true;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating partial index on is_active: %w", err)
	}

	// Enable RLS
	_, err = db.NewRaw(`ALTER TABLE schedule.calendar_periods ENABLE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on calendar_periods: %w", err)
	}
	_, err = db.NewRaw(`ALTER TABLE schedule.calendar_periods FORCE ROW LEVEL SECURITY`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to force RLS on calendar_periods: %w", err)
	}
	_, err = db.NewRaw(`
		CREATE POLICY tenant_isolation_schedule_calendar_periods
		ON schedule.calendar_periods FOR ALL
		USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
		WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create RLS policy on calendar_periods: %w", err)
	}

	// Grants are handled automatically via ALTER DEFAULT PRIVILEGES in migration 1.14.1
	// (phoenix_tenant gets SELECT/INSERT/UPDATE/DELETE on all tables in the schedule schema)

	return nil
}

func createCalendarPeriodsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.33: Dropping calendar periods table...")

	_, err := db.NewRaw(`DROP TABLE IF EXISTS schedule.calendar_periods CASCADE;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping calendar_periods table: %w", err)
	}

	return nil
}
