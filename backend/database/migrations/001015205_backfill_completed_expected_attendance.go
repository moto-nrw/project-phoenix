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

// legacyCareDayBookedPredicate is the SQL form of "a care-plan row that was
// already on file when this instance completed books this child into the OGS on
// its date" (#1747). It is written against the aliases `student`
// (schedule.instance_students) and `instance` (schedule.activity_instances).
// Only positive evidence is actionable: a booked row that stayed 'expected' is
// the absence the old force-completion path forgot. The complement is NOT the
// mirror image — see the note on the missing non-booking backfill below.
//
// It mirrors carePlans.statusFor, minus its CareDayUnknown branch: an exception
// on the date (timed = came, timeless = cancelled a booked day) or a weekly row
// on the date's ISO weekday means booked. "No weekly plan at all" is NOT
// evidence here — see legacyCarePlanSettledPredicate.
const legacyCareDayBookedPredicate = `(
			EXISTS (
				SELECT 1 FROM schedule.student_arrival_exceptions AS arrival_exception
				WHERE arrival_exception.student_id = student.student_id
					AND arrival_exception.tenant_id = student.tenant_id
					AND arrival_exception.exception_date = instance.date
			)
			OR EXISTS (
				SELECT 1 FROM schedule.student_pickup_exceptions AS pickup_exception
				WHERE pickup_exception.student_id = student.student_id
					AND pickup_exception.tenant_id = student.tenant_id
					AND pickup_exception.exception_date = instance.date
			)
			OR EXISTS (
				SELECT 1 FROM schedule.student_arrival_schedules AS arrival_plan
				WHERE arrival_plan.student_id = student.student_id
					AND arrival_plan.tenant_id = student.tenant_id
					AND arrival_plan.weekday = EXTRACT(ISODOW FROM instance.date)
			)
			OR EXISTS (
				SELECT 1 FROM schedule.student_pickup_schedules AS pickup_plan
				WHERE pickup_plan.student_id = student.student_id
					AND pickup_plan.tenant_id = student.tenant_id
					AND pickup_plan.weekday = EXTRACT(ISODOW FROM instance.date)
			)
		)`

// There is deliberately no companion migration stamping the not_scheduled
// marker on the legacy rows this one leaves behind (#1747 review).
//
// Doing so would have to read a MISSING weekly row as "was not booked that
// weekday", and the plan tables keep no history: a Monday entry deleted last
// week is byte-for-byte indistinguishable from one that never existed, and
// legacyCarePlanSettledPredicate cannot see a deletion — it can only inspect
// rows that are still there. Stamping on that basis would irreversibly hide a
// genuine historical attendance record behind a marker nobody can undo.
//
// So those rows keep their plain 'expected' state, which is exactly what the
// old force-completion path left behind. Every block completed from #1747 on
// records its own verdict at the time it ends, where the evidence is real.

// legacyCarePlanSettledPredicate is the guard that keeps the backfill from
// judging a historical day by today's care plan (#1747 review).
//
// The plan tables carry no validity interval: a weekly row added, retimed, or
// moved to another weekday LAST WEEK looks exactly like one that was on file
// when the instance completed months ago. Reading it as evidence about that day
// is how an irreversible migration invents an absence for a child who was not
// booked then, or stamps "war nicht eingeplant" on a genuine expectation.
//
// So the evidence has to predate the completion it is used to explain: no
// arrival/pickup row for this child, and no exception of theirs on the
// instance's date, may have been created or last changed after
// instance.completed_at. A NULL timestamp (the columns are only DEFAULT NOW())
// says nothing about when the row appeared and counts as unsettled.
//
// Deletions leave no trace anywhere, which is why the absence of a plan row is
// deliberately never treated as evidence: a plan deleted after the fact is
// indistinguishable from one that never existed. Rows whose care plan cannot be
// judged this way are simply left as they are — unchanged 'expected' rows are
// the status quo, a wrong verdict is not.
const legacyCarePlanSettledPredicate = `(
			NOT EXISTS (
				SELECT 1 FROM schedule.student_arrival_schedules AS touched_arrival_plan
				WHERE touched_arrival_plan.student_id = student.student_id
					AND touched_arrival_plan.tenant_id = student.tenant_id
					AND (
						touched_arrival_plan.created_at IS NULL
						OR touched_arrival_plan.created_at > instance.completed_at
						OR touched_arrival_plan.updated_at IS NULL
						OR touched_arrival_plan.updated_at > instance.completed_at
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM schedule.student_pickup_schedules AS touched_pickup_plan
				WHERE touched_pickup_plan.student_id = student.student_id
					AND touched_pickup_plan.tenant_id = student.tenant_id
					AND (
						touched_pickup_plan.created_at IS NULL
						OR touched_pickup_plan.created_at > instance.completed_at
						OR touched_pickup_plan.updated_at IS NULL
						OR touched_pickup_plan.updated_at > instance.completed_at
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM schedule.student_arrival_exceptions AS touched_arrival_exception
				WHERE touched_arrival_exception.student_id = student.student_id
					AND touched_arrival_exception.tenant_id = student.tenant_id
					AND touched_arrival_exception.exception_date = instance.date
					AND (
						touched_arrival_exception.created_at IS NULL
						OR touched_arrival_exception.created_at > instance.completed_at
						OR touched_arrival_exception.updated_at IS NULL
						OR touched_arrival_exception.updated_at > instance.completed_at
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM schedule.student_pickup_exceptions AS touched_pickup_exception
				WHERE touched_pickup_exception.student_id = student.student_id
					AND touched_pickup_exception.tenant_id = student.tenant_id
					AND touched_pickup_exception.exception_date = instance.date
					AND (
						touched_pickup_exception.created_at IS NULL
						OR touched_pickup_exception.created_at > instance.completed_at
						OR touched_pickup_exception.updated_at IS NULL
						OR touched_pickup_exception.updated_at > instance.completed_at
					)
			)
		)`

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
// Flipping such a row to 'absent' restores what the normal completion path
// would have written — but ONLY for a child that day's care plan actually
// booked into the OGS. Group- and year-wide assignments (#1838) put every
// member on every occurrence of a block, so a legacy row on a weekday the
// child was never booked for is not a missed absence: writing 'absent' there
// invents the exact false claim this feature exists to prevent, in a
// migration that cannot be rolled back. Those rows are left alone — and are
// not stamped as non-bookings either, for the reason spelled out above
// legacyCarePlanSettledPredicate.
//
// The booking evidence is the same the runtime derivation reads
// (services/schedule/care_day_resolver.go): a weekly arrival or pickup row on
// the instance's ISO weekday, or any arrival/pickup exception on that exact
// date — a timed one says the child came, a timeless "kommt heute nicht" says
// the day WAS booked and the child cancelled, which is an absence to record
// either way.
//
// Unlike the runtime derivation, the evidence must also be OLD ENOUGH to say
// anything about that day: legacyCarePlanSettledPredicate drops every row whose
// care plan was written or changed after the instance completed. The
// derivation's CareDayUnknown branch ("no plan on file, so keep the child
// expected") is deliberately not reproduced either — a plan deleted since is
// indistinguishable from one that never existed, and this migration cannot be
// rolled back. Both kinds of ambiguity leave the row exactly as it is, which is
// what the old path already left behind.
//
// Not every completed+expected row is legacy, though. The attendance PATCH
// endpoint has always been able to set a row back to 'expected' AFTER its
// instance completed, and it carries no completed-instance guard — that reset
// is a deliberate human decision, and this migration cannot be rolled back.
// The two cases are told apart by time: a legacy row was last written before
// its instance completed (the old path never touched it), a manual reset
// after. So only rows untouched since completion are repaired; anything
// written afterwards, and anything on an instance with no completion
// timestamp to compare against, is left exactly as it is.
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
			AND student.status = 'expected'
			AND instance.completed_at IS NOT NULL
			AND student.updated_at <= instance.completed_at
			AND ` + legacyCarePlanSettledPredicate + `
			AND ` + legacyCareDayBookedPredicate + `;
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
