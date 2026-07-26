package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	instanceStudentsManualStatusVersion     = "1.15.208"
	instanceStudentsManualStatusDescription = "Add manual_status_at to schedule.instance_students so a hand-set attendance status survives block completion instead of being relabelled a non-booking (#1747)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     instanceStudentsManualStatusVersion,
		Description: instanceStudentsManualStatusDescription,
		DependsOn:   []string{instanceStudentsNotScheduledVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return instanceStudentsManualStatusUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return instanceStudentsManualStatusDown(ctx, db)
		},
	)
}

// instanceStudentsManualStatusUp records that a human decided an attendance
// row's status by hand.
//
// Ending a block spares the children the care plan does not book and stamps
// not_scheduled on them (#1747). The population it picks is "still 'expected'",
// which is also what the attendance PATCH writes when staff set an unbooked
// slot back to expected — "this child IS coming, the plan is wrong". Without a
// way to tell the two apart, completion stamps the marker over that decision
// and the row disappears from the completed-instance views, the history and the
// exports.
//
// manual_status_at is written by the PATCH endpoint only, on the write that
// sets `status`. It says nothing about which status was chosen; it says a
// person chose it. Completion reads it as "hands off": such a row keeps the
// ordinary expected → absent path and stays visible.
//
// NULL for every existing row is correct — no legacy row can prove it was
// hand-decided, and treating an unknown as automatic is the state those rows
// are already in.
//
// Numbered 1.15.208 rather than 1.15.207 on purpose: the 207 slot was used
// during review by a backfill that has since been dropped, and bun keys applied
// migrations on the numeric filename prefix. Reusing the number would leave any
// database that ran the old file without this column, silently.
func instanceStudentsManualStatusUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.208: Adding manual_status_at column to schedule.instance_students (#1747)...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.instance_students
		ADD COLUMN IF NOT EXISTS manual_status_at TIMESTAMPTZ;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding manual_status_at column: %w", err)
	}

	return nil
}

func instanceStudentsManualStatusDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.208: Dropping manual_status_at column from schedule.instance_students...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.instance_students DROP COLUMN IF EXISTS manual_status_at;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping manual_status_at column: %w", err)
	}

	return nil
}
