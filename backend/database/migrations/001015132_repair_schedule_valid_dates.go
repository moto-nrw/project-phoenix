package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repairScheduleValidDatesVersion     = "1.15.132"
	repairScheduleValidDatesDescription = "Repair activities.schedules valid_until/valid_from columns for databases where 1.15.126 was marked applied before schema changes landed."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairScheduleValidDatesVersion,
		Description: repairScheduleValidDatesDescription,
		DependsOn:   []string{scheduleValidUntilVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.132: Repairing activities.schedules valid date columns...")

			if _, err := db.NewRaw(`
				ALTER TABLE activities.schedules
					ADD COLUMN IF NOT EXISTS valid_until DATE;
				ALTER TABLE activities.schedules
					ADD COLUMN IF NOT EXISTS valid_from DATE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed repairing activities.schedules valid date columns: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.132: no-op; columns are owned by migration 1.15.126.")
			return nil
		},
	)
}
