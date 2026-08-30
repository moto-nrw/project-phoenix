package parent_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func weeklyPlanSettings(bookingsAuthoritative bool) map[string]bool {
	return map[string]bool{
		configModels.KeyEnrollmentBookingsAuthoritative: bookingsAuthoritative,
		configModels.KeyParentCarePickupRequestEnabled:  true,
		configModels.KeyParentCareModeRequestEnabled:    true,
	}
}

func TestGetAndCreateCareScheduleRequest_BookingsAuthoritative(t *testing.T) {
	t.Parallel()

	_, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	svc := careScheduleServiceWithSettings(t, db, repos, weeklyPlanSettings(true))

	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, view.CanRequest)
	assert.Equal(t, parentService.CareScheduleRequestCapabilities{}, view.RequestCapabilities)

	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentService.ErrCareRequestBookingsAuthoritative)
}

func TestExistingCareScheduleRequestSurvivesBookingAuthorityChange(t *testing.T) {
	t.Parallel()

	_, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	weeklyPlan := careScheduleServiceWithSettings(t, db, repos, weeklyPlanSettings(false))

	created, err := weeklyPlan.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, created.PendingRequest)

	bookingLed := careScheduleServiceWithSettings(t, db, repos, weeklyPlanSettings(true))
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

	_, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var requests scheduleService.CareScheduleRequestService
	weeklyPlan := careScheduleServiceWithSettings(t, db, repos, weeklyPlanSettings(false), &requests)
	created, err := weeklyPlan.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, created.PendingRequest)

	bookingLed := careScheduleServiceWithSettings(t, db, repos, weeklyPlanSettings(true))
	view, err := bookingLed.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Review")
	staffCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	staffCtx = context.WithValue(staffCtx, jwt.CtxClaims, jwt.AppClaims{ID: int(staffAccount.ID)})
	staffCtx = context.WithValue(staffCtx, jwt.CtxPermissions, []string{"admin:*"})
	decided, err := requests.Decide(staffCtx, scheduleService.CareRequestDecideInput{
		RequestID: view.PendingRequest.ID, Approve: true, ReviewedBy: staffAccount.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, decided.Request.Status)
}
