package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// rebookingGapDay is the fixed reference day of the rebooking scenarios.
var rebookingGapDay = calendar.NewDate(2026, 9, 14)

// rebookedAfterRealGap seeds the #6288 (EC-1) state: the first booking ran
// out ten days before rebookingGapDay, its withdrawal task is still open, and
// a finite booking started again three days before the reference day. The
// days between are a real gap, so the open task must stay.
func rebookedAfterRealGap(t *testing.T, db *bun.DB, scope testpkg.TenantScope, name string) (studentID int64, firstGap calendar.Date) {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, name, "Lücke", "3e")
	firstGap = rebookingGapDay.AddDays(-10)
	createCareBooking(t, db, scope, student.ID, "before-gap", nil, &firstGap)
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).CareWithdrawal
	studentID = student.ID
	require.NoError(t, api.SeedPendingCareWithdrawal(scope.Context(), repo, &careplan.WithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger: careplan.WithdrawalTriggerBookingExpired, WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
	}))
	rebookedFrom := rebookingGapDay.AddDays(-3)
	rebookedUntil := rebookingGapDay.AddDays(20)
	createCareBooking(t, db, scope, student.ID, "after-gap", &rebookedFrom, &rebookedUntil)
	return studentID, firstGap
}

func fixedDayBookingAuthorityService(t *testing.T, db *bun.DB, today calendar.Date) careplan.CareLifecycle {
	t.Helper()
	repos, err := api.NewCareLifecycleTestRepositories(testpkg.NewDatabase(t, db), nil)
	require.NoError(t, err)
	svc, err := repos.NewCareLifecycle(api.CareLifecycleTestConfig{
		Audit:                 api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(testpkg.NewDatabase(t, db))),
		BookingsAuthoritative: func(context.Context) (bool, error) { return true, nil },
		Today:                 func() calendar.Date { return today },
	})
	require.NoError(t, err)
	return svc
}

// TestDueEffectsKeepTheOpenTaskAfterAFiniteRebookingWithARealGap pins the
// nightly half of #6288 (EC-1): a finite booking that starts again after a
// real gap is no concurrent change. Before the fix every ApplyDueEffects run
// of the school failed with "pending booking completion changed during
// reconciliation", so no child of that school had its exit effects applied.
func TestDueEffectsKeepTheOpenTaskAfterAFiniteRebookingWithARealGap(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	studentID, firstGap := rebookedAfterRealGap(t, db, scope, "Nachts")
	svc := fixedDayBookingAuthorityService(t, db, rebookingGapDay)
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).CareWithdrawal
	before := requirePendingCompletion(t, repo, scope.Context(), studentID)

	for range 2 {
		_, err := svc.ApplyDueEffects(scope.Context(), rebookingGapDay)
		require.NoError(t, err, "a rebooking after a real gap must not fail the school's due effects")
	}
	after := requirePendingCompletion(t, repo, scope.Context(), studentID)
	assert.Equal(t, before.ID, after.ID, "the task of the real gap stays the open task")
	assert.Equal(t, firstGap, after.FirstBookinglessDay, "the gap the school has to decide on is unchanged")
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt, "keeping the task writes nothing")
}

// TestRebookingAfterARealGapKeepsTheBookingAndTheOpenTask pins the booking
// half of #6288 (EC-1): the reconciliation that runs with a booking change
// used to fail the same way and rolled the booking back.
func TestRebookingAfterARealGapKeepsTheBookingAndTheOpenTask(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	studentID, firstGap := rebookedAfterRealGap(t, db, scope, "Tags")
	svc := fixedDayBookingAuthorityService(t, db, rebookingGapDay)

	require.NoError(t, svc.ReconcileAuthoritativeBookingChange(scope.Context(),
		careplan.CareWithdrawalBookingChange{StudentID: studentID, ConfirmedRole: "admin"}),
		"a rebooking after a real gap must be accepted")
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).CareWithdrawal
	assert.Equal(t, firstGap, requirePendingCompletion(t, repo, scope.Context(), studentID).FirstBookinglessDay)
}
