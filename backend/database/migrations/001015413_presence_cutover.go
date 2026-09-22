package migrations

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

const presenceCutoverVersion = "1.15.413"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     presenceCutoverVersion,
		Description: "Cut over Timetable planning and Presence session/attendance storage with previous-image compatibility (#2762)",
		DependsOn: []string{
			presenceBackfillCheckpointVersion,
			staffOwnerCutoverVersion, // the session started_by key already names the membership
			"1.15.412",               // preserves ladder order
		},
		// A school with timetable rows but no completed backfill pass cannot
		// be switched: the final delta closes the small remainder, not a
		// backfill that never ran. Report it before the deployment stops the
		// application instead of rolling the release back mid-migration.
		Precondition: presenceCutoverPrecondition,
	})
	Migrations.MustRegister(presenceCutoverUp, presenceCutoverDown)
}

// presenceCutoverUp switches the authority for the execution of an activity
// instance (status active/completed, live group, actor, timestamps, completion
// snapshot) to active.activity_sessions and for participant attendance to
// active.activity_session_attendance, both owned by Student Presence.
// schedule.activity_instances and schedule.instance_students keep the planning
// fields and remain the base tables under their old names: the previous image
// materializes and books participants with INSERT ... ON CONFLICT, which
// PostgreSQL refuses through a trigger-updatable view, so the rollback shape
// cannot be a view here. Instead the old execution and attendance columns
// stay in place as a rollback-only mirror of the owner tables.
//
// The final delta, the per-tenant verification and the compatibility install
// share one transaction and one write lock, so no write can land between "the
// targets reproduce the old columns" and "the old columns are only a mirror".
// Running it again after the switch is a no-op resume.
func presenceCutoverUp(ctx context.Context, db *bun.DB) error {
	installed, err := presenceCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	return finalizePresenceStorage(ctx, db, installPresenceCompatibility)
}

func presenceCutoverDown(context.Context, *bun.DB) error {
	return errors.New("presence cutover rollback deploys the previous application image against the retained compatibility triggers; the old authority is not restored here")
}
