package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseCalendarPeriodVersion     = "1.15.167"
	phaseCalendarPeriodDescription = "Add calendar_period_id FK to enrollment.phases - links enrollment phases to the shared planning calendar (schedule.calendar_periods)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseCalendarPeriodVersion,
		Description: phaseCalendarPeriodDescription,
		DependsOn: []string{
			formSchemasMigrateLegacyDepartureVersion, // 1.15.166
			createCalendarPeriodsVersion,             // 1.15.33
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return phaseCalendarPeriodUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return phaseCalendarPeriodDown(ctx, db)
		},
	)
}

func phaseCalendarPeriodUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.167: Adding calendar_period_id to enrollment.phases...")

	// ON DELETE SET NULL: deleting a calendar period clears references
	// instead of blocking the delete with a FK violation (same pattern
	// as activities.schedules.calendar_period_id, migration 1.15.34).
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
			ADD COLUMN IF NOT EXISTS calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding calendar_period_id to enrollment.phases: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_enrollment_phases_tenant_calendar_period
			ON enrollment.phases (tenant_id, calendar_period_id)
			WHERE calendar_period_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating enrollment.phases calendar_period_id index: %w", err)
	}

	fmt.Println("Migration 1.15.167: Completed successfully")
	return nil
}

func phaseCalendarPeriodDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.167: Dropping calendar_period_id from enrollment.phases...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS enrollment.idx_enrollment_phases_tenant_calendar_period;

		ALTER TABLE enrollment.phases
			DROP COLUMN IF EXISTS calendar_period_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping calendar_period_id from enrollment.phases: %w", err)
	}

	return nil
}
