package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffScheduleRotationAnchorVersion     = "1.15.203"
	staffScheduleRotationAnchorDescription = "Add rotation_anchor_date to config.staff_work_schedules (#1842): the rotation anchor lived only on users.staff and was overwritten on every schedule change, so editing a multi-week schedule retroactively flipped the A/B parity of already-closed weeks. Each schedule version now carries the anchor it was written with."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffScheduleRotationAnchorVersion,
		Description: staffScheduleRotationAnchorDescription,
		DependsOn:   []string{sickAbsenceProvenanceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.203: Adding rotation_anchor_date to config.staff_work_schedules (#1842)...")
			if _, err := db.NewRaw(`
				ALTER TABLE config.staff_work_schedules
					ADD COLUMN IF NOT EXISTS rotation_anchor_date DATE;
				COMMENT ON COLUMN config.staff_work_schedules.rotation_anchor_date IS
					'Rotation anchor this schedule version was written with (#1842). Immutable: later schedule changes write a new version with its own anchor instead of moving this one, so historical A/B weeks keep the parity they were computed with. NULL only for single-week versions (rotation_length = 1), which have no A/B parity an anchor could shift; readers then fall back to users.staff.rotation_anchor_date and finally to the earliest valid_from.';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding rotation_anchor_date to config.staff_work_schedules: %w", err)
			}

			// Freeze today's effective anchor onto existing rows. The staff-level
			// anchor is all the history we have — the true anchor of an older
			// version is unrecoverable — but writing it down now means the NEXT
			// schedule change no longer rewrites the past.
			if _, err := db.NewRaw(`
				UPDATE config.staff_work_schedules AS sws
				SET rotation_anchor_date = s.rotation_anchor_date
				FROM users.staff AS s
				WHERE s.id = sws.staff_id
					AND s.rotation_anchor_date IS NOT NULL
					AND sws.rotation_anchor_date IS NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling rotation_anchor_date from users.staff: %w", err)
			}

			// Rotational versions whose staff row has no anchor at all (the
			// schedule request has always allowed omitting it) are still NULL
			// here and would keep falling back to users.staff at read time —
			// so the next template assignment, which always writes a
			// staff-level anchor, would re-parity their A/B weeks and move an
			// already-closed Saldo. That is exactly what this migration
			// exists to prevent, so pin their pre-migration anchor now:
			// ResolveScheduleAnchor's last fallback was the earliest
			// valid_from of the version, and ReplaceSchedule stamps one
			// valid_from across every row of a version, so the row's own
			// valid_from IS that version start. Single-week versions are left
			// NULL on purpose: with rotation_length = 1 the week index is
			// always 0, so they have no parity an anchor could shift.
			if _, err := db.NewRaw(`
				UPDATE config.staff_work_schedules AS sws
				SET rotation_anchor_date = sws.valid_from
				WHERE sws.rotation_anchor_date IS NULL
					AND EXISTS (
						SELECT 1
						FROM config.staff_work_schedules AS peer
						WHERE peer.staff_id = sws.staff_id
							AND peer.valid_from = sws.valid_from
							AND peer.rotation_length > 1
					);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling rotation_anchor_date from version valid_from: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.203...")
			if _, err := db.NewRaw(`
				ALTER TABLE config.staff_work_schedules
					DROP COLUMN IF EXISTS rotation_anchor_date;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping rotation_anchor_date from config.staff_work_schedules: %w", err)
			}
			return nil
		},
	)
}
