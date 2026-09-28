package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	seriesIncludeClosingDaysVersion     = "1.15.420"
	seriesIncludeClosingDaysDescription = "Recurring series may opt into closing days (#3594)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     seriesIncludeClosingDaysVersion,
		Description: seriesIncludeClosingDaysDescription,
		DependsOn:   []string{staffTargetOverridesVersion}, // preserves ladder order
	})

	Migrations.MustRegister(seriesIncludeClosingDaysUp, seriesIncludeClosingDaysDown)
}

// seriesIncludeClosingDaysUp adds the per-series opt-in for closing days
// (#3594). Materialization skips closing days and statutory holidays; a
// series that exists for holiday care sets the flag so its occurrences keep
// landing on closing days. Holidays stay skipped either way. The default
// false applies the new skip rule to every existing series.
func seriesIncludeClosingDaysUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS include_closing_days BOOLEAN NOT NULL DEFAULT false;
	`)
	if err != nil {
		return fmt.Errorf("error adding include_closing_days to activities.groups: %w", err)
	}
	return nil
}

func seriesIncludeClosingDaysDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS include_closing_days;
	`)
	if err != nil {
		return fmt.Errorf("error removing include_closing_days from activities.groups: %w", err)
	}
	return nil
}
