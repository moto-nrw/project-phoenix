package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	// development occupies everything up to and including 1.15.226 (staff
	// balance adjustments, month balance snapshots, audit append-only grants,
	// nullable attendance staff), so this branch's four migrations start above
	// it at 1.15.227.
	//
	// Renumbered once already: 1.15.222 and 1.15.226 were free when this branch
	// was written and were taken on development since. A duplicate version
	// panics SafeMigrationMap.Register at package init — the binary would not
	// boot after the merge — and, worse, bun tracks applied migrations by
	// FILENAME PREFIX, so a database that ran development's file first silently
	// skips ours and ends up without the column while reporting success.
	// Re-check this range against development before merging.
	gradeTransitionHistoryFromStatusVersion     = "1.15.227"
	gradeTransitionHistoryFromStatusDescription = "Record each graduate's pre-transition lifecycle status so a revert restores it exactly"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     gradeTransitionHistoryFromStatusVersion,
		Description: gradeTransitionHistoryFromStatusDescription,
		// Depends on the grade_transition_history table.
		DependsOn: []string{GradeTransitionsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// from_status snapshots the student's status at apply time so a
			// revert can restore pending / inactive students to what they were
			// instead of blanket-activating every graduate. Nullable: history
			// rows written before this migration carry no snapshot and the
			// revert falls back to 'active'.
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transition_history
				ADD COLUMN IF NOT EXISTS from_status VARCHAR(20)
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transition_history
				DROP COLUMN IF EXISTS from_status
			`)
			return err
		},
	)
}
