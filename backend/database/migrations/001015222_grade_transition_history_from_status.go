package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	// 1.15.219 through 1.15.221 are taken on development (staff balance
	// adjustments + absence comp-time type), so this branch's migration claims
	// the next unused version instead of panicking SafeMigrationMap.Register at
	// package init on merge.
	gradeTransitionHistoryFromStatusVersion     = "1.15.222"
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
