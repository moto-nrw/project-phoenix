package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.889",
		Description: "Distinguish deliberate shift-series deviations from sickness-only detachments",
		DependsOn:   []string{holidayClosureNoticesVersion},
	})
	Migrations.MustRegister(staffShiftSeriesOverrideUp, staffShiftSeriesOverrideDown)
}

func staffShiftSeriesOverrideUp(ctx context.Context, db *bun.DB) error {
	// Existing detachments have no reliable provenance. Preserve them
	// conservatively rather than risk deleting an administrator's edit.
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.staff_shifts ADD COLUMN series_override BOOLEAN NOT NULL DEFAULT FALSE;
		UPDATE schedule.staff_shifts SET series_override = TRUE WHERE detached;
	`)
	if err != nil {
		return fmt.Errorf("add staff shift series override provenance: %w", err)
	}
	return nil
}

func staffShiftSeriesOverrideDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE schedule.staff_shifts DROP COLUMN series_override`)
	return err
}
