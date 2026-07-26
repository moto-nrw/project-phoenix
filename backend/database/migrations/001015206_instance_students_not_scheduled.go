package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	instanceStudentsNotScheduledVersion     = "1.15.206"
	instanceStudentsNotScheduledDescription = "Add not_scheduled marker column to schedule.instance_students so the #1747 'war an dem Tag nicht eingeplant' fact is persisted instead of inferred from status='expected'"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     instanceStudentsNotScheduledVersion,
		Description: instanceStudentsNotScheduledDescription,
		DependsOn:   []string{backfillCompletedExpectedAttendanceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return instanceStudentsNotScheduledUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return instanceStudentsNotScheduledDown(ctx, db)
		},
	)
}

// instanceStudentsNotScheduledUp gives the #1747 non-booking marker its own
// column.
//
// The first cut of #1747 inferred the marker: "status = 'expected' on a
// COMPLETED instance means the child was never booked into care that day".
// That overloads a column other writers own. Resetting attendance through
// PATCH /instances/{id}/students/{id} writes 'expected' back and would turn a
// genuinely expected child into a phantom non-booking that vanishes from the
// history and the exports; ApplyStatusDay would do the reverse and stamp a
// sick-report absence onto care that was never booked. Neither writer can be
// expected to know about a meaning that is not written down.
//
// not_scheduled is written by exactly the two paths that decide it (instance
// Complete and the nightly session-end bridge) and read by everything else.
//
// It starts FALSE for every existing row and stays that way: no backfill
// stamps it. Migration 1.15.205 repaired the legacy rows it had positive
// evidence for (a care plan that booked the child, already on file when the
// instance finished) by writing the missing absence. The complement cannot be
// repaired — concluding "was not booked" from a missing weekly row would read a
// deletion the plan tables never recorded as evidence. Those rows keep the
// plain 'expected' state the old path left them in.
func instanceStudentsNotScheduledUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.206: Adding not_scheduled column to schedule.instance_students (#1747)...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.instance_students
		ADD COLUMN IF NOT EXISTS not_scheduled BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding not_scheduled column: %w", err)
	}

	return nil
}

func instanceStudentsNotScheduledDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.206: Dropping not_scheduled column from schedule.instance_students...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.instance_students DROP COLUMN IF EXISTS not_scheduled;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping not_scheduled column: %w", err)
	}

	return nil
}
