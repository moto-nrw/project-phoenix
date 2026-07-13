package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffShiftFlexibleChangesVersion     = "1.15.192"
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
			fmt.Println("Migration 1.15.192: Adding flexible-change columns to schedule.staff_shifts...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
				ADD COLUMN IF NOT EXISTS cancelled BOOLEAN NOT NULL DEFAULT FALSE,
				ADD COLUMN IF NOT EXISTS change_reason TEXT,
				ADD COLUMN IF NOT EXISTS origin_shift_id BIGINT;
				COMMENT ON COLUMN schedule.staff_shifts.cancelled IS
					'Shift does not take place (staff absent / gap deliberately left open, #1841). Excluded from planned minutes and auto-checkout.';
				COMMENT ON COLUMN schedule.staff_shifts.change_reason IS
					'Optional reason for a flexible daily change: why the times moved, why it was cancelled, or why this replacement was entered (#1841).';
				COMMENT ON COLUMN schedule.staff_shifts.origin_shift_id IS
					'When set, this shift covers another (cancelled) shift as a replacement; several replacements pointing at the same origin split one gap across people (#1841). Tenant-scoped composite FK; ON DELETE SET NULL keeps the cover if the origin is removed.';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding flexible-change columns to schedule.staff_shifts: %w", err)
			}
			// The origin link must be tenant-safe: schedule.staff_shifts is
			// tenant-scoped, so a bare REFERENCES ...(id) would accept a cover
			// whose tenant differs from its origin (and ON DELETE SET NULL could
			// then null a row in another tenant). Mirror the shift-type / series
			// pattern: a UNIQUE (tenant_id, id) backs a composite FK on
			// (tenant_id, origin_shift_id), and SET NULL (origin_shift_id) clears
			// ONLY the cover reference — a plain SET NULL would also null the
			// shared NOT NULL tenant_id column and fail the delete.
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS uniq_staff_shifts_tenant_id;
				ALTER TABLE schedule.staff_shifts
					ADD CONSTRAINT uniq_staff_shifts_tenant_id UNIQUE (tenant_id, id);
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS fk_staff_shifts_origin;
				ALTER TABLE schedule.staff_shifts
					ADD CONSTRAINT fk_staff_shifts_origin
						FOREIGN KEY (tenant_id, origin_shift_id)
						REFERENCES schedule.staff_shifts(tenant_id, id)
						ON DELETE SET NULL (origin_shift_id);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding tenant-scoped origin_shift_id foreign key: %w", err)
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
			fmt.Println("Rolling back migration 1.15.192...")
			// The partial index this rollback replaces only forbade duplicate
			// start_times among NON-cancelled rows, so normal feature use can leave
			// a cancelled shift and an active shift (or several cancelled rows)
			// sharing (tenant_id, staff_id, date, start_time). The plain UNIQUE
			// constraint restored below counts every row, so those groups must be
			// collapsed to one row first — otherwise the ADD CONSTRAINT fails and
			// the rollback is impossible after the feature has run. Cancellation and
			// the origin link are being dropped here anyway, so keep the row that
			// actually takes place (prefer non-cancelled, then the lowest id) and
			// delete the rest; the tenant-scoped FK's ON DELETE SET NULL clears any
			// replacement pointer to a removed origin (also about to be dropped).
			if _, err := db.NewRaw(`
				DELETE FROM schedule.staff_shifts s
				USING schedule.staff_shifts keep
				WHERE s.tenant_id = keep.tenant_id
					AND s.staff_id = keep.staff_id
					AND s.date = keep.date
					AND s.start_time = keep.start_time
					AND s.id <> keep.id
					AND (
						(s.cancelled AND NOT keep.cancelled)
						OR (s.cancelled = keep.cancelled AND s.id > keep.id)
					);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed collapsing duplicate start_times before restoring uniq_staff_shift_start: %w", err)
			}
			// Restore the plain start-time uniqueness now that every
			// (tenant_id, staff_id, date, start_time) group holds a single row.
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
			// Drop the tenant-scoped origin FK and its backing unique before the
			// columns; DROP COLUMN would drop the FK anyway, but the UNIQUE
			// (tenant_id, id) is independent and must be removed explicitly.
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS fk_staff_shifts_origin;
				ALTER TABLE schedule.staff_shifts
					DROP CONSTRAINT IF EXISTS uniq_staff_shifts_tenant_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping tenant-scoped origin_shift_id foreign key: %w", err)
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
