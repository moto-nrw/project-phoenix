package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffShiftSeriesRetainedOccurrenceVersion     = "1.15.242"
	staffShiftSeriesRetainedOccurrenceDescription = "Track retained current-day rows for staff shift series splits"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftSeriesRetainedOccurrenceVersion,
		Description: staffShiftSeriesRetainedOccurrenceDescription,
		DependsOn:   []string{parentSuggestionsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.242: Tracking retained staff-shift series occurrences...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shift_series
					ADD COLUMN IF NOT EXISTS retained_occurrence_shift_id BIGINT;

				ALTER TABLE schedule.staff_shift_series
					DROP CONSTRAINT IF EXISTS fk_staff_shift_series_retained_occurrence,
					ADD CONSTRAINT fk_staff_shift_series_retained_occurrence
						FOREIGN KEY (tenant_id, retained_occurrence_shift_id)
						REFERENCES schedule.staff_shifts(tenant_id, id)
						ON DELETE SET NULL (retained_occurrence_shift_id);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding retained staff-shift series occurrence: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.242: Dropping retained staff-shift series occurrences...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shift_series
					DROP CONSTRAINT IF EXISTS fk_staff_shift_series_retained_occurrence,
					DROP COLUMN IF EXISTS retained_occurrence_shift_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping retained staff-shift series occurrence: %w", err)
			}
			return nil
		},
	)
}
