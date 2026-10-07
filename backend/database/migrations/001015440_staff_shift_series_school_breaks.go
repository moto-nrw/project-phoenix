package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffShiftSeriesSchoolBreaksVersion     = "1.15.440"
	staffShiftSeriesSchoolBreaksDescription = "Shift series may opt into holidays and closing days (#3820)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftSeriesSchoolBreaksVersion,
		Description: staffShiftSeriesSchoolBreaksDescription,
		DependsOn:   []string{studentNotesVersion}, // preserves ladder order
	})

	Migrations.MustRegister(staffShiftSeriesSchoolBreaksUp, staffShiftSeriesSchoolBreaksDown)
}

// staffShiftSeriesSchoolBreaksUp adds the per-series opt-in for school
// breaks (#3820). Materialization skips Ferien (active holiday calendar
// periods) and closing days; a series for staff who also work in the
// holidays sets the flag. Statutory holidays stay skipped either way. The
// default false applies the new rule to every existing series from its next
// re-plan on; shifts already materialized stay untouched.
func staffShiftSeriesSchoolBreaksUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.staff_shift_series
			ADD COLUMN IF NOT EXISTS include_school_breaks BOOLEAN NOT NULL DEFAULT false;
	`)
	if err != nil {
		return fmt.Errorf("error adding include_school_breaks to schedule.staff_shift_series: %w", err)
	}
	return nil
}

func staffShiftSeriesSchoolBreaksDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.staff_shift_series
			DROP COLUMN IF EXISTS include_school_breaks;
	`)
	if err != nil {
		return fmt.Errorf("error removing include_school_breaks from schedule.staff_shift_series: %w", err)
	}
	return nil
}
