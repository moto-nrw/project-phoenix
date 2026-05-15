package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	timeframesUseTimeWithoutTimezoneVersion     = "1.15.50"
	timeframesUseTimeWithoutTimezoneDescription = "Store schedule timeframes as timezone-free clock times"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     timeframesUseTimeWithoutTimezoneVersion,
		Description: timeframesUseTimeWithoutTimezoneDescription,
		DependsOn: []string{
			ScheduleTimeframesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return timeframesUseTimeWithoutTimezoneUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return timeframesUseTimeWithoutTimezoneDown(ctx, db)
		},
	)
}

func timeframesUseTimeWithoutTimezoneUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.50: Converting schedule.timeframes clock values to TIME...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.timeframes
			ALTER COLUMN start_time TYPE TIME WITHOUT TIME ZONE
				USING (start_time AT TIME ZONE 'UTC')::time,
			ALTER COLUMN end_time TYPE TIME WITHOUT TIME ZONE
				USING CASE
					WHEN end_time IS NULL THEN NULL
					ELSE (end_time AT TIME ZONE 'UTC')::time
				END;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed converting schedule.timeframes to TIME: %w", err)
	}
	return nil
}

func timeframesUseTimeWithoutTimezoneDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.50: Converting schedule.timeframes clock values back to TIMESTAMPTZ...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.timeframes
			ALTER COLUMN start_time TYPE TIMESTAMPTZ
				USING ('2000-01-01'::date + start_time) AT TIME ZONE 'UTC',
			ALTER COLUMN end_time TYPE TIMESTAMPTZ
				USING CASE
					WHEN end_time IS NULL THEN NULL
					ELSE ('2000-01-01'::date + end_time) AT TIME ZONE 'UTC'
				END;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed converting schedule.timeframes back to TIMESTAMPTZ: %w", err)
	}
	return nil
}
