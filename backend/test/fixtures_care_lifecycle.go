package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The two care-lifecycle fixtures the Care Plan behaviour tests and the
// student-deletion workflow tests share (#3350). They live in the catalog
// rather than in either package because both suites need the identical
// starting state and a second copy is how the two drift apart.

// CareWithdrawalWriter is the slice of the withdrawal repository the fixture
// stores through. Care Plan owns the task's upsert rule (one pending task per
// child, a school confirmation surviving a booking expiry), so the fixture
// asks the owner rather than inserting a row the owner would never write.
type CareWithdrawalWriter interface {
	UpsertPending(ctx context.Context, completion *users.CareWithdrawalCompletion) error
}

// CreateTestCareWithdrawalCompletion stores one pending withdrawal task for
// the child: the school confirmed the withdrawal and firstGap is the first day
// without a booking.
func CreateTestCareWithdrawalCompletion(
	tb testing.TB,
	withdrawals CareWithdrawalWriter,
	studentID, actorID int64,
	firstGap timezone.Date,
) *users.CareWithdrawalCompletion {
	tb.Helper()
	row := &users.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 users.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy:   &actorID,
		WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt:   time.Now(),
	}
	require.NoError(tb, withdrawals.UpsertPending(Ctx(tb), row))
	return row
}

// AssignStudentGroup puts a fixture child into an education group.
func AssignStudentGroup(tb testing.TB, db *bun.DB, studentID, groupID int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(Ctx(tb), 5*time.Second)
	defer cancel()
	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(ctx)
	require.NoError(tb, err, "assign group to test student")
}

// HoldStudentRowLock opens its own transaction, locks the child's row and
// keeps it locked until the test ends — the stand-in for "someone else is
// editing this child right now", which every companion lock-order test needs.
func HoldStudentRowLock(tb testing.TB, db *bun.DB, studentID int64) {
	tb.Helper()
	transaction, err := db.BeginTx(context.Background(), nil)
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = transaction.Rollback() })

	var locked int64
	require.NoError(tb, transaction.NewRaw(
		`SELECT id FROM users.students WHERE id = ? FOR UPDATE`, studentID,
	).Scan(context.Background(), &locked))
	require.Equal(tb, studentID, locked)
}

// SetStudentLifecycle stages the enrolment columns a lifecycle test needs.
// CreateTestStudent exposes no status / enrolled_from / enrolled_until knobs,
// so the pre-condition is patched with one UPDATE scoped to the fixture row.
func SetStudentLifecycle(
	tb testing.TB,
	db *bun.DB,
	studentID int64,
	status users.StudentStatus,
	enrolledFrom, enrolledUntil *timezone.Date,
) {
	tb.Helper()
	query := db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(status)).
		Where("id = ?", studentID)
	if enrolledFrom != nil {
		query = query.Set("enrolled_from = ?", *enrolledFrom)
	} else {
		query = query.Set("enrolled_from = NULL")
	}
	if enrolledUntil != nil {
		query = query.Set("enrolled_until = ?", *enrolledUntil)
	} else {
		query = query.Set("enrolled_until = NULL")
	}
	_, err := query.Exec(Ctx(tb))
	require.NoError(tb, err, "failed to seed lifecycle columns")
}

// StudentPlanWriter is the slice of the child repository the departure-plan
// fixture needs. It writes through the repository rather than in raw SQL so
// the stored plan is the normalized one every production write leaves behind;
// a hand-written UPDATE would desynchronize the derived columns and make the
// next real write look like it changed them.
type StudentPlanWriter interface {
	FindByID(ctx context.Context, id any) (*users.Student, error)
	Update(ctx context.Context, student *users.Student) error
}

// SetAccompaniedDepartureDays gives the child an "Anderes Kind" departure plan
// on the given weekdays, which is the precondition for carrying a companion
// link. The free-text note comes along because an accompanied plan must say
// "mit wem" and there is no link yet at this point.
func SetAccompaniedDepartureDays(
	tb testing.TB,
	ctx context.Context,
	students StudentPlanWriter,
	studentID int64,
	days ...string,
) *users.Student {
	tb.Helper()
	student, err := students.FindByID(ctx, studentID)
	require.NoError(tb, err)
	require.NotNil(tb, student)

	note := "Nachbarskind"
	student.AllowedDepartureModes = users.WithAccompaniedDays(student.AllowedDepartureModes, days)
	student.DepartureCompanionNote = &note
	require.NoError(tb, students.Update(ctx, student))
	return student
}
