package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupCalendarPeriodVersion     = "1.15.181"
	groupCalendarPeriodDescription = "Add calendar_period_id FK to activities.groups - lets a template pin its own calendar period independent of its schedule rows"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupCalendarPeriodVersion,
		Description: groupCalendarPeriodDescription,
		DependsOn: []string{
			enrollmentNotificationModeAndCleanupGrantsVersion, // 1.15.180
			createCalendarPeriodsVersion,                      // 1.15.33
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return groupCalendarPeriodUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return groupCalendarPeriodDown(ctx, db)
		},
	)
}

func groupCalendarPeriodUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.181: Adding calendar_period_id to activities.groups...")

	// ON DELETE SET NULL: deleting a calendar period clears references
	// instead of blocking the delete with a FK violation (same pattern as
	// enrollment.phases.calendar_period_id, migration 1.15.167). The
	// materialization service's selectPeriod() already prefers the more
	// specific per-schedule-row pin over this template-level one; this
	// column is the fallback for read paths that only have the Group row
	// (e.g. the template list response) and for templates whose schedule
	// rows don't carry their own pin.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding calendar_period_id to activities.groups: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activities_groups_tenant_calendar_period
			ON activities.groups (tenant_id, calendar_period_id)
			WHERE calendar_period_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activities.groups calendar_period_id index: %w", err)
	}

	fmt.Println("Migration 1.15.181: Completed successfully")
	return nil
}

func groupCalendarPeriodDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.181: Dropping calendar_period_id from activities.groups...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activities_groups_tenant_calendar_period;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS calendar_period_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping calendar_period_id from activities.groups: %w", err)
	}

	return nil
}
