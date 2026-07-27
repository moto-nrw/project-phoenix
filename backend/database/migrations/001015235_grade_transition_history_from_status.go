package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	// development occupies everything up to and including 1.15.234, so this
	// branch's four migrations start above it at 1.15.235.
	//
	// Renumbered four times already: 1.15.222/1.15.226 were free when this
	// branch was written, then 1.15.227/1.15.228, then 1.15.229, then
	// 1.15.234 — development took every one of them while this branch was
	// open. A duplicate version panics SafeMigrationMap.Register at package
	// init — the binary would not boot after the merge, which is what CI
	// caught — and, worse, bun tracks applied migrations by FILENAME PREFIX,
	// so a database that ran development's file first silently skips ours and
	// ends up without the column while reporting success. That is not
	// hypothetical: the local test DB recorded 001015227-001015230 from this
	// branch and consequently never created audit.student_field_edits.
	//
	// Renumber THIS branch only. development's own migrations keep their
	// published versions: the last round moved its push_subscriptions file to
	// 1.15.238, development had meanwhile settled the same collision at
	// 1.15.234, and the merge carried both copies of the file into one
	// package — a redeclaration that broke every backend job.
	//
	gradeTransitionHistoryFromStatusVersion     = "1.15.235"
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
