package application

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The school-wide overview rule itself (which setting value grants what) is
// bound at the composition root; services/caller_context_internal_test.go
// pins it against the settings. These tests fake the overview port, assert
// the arguments the caller context hands it, and pin which room sessions a
// live-update client subscribes to.

// subscriptionSessions is the successor of resolveSSESupervisions.
func subscriptionSessionsOf(t *testing.T, h *callerHarness) ([]int64, error) {
	t.Helper()
	return h.build(t).subscriptionSessions(context.Background(), h.caller, testStaffID)
}

// A system-wide admin permission (admin:* or *:*, mapped to AdminWildcard at
// the root) subscribes without a staff record.
func TestSSESubscription_WildcardAdminWithoutStaff(t *testing.T) {
	t.Parallel()

	caller := authenticatedCaller(false)
	caller.AdminWildcard = true
	h := newCallerHarness(caller)
	h.overview = overviewForScope(overviewAdmins)
	h.live = &callerTestLiveTopics{}

	subscription, err := h.build(t).SSESubscription(context.Background())

	require.NoError(t, err)
	assert.Zero(t, subscription.StaffID)
	assert.Empty(t, subscription.AllTopics)
	assert.True(t, h.overview.gotAdmin, "the wildcard counts as effective admin")
	assert.Equal(t, 1, h.live.openCalls)
}

func TestSubscriptionSessions_AdminWithSettingEnabled(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(true))
	h.overview = overviewForScope(overviewAdmins)
	h.live = &callerTestLiveTopics{open: []int64{10, 11}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(10), result[0])
	assert.Equal(t, int64(11), result[1])
	assert.True(t, h.overview.gotAdmin)
	assert.False(t, h.overview.gotAssignmentBound)
}

func TestSubscriptionSessions_AdminWithOwnScope(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(true))
	h.overview = overviewForScope(overviewOwn)
	h.live = &callerTestLiveTopics{open: []int64{20}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(20), result[0])
}

func TestSubscriptionSessions_NonAdmin(t *testing.T) {
	t.Parallel()

	// Admin scope, but the caller is not an admin.
	h := newCallerHarness(authenticatedCaller(false))
	h.overview = overviewForScope(overviewAdmins)
	h.live = &callerTestLiveTopics{staff: []int64{30}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.False(t, h.overview.gotAdmin)
	assert.Zero(t, h.live.openCalls)
}

func TestSubscriptionSessions_NilOverview(t *testing.T) {
	t.Parallel()

	// Admin, but no overview port bound (the root binds none without settings).
	h := newCallerHarness(authenticatedCaller(true))
	h.live = &callerTestLiveTopics{staff: []int64{40}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Zero(t, h.live.openCalls)
}

func TestSubscriptionSessions_SettingErrorFallsBack(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(true))
	h.overview = failingOverview()
	h.live = &callerTestLiveTopics{staff: []int64{50}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, h.overview.calls)
	assert.Zero(t, h.live.openCalls)
}

func TestSubscriptionSessions_NilLiveTopicsReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		isAdmin  bool
		overview func() *callerTestOverview
	}{
		{name: "non-admin", isAdmin: false},
		{name: "admin without settings service", isAdmin: true},
		{name: "admin with overview disabled", isAdmin: true, overview: func() *callerTestOverview { return overviewForScope(overviewOwn) }},
		{name: "admin with setting error", isAdmin: true, overview: failingOverview},
		{name: "admin with overview enabled", isAdmin: true, overview: func() *callerTestOverview { return overviewForScope(overviewAdmins) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newCallerHarness(authenticatedCaller(tt.isAdmin))
			if tt.overview != nil {
				h.overview = tt.overview()
			}

			result, err := subscriptionSessionsOf(t, h)

			require.ErrorContains(t, err, "SSE active service is not configured")
			assert.Nil(t, result)
		})
	}
}

func TestSubscriptionSessions_StaffSupervisionsError(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false))
	h.overview = overviewForScope(overviewOwn)
	h.live = &callerTestLiveTopics{staffErr: errors.New("database connection lost")}

	result, err := subscriptionSessionsOf(t, h)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSubscriptionSessions_NonAdminStaffError(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false))
	h.overview = failingOverview()
	h.live = &callerTestLiveTopics{staffErr: errors.New("timeout")}

	result, err := subscriptionSessionsOf(t, h)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSubscriptionSessions_OpenSessionsError(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(true))
	h.overview = overviewForScope(overviewAdmins)
	h.live = &callerTestLiveTopics{openErr: errors.New("database error")}

	result, err := subscriptionSessionsOf(t, h)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// Open sessions without a supervisor row (e.g. Schulhof without a current
// claim) are part of the school-wide topic list: the live-topic port lists
// open sessions, not supervisions.
func TestSubscriptionSessions_AdminIncludesUnclaimedGroups(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(true))
	h.overview = overviewForScope(overviewAdmins)
	h.live = &callerTestLiveTopics{open: []int64{50, 51}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, int64(50), result[0])
	assert.Equal(t, int64(51), result[1])
	assert.Zero(t, h.live.staffCalls)
}

// A caregiver who SEES every running module in the list must also receive
// its live events under the all_staff scope (#2380).
func TestSubscriptionSessions_AllStaffScopeSubscribesNonAdmin(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false)).withStaff()
	h.overview = overviewForScope(overviewAllStaff)
	h.live = &callerTestLiveTopics{open: []int64{60, 61}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, int64(60), result[0])
	assert.Equal(t, int64(61), result[1])
	assert.Zero(t, h.live.staffCalls, "must not fall back to own supervisions under the all_staff scope")
	assert.True(t, h.overview.gotHasStaff, "the caller context verifies the staff record")
	assert.False(t, h.overview.gotAdmin)
}

// A caller without a staff record (guardian, guest) stays on their own
// supervisions even under the broadest scope.
func TestSubscriptionSessions_AllStaffScopeDeniesNonStaff(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false))
	h.overview = overviewForScope(overviewAllStaff)
	h.live = &callerTestLiveTopics{staff: []int64{70}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(70), result[0])
	assert.Zero(t, h.live.openCalls, "a non-staff caller must never enumerate all active groups")
	assert.False(t, h.overview.gotHasStaff)
}

// The deactivation case: after the school switches back, foreign modules
// disappear from the subscription again.
func TestSubscriptionSessions_OwnScopeKeepsCaregiverNarrow(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false)).withStaff()
	h.overview = overviewForScope(overviewOwn)
	h.live = &callerTestLiveTopics{staff: []int64{80}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(80), result[0])
	assert.Zero(t, h.live.openCalls, "the own scope must never enumerate all active groups")
}

// A school-portal token is assignment-bound (#2527): the caller context hands
// that fact to the overview rule.
func TestSubscriptionSessions_SchoolScopeIsAssignmentBound(t *testing.T) {
	t.Parallel()

	caller := authenticatedCaller(true)
	caller.SchoolScope = true
	h := newCallerHarness(caller).withStaff()
	h.overview = overviewForScope(overviewAllStaff)
	h.live = &callerTestLiveTopics{staff: []int64{90}}

	result, err := subscriptionSessionsOf(t, h)

	require.NoError(t, err)
	assert.Equal(t, []int64{90}, result)
	assert.True(t, h.overview.gotAssignmentBound)
	assert.True(t, h.overview.gotAdmin)
	assert.Zero(t, h.live.openCalls)
}

// SSESubscription rejects a caller who is neither staff nor an effective
// admin with the status the handler renders.
func TestSSESubscription_RejectsWithSetupError(t *testing.T) {
	t.Parallel()

	noPerson := newCallerHarness(authenticatedCaller(false))
	noPerson.people.found = false
	noPerson.live = &callerTestLiveTopics{}
	_, err := noPerson.build(t).SSESubscription(context.Background())
	var setupErr *domain.SSESetupError
	require.ErrorAs(t, err, &setupErr)
	assert.Equal(t, "Account not found", setupErr.Message)
	assert.Equal(t, http.StatusUnauthorized, setupErr.Status)

	noStaff := newCallerHarness(authenticatedCaller(false))
	noStaff.live = &callerTestLiveTopics{}
	_, err = noStaff.build(t).SSESubscription(context.Background())
	require.ErrorAs(t, err, &setupErr)
	assert.Equal(t, "User is not a staff member", setupErr.Message)
	assert.Equal(t, http.StatusForbidden, setupErr.Status)
}
