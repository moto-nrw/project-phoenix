package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillCompletedExpectedAttendanceVersion     = "1.15.205"
	backfillCompletedExpectedAttendanceDescription = "Flip legacy 'expected' attendance rows on completed timetable instances to 'absent' so a surviving 'expected' row can carry the #1747 'nicht eingeplant' marker unambiguously."
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

// backfillCompletedExpectedAttendanceUp repairs the rows that would otherwise
// be misread by the #1747 marker.
//
// From #1747 on, completing an instance finalizes its attendance first: every
// genuinely expected child flips to 'absent', and only children the care plan
// does not place there that day keep their 'expected' row. Readers (attendance
// history, week view, exports) therefore treat 'expected' on a COMPLETED
// instance as "war an dem Tag nicht eingeplant" and hide it.
//
// Rows written BEFORE that change do not carry that meaning. The force-start
// path completed instances through the instance repository directly and left
// every expected row untouched, so genuinely expected children — real absences
// — sit in the same shape. Without this repair the new predicate would silently
// erase those absences from the history and the exports.
//
// Every completed+expected row existing at migration time is by definition
// legacy (the marker-writing code ships with this deploy), so flipping all of
// them to 'absent' restores what the normal completion path would have written
// and leaves the marker unambiguous from here on.
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
