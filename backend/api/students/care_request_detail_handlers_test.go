package students_test

// Hermetic router tests for #3135: GET /care-schedule-change-requests/{id}
// opens ONE pickup-change request of any status for a reviewer, keeps the
// requested day and time after the decision, and refuses readers the route
// gate or the per-child scope does not admit.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careRequestDetailBody struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	RequestKind   string `json:"request_kind"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	RequestReason string `json:"request_reason"`
	Requested     []struct {
		Label string `json:"label"`
		New   string `json:"new"`
	} `json:"requested"`
	Diff []struct {
		Label string `json:"label"`
		Old   string `json:"old"`
		New   string `json:"new"`
	} `json:"diff"`
	DecisionReason *string    `json:"decision_reason"`
	DecidedAt      *time.Time `json:"decided_at"`
	DecidedByName  string     `json:"decided_by_name"`
	PickupChange   *struct {
		Date               string `json:"date"`
		PickupTime         string `json:"pickup_time"`
		PreviousPickupTime string `json:"previous_pickup_time"`
	} `json:"pickup_change"`
}

// pinCareRequestToday fixes the request window on a Berlin date so the
// fixture dates below never depend on the wall clock.
func pinCareRequestToday(t *testing.T, tc *testContext) {
	t.Helper()
	setter, ok := tc.resource.CareRequestService.(interface{ SetTodayDate(func() timezone.Date) })
	require.True(t, ok)
	setter.SetTodayDate(func() timezone.Date { return timezone.NewDate(2026, 9, 9) })
}

func getCareRequestDetail(t *testing.T, tc *testContext, requestID int64, claims jwt.AppClaims, perms []string) (int, careRequestDetailBody) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/care-schedule-change-requests/%d", requestID), nil)
	require.NoError(t, err)
	rr := authExec(t, tc, req, claims, perms)
	var envelope struct {
		Data careRequestDetailBody `json:"data"`
	}
	if rr.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope), rr.Body.String())
	}
	return rr.Code, envelope.Data
}

func TestCareRequestDetail_OpensPickupChangeBeforeAndAfterDecision(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Paula", "Planerin")
	tenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	pinCareRequestToday(t, tc)

	// A pickup change for a school day within the request window, with a
	// weekly pickup on that weekday so the request records the previous time.
	date := timezone.NewDate(2026, 9, 15) // a Tuesday
	require.NoError(t, tc.resource.PickupScheduleService.UpsertStudentPickupSchedule(tenantCtx, &scheduleModels.StudentPickupSchedule{
		StudentID:  chain.StudentID,
		Weekday:    int(date.Weekday()),
		PickupTime: timezone.NormalizeWallClock(time.Date(1, 1, 1, 15, 30, 0, 0, time.UTC)),
		CreatedBy:  staff.ID,
	}))
	pending, err := tc.resource.CareRequestService.CreatePickupChangeRequest(
		tenantCtx, chain.StudentID, chain.AccountID, date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC), "Arzttermin",
	)
	require.NoError(t, err)
	expectedLabel := date.Format("02.01.2006") + " · Abholzeit"

	code, body := getCareRequestDetail(t, tc, pending.ID, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, fmt.Sprint(pending.ID), body.ID)
	assert.Equal(t, "pending", body.Status)
	assert.Equal(t, "pickup_change", body.RequestKind)
	assert.Equal(t, "Arzttermin", body.RequestReason)
	require.Len(t, body.Requested, 1)
	assert.Equal(t, expectedLabel, body.Requested[0].Label)
	assert.Equal(t, "14:30", body.Requested[0].New)
	assert.Nil(t, body.DecidedAt, "a pending request carries no decision")
	assert.Empty(t, body.DecidedByName)
	require.NotNil(t, body.PickupChange, "a pickup change names its stored day and times")
	assert.Equal(t, "2026-09-15", body.PickupChange.Date)
	assert.Equal(t, "14:30", body.PickupChange.PickupTime)
	assert.Equal(t, "15:30", body.PickupChange.PreviousPickupTime)

	// Reject through the production decide route.
	decideReq, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("/care-schedule-change-requests/%d/decide", pending.ID),
		strings.NewReader(`{"approve":false,"reason":"Passt nicht"}`))
	require.NoError(t, err)
	decideReq.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, decideReq, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// The same reference still opens the same request, now with the decision.
	code, body = getCareRequestDetail(t, tc, pending.ID, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, fmt.Sprint(pending.ID), body.ID)
	assert.Equal(t, "rejected", body.Status)
	assert.Equal(t, "Arzttermin", body.RequestReason)
	require.NotNil(t, body.DecisionReason)
	assert.Equal(t, "Passt nicht", *body.DecisionReason)
	require.NotNil(t, body.DecidedAt)
	assert.Equal(t, "Paula Planerin", body.DecidedByName)
	require.Len(t, body.Requested, 1)
	assert.Equal(t, expectedLabel, body.Requested[0].Label)
	require.NotEmpty(t, body.Diff, "the decision froze the previous → requested comparison")
	assert.Equal(t, "15:30", body.Diff[0].Old)
	assert.Equal(t, "14:30", body.Diff[0].New)
}

func TestCareRequestDetail_RefusesUnauthorizedReaders(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Paula", "Planerin")
	tenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	pinCareRequestToday(t, tc)

	date := timezone.NewDate(2026, 9, 15)
	pending, err := tc.resource.CareRequestService.CreatePickupChangeRequest(
		tenantCtx, chain.StudentID, chain.AccountID, date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC), "Arzttermin",
	)
	require.NoError(t, err)

	// Reading messages does not grant request details: without users:update
	// the route gate refuses.
	code, _ := getCareRequestDetail(t, tc, pending.ID, testutil.TeacherTestClaims(int(staffAccount.ID)), []string{"users:view"})
	assert.Equal(t, http.StatusForbidden, code)

	// users:update without a staff record behind the account (the guardian)
	// is outside the per-child review scope.
	code, _ = getCareRequestDetail(t, tc, pending.ID, testutil.TeacherTestClaims(int(chain.AccountID)), []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, code)

	// A removed or foreign request is not found, never misattributed.
	code, _ = getCareRequestDetail(t, tc, pending.ID+1_000_000, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, code)
}
