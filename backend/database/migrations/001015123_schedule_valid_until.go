package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	scheduleValidUntilVersion     = "1.15.123"
	scheduleValidUntilDescription = "Add valid_until DATE column to activities.schedules for template splits (WP-B3)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     scheduleValidUntilVersion,
		Description: scheduleValidUntilDescription,
		// 1.15.34 added the template extension columns (week_pattern,
		// calendar_period_id) to activities.schedules — valid_until extends
		// that same recurrence row.
		DependsOn: []string{templateExtensionsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.123: Adding valid_until to activities.schedules...")

			// Exclusive end of a schedule's recurrence: the schedule no longer
			// produces instances ON or AFTER this date (same convention as
			// activities.student_enrollments.valid_until). NULL = open-ended.
			if _, err := db.NewRaw(`
				ALTER TABLE activities.schedules
					ADD COLUMN IF NOT EXISTS valid_until DATE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding valid_until to activities.schedules: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.123: Removing valid_until from activities.schedules...")

			if _, err := db.NewRaw(`
				ALTER TABLE activities.schedules
					DROP COLUMN IF EXISTS valid_until;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping valid_until from activities.schedules: %w", err)
			}

			return nil
		},
	)
}
