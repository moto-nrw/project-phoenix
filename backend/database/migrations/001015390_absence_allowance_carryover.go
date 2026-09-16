package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	absenceAllowanceCarryoverVersion     = "1.15.390"
	absenceAllowanceCarryoverDescription = "Let the rest of a staff absence allowance stay usable into the following year (#3257)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     absenceAllowanceCarryoverVersion,
		Description: absenceAllowanceCarryoverDescription,
		DependsOn:   []string{absenceAllowanceAlwaysBlockVersion},
	})
	Migrations.MustRegister(absenceAllowanceCarryoverUp, absenceAllowanceCarryoverDown)
}

// absenceAllowanceCarryoverUp stores, per school-defined absence type, the
// day and month of the following year until which a yearly rest stays usable
// ("03-31"). NULL keeps the previous behavior: the rest expires on 31.12.
// February 29 is rejected by the application so every year has the date.
func absenceAllowanceCarryoverUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE active.staff_absence_types
			ADD COLUMN IF NOT EXISTS carryover_until CHAR(5);
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_carryover_until;
		ALTER TABLE active.staff_absence_types
			ADD CONSTRAINT chk_sat_carryover_until CHECK (
				carryover_until IS NULL
				OR carryover_until ~ '^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'
			);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add staff absence type carryover: %w", err)
	}
	return nil
}

func absenceAllowanceCarryoverDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_carryover_until,
			DROP COLUMN IF EXISTS carryover_until;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop staff absence type carryover: %w", err)
	}
	return nil
}
