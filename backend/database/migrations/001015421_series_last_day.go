package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	seriesLastDayVersion     = "1.15.421"
	seriesLastDayDescription = "Recurring series may end before their planning period (#3594)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     seriesLastDayVersion,
		Description: seriesLastDayDescription,
		DependsOn:   []string{seriesIncludeClosingDaysVersion}, // preserves ladder order
	})

	Migrations.MustRegister(seriesLastDayUp, seriesLastDayDown)
}

// seriesLastDayUp adds the optional last day of a recurring series (#3594),
// e.g. a holiday-care week. Materialization plans no occurrence after it.
// It is deliberately not schedules.valid_until: a set valid_until marks a
// segment capped by a split or an end, which the active template CRUD hides.
// NULL = the series runs until its planning period ends.
func seriesLastDayUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS series_last_day DATE;
	`)
	if err != nil {
		return fmt.Errorf("error adding series_last_day to activities.groups: %w", err)
	}
	return nil
}

func seriesLastDayDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS series_last_day;
	`)
	if err != nil {
		return fmt.Errorf("error removing series_last_day from activities.groups: %w", err)
	}
	return nil
}
