package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	absenceAllowanceAlwaysBlockVersion     = "1.15.388"
	absenceAllowanceAlwaysBlockDescription = "Drop the overrun policy of staff absence types: allowances never go negative (#3256)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     absenceAllowanceAlwaysBlockVersion,
		Description: absenceAllowanceAlwaysBlockDescription,
		DependsOn:   []string{customLeaveAllowancesVersion},
	})
	Migrations.MustRegister(absenceAllowanceAlwaysBlockUp, absenceAllowanceAlwaysBlockDown)
}

// absenceAllowanceAlwaysBlockUp removes the per-type choice between warning
// and blocking. A booking above a school-defined allowance is now always
// rejected, so the column has nothing left to decide.
func absenceAllowanceAlwaysBlockUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_overrun_policy,
			DROP COLUMN IF EXISTS overrun_policy;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop staff absence type overrun policy: %w", err)
	}
	return nil
}

// absenceAllowanceAlwaysBlockDown restores the column with the behavior the
// application now enforces, so a rollback does not reopen overbooking.
func absenceAllowanceAlwaysBlockDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE active.staff_absence_types
			ADD COLUMN IF NOT EXISTS overrun_policy VARCHAR(10) NOT NULL DEFAULT 'block';
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_overrun_policy;
		ALTER TABLE active.staff_absence_types
			ADD CONSTRAINT chk_sat_overrun_policy CHECK (overrun_policy IN ('warn', 'block'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("restore staff absence type overrun policy: %w", err)
	}
	return nil
}
