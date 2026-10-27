package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The two care-lifecycle fixtures the Care Plan behaviour tests and the
// student-deletion workflow tests share (#3350). They live in the catalog
// rather than in either package because both suites need the identical
// starting state and a second copy is how the two drift apart.

// CreateTestCareWithdrawalCompletion stores one pending withdrawal task for
// the child: the school confirmed the withdrawal and firstGap is the first day
// without a booking.
func CreateTestCareWithdrawalCompletion(
	tb testing.TB,
	db bun.IDB,
	studentID, actorID int64,
	firstGap calendar.Date,
) *CareWithdrawalCompletion {
	tb.Helper()
	return CreateTestCareWithdrawalCompletionConfirmedAt(tb, db, studentID, actorID, firstGap, time.Now())
}

// ResolveTestCareWithdrawal marks the withdrawal task as resolved with the
// care ended, the state a completed withdrawal shows in the child's
// Änderungsprotokoll. Ending the care itself is not reproduced; a test of
// that path drives Care Plan.
func ResolveTestCareWithdrawal(tb testing.TB, db bun.IDB, completionID int64) {
	tb.Helper()
	_, err := db.NewUpdate().
		Model((*CareWithdrawalCompletion)(nil)).
		Set("state = 'resolved'").
		Set("outcome = 'care_ended'").
		Set("resolved_at = NOW()").
		Where("id = ?", completionID).
		Where("tenant_id = ?", fixtureTenantID(tb)).
		Exec(Ctx(tb))
	require.NoError(tb, err, "resolve care withdrawal task")
}

// upsertTestPendingCareWithdrawal stores row as the child's pending
// withdrawal task in the test's tenant. Care Plan keeps one pending task per
// child, so a second call for the same child moves that task to the new
// values. Care Plan's rule that a school confirmation survives a later
// booking expiry is not reproduced here; a test of that rule drives Care Plan.
func upsertTestPendingCareWithdrawal(tb testing.TB, db bun.IDB, row *CareWithdrawalCompletion) *CareWithdrawalCompletion {
	tb.Helper()
	row.TenantID = fixtureTenantID(tb)
	row.State = "pending"
	row.SourceOfferings = json.RawMessage(`[]`)
	_, err := db.NewInsert().
		Model(row).
		On("CONFLICT (tenant_id, student_id) WHERE state = 'pending' AND student_id IS NOT NULL DO UPDATE").
		Set("first_bookingless_day = EXCLUDED.first_bookingless_day").
		Set("trigger = EXCLUDED.trigger").
		Set("withdrawal_confirmed_by = EXCLUDED.withdrawal_confirmed_by").
		Set("withdrawal_confirmed_role = EXCLUDED.withdrawal_confirmed_role").
		Set("withdrawal_confirmed_at = EXCLUDED.withdrawal_confirmed_at").
		Set("updated_at = NOW()").
		Returning("id, created_at, updated_at").
		Exec(Ctx(tb))
	require.NoError(tb, err, "store pending care withdrawal task")
	return row
}

// AssignStudentGroup puts a fixture child into an education group.
func AssignStudentGroup(tb testing.TB, db *bun.DB, studentID, groupID int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(Ctx(tb), 5*time.Second)
	defer cancel()
	query, err := updateStudentMembershipFixture(ctx, db, studentID)
	require.NoError(tb, err)
	_, err = query.Set("group_id = ?", groupID).Exec(ctx)
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
		`SELECT p.id FROM users.student_profiles p
		JOIN users.student_school_memberships m ON m.tenant_id=p.tenant_id AND m.student_profile_id=p.id AND m.deleted_at IS NULL
		JOIN users.student_care_profiles c ON c.tenant_id=m.tenant_id AND c.membership_id=m.id
		WHERE p.id = ? FOR UPDATE OF p, m, c`, studentID,
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
	status string,
	enrolledFrom, enrolledUntil *calendar.Date,
) {
	tb.Helper()
	query, err := updateStudentMembershipFixture(Ctx(tb), db, studentID)
	require.NoError(tb, err)
	query = query.Set("status = ?", status)
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
	_, err = query.Exec(Ctx(tb))
	require.NoError(tb, err, "failed to seed lifecycle columns")
}
