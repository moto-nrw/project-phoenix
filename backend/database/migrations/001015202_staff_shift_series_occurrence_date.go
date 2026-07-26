package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffShiftSeriesOccurrenceDateVersion     = "1.15.202"
	staffShiftSeriesOccurrenceDateDescription = "Track the original recurrence date of materialized staff shifts"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftSeriesOccurrenceDateVersion,
		Description: staffShiftSeriesOccurrenceDateDescription,
		DependsOn:   []string{offeringAdjustmentsAppendOnlyVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.202: Tracking original staff-shift series occurrence dates...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					ADD COLUMN IF NOT EXISTS series_occurrence_date DATE;

				UPDATE schedule.staff_shifts
				SET series_occurrence_date = date
				WHERE series_id IS NOT NULL
				  AND series_occurrence_date IS NULL;

				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS ck_staff_shifts_series_occurrence_date,
					ADD CONSTRAINT ck_staff_shifts_series_occurrence_date
						CHECK (series_id IS NULL OR series_occurrence_date IS NOT NULL);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding staff shift series occurrence dates: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.202: Dropping staff-shift series occurrence dates...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS ck_staff_shifts_series_occurrence_date,
					DROP COLUMN IF EXISTS series_occurrence_date;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping staff shift series occurrence dates: %w", err)
			}
			return nil
		},
	)
}
