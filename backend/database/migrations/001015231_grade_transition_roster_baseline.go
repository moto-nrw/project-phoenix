package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	// 1.15.230 is this branch's previous migration; see the version note in
	// 001015229 for why this branch sits above development's 1.15.228.
	gradeTransitionRosterBaselineVersion     = "1.15.231"
	gradeTransitionRosterBaselineDescription = "Record which timetable instances already existed when a grade transition was applied"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     gradeTransitionRosterBaselineVersion,
		Description: gradeTransitionRosterBaselineDescription,
		DependsOn:   []string{gradeTransitionRosterRemovalsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// The revert must tell two kinds of upcoming instance apart: the ones
			// that existed when the transition was applied (whose rosters the
			// removal ledger owns) and the ones the materializer built WHILE the
			// children were alumni (which carry no ledger row and must be filled
			// from enrollments).
			//
			// Comparing activity_instances.created_at against applied_at cannot do
			// that: created_at defaults to current_timestamp, which in PostgreSQL
			// is the TRANSACTION START time. A materialization transaction that
			// started before the apply committed and then blocked on the tenant
			// grade-transition lock stamps its rows with a timestamp from before
			// the apply, even though it inserted them afterwards — and the revert
			// would classify exactly those instances as pre-transition and leave
			// the restored child off their rosters for good.
			//
			// The sequence-assigned id is the ordering marker that does reflect
			// insertion order (nextval runs at INSERT, not at BEGIN), so the apply
			// records the highest instance id it can see and the revert fills only
			// instances above it. Nullable: transitions applied before this
			// migration have no marker and their revert skips the enrollment fill
			// entirely rather than reconstructing rosters from a guess.
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transitions
				ADD COLUMN IF NOT EXISTS roster_baseline_instance_id BIGINT;

				COMMENT ON COLUMN education.grade_transitions.roster_baseline_instance_id IS
					'Highest schedule.activity_instances.id visible when this transition was applied. Instances above it were materialized during the alumnus window and are refilled from enrollments on revert (#405).';
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE education.grade_transitions
				DROP COLUMN IF EXISTS roster_baseline_instance_id
			`)
			return err
		},
	)
}
