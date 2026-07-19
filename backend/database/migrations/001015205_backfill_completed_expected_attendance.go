package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillCompletedExpectedAttendanceVersion     = "1.15.205"
	backfillCompletedExpectedAttendanceDescription = "Flip legacy 'expected' attendance rows on completed timetable instances to 'absent' — the absence the old force-start path never wrote (#1747)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillCompletedExpectedAttendanceVersion,
		Description: backfillCompletedExpectedAttendanceDescription,
		DependsOn:   []string{refreshTokenRotationRecoveryVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillCompletedExpectedAttendanceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return backfillCompletedExpectedAttendanceDown(ctx, db)
		},
	)
}

// backfillCompletedExpectedAttendanceUp writes the absences an old code path
// forgot.
//
// From #1747 on, completing an instance finalizes its attendance first: every
// genuinely expected child flips to 'absent', and only children the care plan
// does not place there that day keep their 'expected' row — stamped with the
// not_scheduled marker added in 1.15.206.
//
// Rows written BEFORE that change never went through that step. The
// force-start path completed instances through the instance repository
// directly and left every expected row untouched, so children who were
// genuinely expected and did not come still sit there as 'expected' — an
// absence nobody recorded, on a day that is long over.
//
// Every completed+expected row existing at migration time is by definition
// legacy (the finalizing code ships with this deploy), so flipping all of them
// to 'absent' restores what the normal completion path would have written.
// They get no marker: 1.15.206 defaults not_scheduled to FALSE, which is
// correct — nothing in the old data claimed these children were unbooked.
func backfillCompletedExpectedAttendanceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.205: Backfilling legacy 'expected' attendance on completed timetable instances (#1747)...")

	// Cancelled instances are deliberately untouched: their 'expected' rows
	// stay expected (the instance never ran, so 'absent' would be a misclaim)
	// and the readers keep showing them — the instance status carries that
	// story, not the attendance row.
	res, err := db.NewRaw(`
		UPDATE schedule.instance_students AS student
		SET status = 'absent',
			updated_at = NOW()
		FROM schedule.activity_instances AS instance
		WHERE instance.id = student.instance_id
			AND instance.tenant_id = student.tenant_id
			AND instance.status = 'completed'
			AND student.status = 'expected';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling expected attendance on completed instances: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil {
		fmt.Printf("Migration 1.15.205: repaired %d attendance rows\n", affected)
	}

	return nil
}

// backfillCompletedExpectedAttendanceDown is a no-op: the backfill merges
// legacy rows into the same 'absent' state the regular completion path writes,
// and nothing records which rows it touched. Reverting them all to 'expected'
// would destroy genuine absences.
func backfillCompletedExpectedAttendanceDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.205: no-op for legacy attendance repair")
	return nil
}
