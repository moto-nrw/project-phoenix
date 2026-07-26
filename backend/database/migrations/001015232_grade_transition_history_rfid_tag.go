package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	// Last of this branch's four migrations; see the version note in 001015227
	// for why they sit above development's 1.15.228.
	gradeTransitionHistoryRFIDTagVersion     = "1.15.232"
	gradeTransitionHistoryRFIDTagDescription = "Ledger the RFID tag a graduate was holding so graduation can release it and a revert can hand it back"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     gradeTransitionHistoryRFIDTagVersion,
		Description: gradeTransitionHistoryRFIDTagDescription,
		// Depends on the from_status column migration, i.e. on the same table.
		DependsOn: []string{gradeTransitionHistoryFromStatusVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Graduation is a soft delete: the child keeps their person row, and
			// with it users.persons.tag_id — so the physical bracelet stayed
			// bound to a departed child while every staff-facing route 404s on
			// them. rfid_tag snapshots the released tag at apply time so the
			// bracelet can be cleared for reissue and a revert can re-link it.
			// Nullable: graduates without a tag, and history rows written before
			// this migration, carry NULL and the revert re-links nothing.
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transition_history
				ADD COLUMN IF NOT EXISTS rfid_tag TEXT
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transition_history
				DROP COLUMN IF EXISTS rfid_tag
			`)
			return err
		},
	)
}
