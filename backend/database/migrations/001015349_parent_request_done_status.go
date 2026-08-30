package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	parentRequestDoneVersion     = "1.15.349"
	parentRequestDoneDescription = "Allow the terminal 'done' status on the four parent request queues (#2267)"
)

// parentRequestDoneStatus closes a request that only covers days already
// gone: approving it would write nothing and rejecting it tells the family
// their wish was refused, which is not what happened. Named once so the model
// constants and this migration cannot drift.
const parentRequestDoneStatus = "done"

func init() {
	MigrationRegistry.Register(&Migration{
		Version: parentRequestDoneVersion, Description: parentRequestDoneDescription,
		DependsOn: []string{parentRequestEventsVersion, studentCareExitsVersion},
	})
	Migrations.MustRegister(parentRequestDoneUp, parentRequestDoneDown)
}

// parentRequestDoneStatusValues is the full value set after 1.15.319 widened
// each table with care_ended. Re-listed per table because the sets differ.
var parentRequestDoneStatusValues = []struct {
	Schema   string
	Table    string
	Existing []string
}{
	{"users", "student_data_change_requests", []string{"auto_applied", "pending", "approved", "rejected", careEndedStatus}},
	{"active", "excused_absence_requests", []string{"pending", "approved", "rejected", "withdrawn", careEndedStatus}},
	{"enrollment", "offering_change_requests", []string{"pending", "approved", "rejected", "withdrawn", careEndedStatus}},
	{"schedule", "care_schedule_change_requests", []string{"pending", "approved", "rejected", "withdrawn", careEndedStatus}},
}

func parentRequestDoneUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", slog.String("migration", parentRequestDoneVersion))
	for _, target := range parentRequestDoneStatusValues {
		if err := widenStatusCheck(ctx, db, target.Schema, target.Table,
			append(target.Existing, parentRequestDoneStatus)); err != nil {
			return err
		}
	}
	return nil
}

// parentRequestDoneDown refuses rather than rewriting a staff decision: a
// 'done' row narrowed back would have to become something the office never
// chose.
func parentRequestDoneDown(ctx context.Context, db *bun.DB) error {
	// Every table is checked before any is narrowed: a rollback that half
	// applied would leave the queues on different constraint sets.
	for _, target := range parentRequestDoneStatusValues {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`
			DO $$
			BEGIN
				IF to_regclass('%[1]s.%[2]s') IS NULL THEN
					RETURN;
				END IF;
				IF EXISTS (SELECT 1 FROM %[1]s.%[2]s WHERE status = '%[3]s') THEN
					RAISE EXCEPTION 'cannot narrow %[1]s.%[2]s.status while %[3]s rows exist';
				END IF;
			END $$;
		`, target.Schema, target.Table, parentRequestDoneStatus)); err != nil {
			return err
		}
	}
	for _, target := range parentRequestDoneStatusValues {
		if err := widenStatusCheck(ctx, db, target.Schema, target.Table, target.Existing); err != nil {
			return err
		}
	}
	return nil
}
