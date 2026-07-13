package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffShiftFlexibleChangesVersion     = "1.15.191"
	staffShiftFlexibleChangesDescription = "Add flexible daily-change columns to schedule.staff_shifts (#1841): cancelled + change_reason so a shift can be left open with a 'why', and origin_shift_id so a replacement shift points at the shift it covers (1:1 substitution and split across several people)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftFlexibleChangesVersion,
		Description: staffShiftFlexibleChangesDescription,
		DependsOn:   []string{staffShiftSeriesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.191: Adding flexible-change columns to schedule.staff_shifts...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
				ADD COLUMN IF NOT EXISTS cancelled BOOLEAN NOT NULL DEFAULT FALSE,
				ADD COLUMN IF NOT EXISTS change_reason TEXT,
				ADD COLUMN IF NOT EXISTS origin_shift_id BIGINT
					REFERENCES schedule.staff_shifts(id) ON DELETE SET NULL;
				COMMENT ON COLUMN schedule.staff_shifts.cancelled IS
					'Shift does not take place (staff absent / gap deliberately left open, #1841). Excluded from planned minutes and auto-checkout.';
				COMMENT ON COLUMN schedule.staff_shifts.change_reason IS
					'Optional reason for a flexible daily change: why the times moved, why it was cancelled, or why this replacement was entered (#1841).';
				COMMENT ON COLUMN schedule.staff_shifts.origin_shift_id IS
					'When set, this shift covers another (cancelled) shift as a replacement; several replacements pointing at the same origin split one gap across people (#1841). ON DELETE SET NULL keeps the cover if the origin is removed.';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding flexible-change columns to schedule.staff_shifts: %w", err)
			}
			// Partial index: replacement lookups (all covers of a given gap) only
			// ever query the small set of rows that actually carry an origin.
			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_staff_shifts_origin_shift_id
					ON schedule.staff_shifts (origin_shift_id)
					WHERE origin_shift_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating origin_shift_id index: %w", err)
			}
			// A cancelled shift does not take place (#1841). The overlap check
			// already lets a fresh 08:00 shift reuse the window of a cancelled
			// 08:00 shift for the same staff/date, but the original
			// uniq_staff_shift_start UNIQUE (tenant_id, staff_id, date, start_time)
			// still counts the cancelled row and rejects the insert. Replace the
			// table constraint with a partial UNIQUE index that ignores cancelled
			// rows, so only shifts that actually happen contend for a start time.
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS uniq_staff_shift_start;
				CREATE UNIQUE INDEX IF NOT EXISTS uniq_staff_shift_start_active
					ON schedule.staff_shifts (tenant_id, staff_id, date, start_time)
					WHERE NOT cancelled;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed replacing uniq_staff_shift_start with a cancellation-aware index: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.191...")
			// Restore the plain start-time uniqueness (best-effort; fails if
			// cancellation reuse has produced duplicate active/cancelled start
			// times, which only exists once this feature has run).
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS schedule.uniq_staff_shift_start_active;
				ALTER TABLE schedule.staff_shifts
					ADD CONSTRAINT uniq_staff_shift_start UNIQUE (tenant_id, staff_id, date, start_time);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed restoring uniq_staff_shift_start: %w", err)
			}
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS schedule.idx_staff_shifts_origin_shift_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping origin_shift_id index: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
				DROP COLUMN IF EXISTS origin_shift_id,
				DROP COLUMN IF EXISTS change_reason,
				DROP COLUMN IF EXISTS cancelled;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping flexible-change columns: %w", err)
			}
			return nil
		},
	)
}
