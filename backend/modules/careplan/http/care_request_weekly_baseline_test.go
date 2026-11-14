package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type baselineDiff struct {
	Old          string `json:"old"`
	New          string `json:"new"`
	SubmittedOld string `json:"submitted_old"`
}

// #6990: the family sees pickup 15:00 and asks for 16:00. Staff then set
// 16:30. The reviewer must see that the family asked against 15:00, so a
// later pickup does not read as an earlier one; the decision freezes it.
func TestWeeklyReviewShowsThePlanTheFamilySubmittedAgainst(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Paula", "Planerin")
	claims := testutil.AdminTestClaims(int(staffAccount.ID))

	tenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	upsertPickup := func(hour, minute int) {
		require.NoError(t, tc.resource.PickupScheduleService.UpsertStudentPickupSchedule(tenantCtx, &careplan.PickupSchedule{
			StudentID: chain.StudentID, Weekday: 3, PickupTime: time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC), CreatedBy: staff.ID,
		}))
	}
	upsertPickup(15, 0)
	pending, err := testpkg.TenantQuery3(t, tenantCtx, tc.db, tc.careRequests.CreateRequest, chain.StudentID, chain.AccountID,
		json.RawMessage(`{"weekdays":[{"weekday":3,"pickup":"16:00"}]}`))
	require.NoError(t, err)

	token, diff := openWeeklyBaseline(t, tc, claims, chain.StudentID, pending.ID)
	assert.Equal(t, baselineDiff{Old: "15:00", New: "16:00"}, diff, "an unchanged plan carries no hint")
	unchangedToken := token

	upsertPickup(16, 30)
	token, diff = openWeeklyBaseline(t, tc, claims, chain.StudentID, pending.ID)
	assert.Equal(t, baselineDiff{Old: "16:30", New: "16:00", SubmittedOld: "15:00"}, diff)
	assert.NotEqual(t, unchangedToken, token)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/care-schedule-change-requests/%d/decide", pending.ID),
		strings.NewReader(fmt.Sprintf(`{"approve":true,"reason":"Passt so","impact_token":%q}`, token)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	req, err = http.NewRequest(http.MethodGet, "/change-requests?view=history&types=care_schedule", nil)
	require.NoError(t, err)
	rr = authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var history struct {
		Data struct {
			Items []struct {
				Data struct {
					ID   string         `json:"id"`
					Diff []baselineDiff `json:"diff"`
				} `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &history))
	for _, item := range history.Data.Items {
		if item.Data.ID == strconv.FormatInt(pending.ID, 10) {
			assert.Equal(t, []baselineDiff{{Old: "16:30", New: "16:00", SubmittedOld: "15:00"}}, item.Data.Diff, "the decision freezes the hint")
			return
		}
	}
	t.Fatalf("decided request %d missing from the history", pending.ID)
}

func openWeeklyBaseline(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID, requestID int64) (string, baselineDiff) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/change-requests?view=open&types=care_schedule&student_id=%d", studentID), nil)
	require.NoError(t, err)
	rr := authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Data struct {
			Items []struct {
				Data struct {
					ID          string         `json:"id"`
					ImpactToken string         `json:"impact_token"`
					Diff        []baselineDiff `json:"diff"`
				} `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	for _, item := range body.Data.Items {
		if item.Data.ID == strconv.FormatInt(requestID, 10) {
			require.Len(t, item.Data.Diff, 1)
			return item.Data.ImpactToken, item.Data.Diff[0]
		}
	}
	t.Fatalf("pending request %d missing from the open queue", requestID)
	return "", baselineDiff{}
}
