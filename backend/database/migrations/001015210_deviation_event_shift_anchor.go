package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deviationEventShiftAnchorVersion     = "1.15.210"
	deviationEventShiftAnchorDescription = "Add audit.deviation_events.staff_shift_id: anchor for Dienstplan shift-move events (#1884)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     deviationEventShiftAnchorVersion,
		Description: deviationEventShiftAnchorDescription,
		DependsOn: []string{
			deviationEventsAuditVersion, // the table this column extends
			studentCompanionsVersion,    // latest development migration before this branch
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.210: Adding staff_shift_id anchor to audit.deviation_events...")
			if _, err := db.NewRaw(`
				-- A Dienstplan shift is not an activity slot, so shift-move events
				-- (#1884) cannot use the (activity_group_id, occurrence_date,
				-- start_time) slot anchor. SET NULL keeps the trail when the shift
				-- row is deleted; occurrence_date/start_time still snapshot the slot.
				ALTER TABLE audit.deviation_events
					ADD COLUMN IF NOT EXISTS staff_shift_id BIGINT
					REFERENCES schedule.staff_shifts(id) ON DELETE SET NULL;

				CREATE INDEX IF NOT EXISTS idx_deviation_events_staff_shift
					ON audit.deviation_events (staff_shift_id) WHERE staff_shift_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("add staff_shift_id to audit.deviation_events: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.210: Dropping audit.deviation_events.staff_shift_id...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS audit.idx_deviation_events_staff_shift;
				ALTER TABLE audit.deviation_events DROP COLUMN IF EXISTS staff_shift_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("drop staff_shift_id from audit.deviation_events: %w", err)
			}
			return nil
		},
	)
}
