package care_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func weeklyPlanSettings(bookingsAuthoritative bool) map[string]bool {
	return map[string]bool{
		parentportal.KeyEnrollmentBookingsAuthoritative: bookingsAuthoritative,
		parentportal.KeyParentCarePickupRequestEnabled:  true,
		parentportal.KeyParentCareModeRequestEnabled:    true,
	}
}

func TestGetAndCreateCareScheduleRequest_BookingsAuthoritative(t *testing.T) {
	t.Parallel()

	_, db, repos, reposPeople := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	svc := careScheduleServiceWithSettings(t, db, repos, reposPeople, weeklyPlanSettings(true))

	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, view.CanRequest)
	assert.Equal(t, parentportal.CareScheduleRequestCapabilities{}, view.RequestCapabilities)

	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentportal.ErrCareRequestBookingsAuthoritative)
}

func TestExistingCareScheduleRequestSurvivesBookingAuthorityChange(t *testing.T) {
	t.Parallel()

	_, db, repos, reposPeople := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	weeklyPlan := careScheduleServiceWithSettings(t, db, repos, reposPeople, weeklyPlanSettings(false))

	created, err := weeklyPlan.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, created.PendingRequest)

	bookingLed := careScheduleServiceWithSettings(t, db, repos, reposPeople, weeklyPlanSettings(true))
	view, err := bookingLed.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest)
	assert.True(t, view.PendingRequest.SubmittedBySelf)
	assert.False(t, view.CanRequest)

	// Withdrawal was retired (#2267); editing the own pending request is the
	// path that replaced it, and it must stay open after the switch.
	edited, err := bookingLed.EditCareScheduleRequest(
		testpkg.WithPackageTenantRuntime(context.Background()),
		chain.AccountID, chain.StudentID, view.PendingRequest.ID, carePayload(), "",
	)
	require.NoError(t, err)
	require.NotNil(t, edited.PendingRequest)
	assert.Equal(t, view.PendingRequest.ID, edited.PendingRequest.ID)
}

func TestExistingCareScheduleRequestCanBeDecidedAfterAuthorityChange(t *testing.T) {
	t.Parallel()

	_, db, repos, reposPeople := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var requests carerequests.Service
	weeklyPlan := careScheduleServiceWithSettings(t, db, repos, reposPeople, weeklyPlanSettings(false), &requests)
	created, err := weeklyPlan.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, created.PendingRequest)

	bookingLed := careScheduleServiceWithSettings(t, db, repos, reposPeople, weeklyPlanSettings(true))
	view, err := bookingLed.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Review")
	staffCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	staffCtx = testpkg.WithAuthenticatedContext(staffCtx, testpkg.Claims{ID: int(staffAccount.ID)}, []string{"admin:*"})
	var decided *carerequests.ReviewItem
	err = testpkg.WithTenantTx(t, staffCtx, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var decideErr error
		decided, decideErr = requests.Decide(txCtx, carerequests.DecideInput{
			RequestID: view.PendingRequest.ID, Approve: true, ReviewedBy: staffAccount.ID,
		})
		return decideErr
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", decided.Request.Status)
}
