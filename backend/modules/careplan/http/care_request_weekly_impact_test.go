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
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #6843: a weekly approval pins the diff the reviewer was shown. A staff
// change to the plan after the reviewer opened the queue refuses the
// approval instead of being overwritten; the reloaded diff approves.
func TestWeeklyApprovalRefusesPlanChangedSinceReview(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Paula", "Planerin")
	claims := testutil.AdminTestClaims(int(staffAccount.ID))

	tenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	upsertPickup := func(hour int) {
		require.NoError(t, tc.resource.PickupScheduleService.UpsertStudentPickupSchedule(tenantCtx, &careplan.PickupSchedule{
			StudentID: chain.StudentID, Weekday: 1, PickupTime: time.Date(1, 1, 1, hour, 0, 0, 0, time.UTC), CreatedBy: staff.ID,
		}))
	}
	upsertPickup(15)
	pending, err := testpkg.TenantQuery3(t, tenantCtx, tc.db, tc.careRequests.CreateRequest, chain.StudentID, chain.AccountID,
		json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`))
	require.NoError(t, err)

	open := func() (token, old string) {
		req, reqErr := http.NewRequest(http.MethodGet, fmt.Sprintf("/change-requests?view=open&types=care_schedule&student_id=%d", chain.StudentID), nil)
		require.NoError(t, reqErr)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data struct {
				Items []struct {
					Data struct {
						ID          string `json:"id"`
						ImpactToken string `json:"impact_token"`
						Diff        []struct {
							Old string `json:"old"`
						} `json:"diff"`
					} `json:"data"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		for _, item := range body.Data.Items {
			if item.Data.ID == strconv.FormatInt(pending.ID, 10) {
				require.Len(t, item.Data.Diff, 1)
				return item.Data.ImpactToken, item.Data.Diff[0].Old
			}
		}
		t.Fatalf("pending request %d missing from the open queue", pending.ID)
		return "", ""
	}
	decide := func(token string) (int, string) {
		req, reqErr := http.NewRequest(http.MethodPost, fmt.Sprintf("/care-schedule-change-requests/%d/decide", pending.ID),
			strings.NewReader(fmt.Sprintf(`{"approve":true,"reason":"Passt so","impact_token":%q}`, token)))
		require.NoError(t, reqErr)
		req.Header.Set("Content-Type", "application/json")
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		return rr.Code, rr.Body.String()
	}

	shown, old := open()
	require.NotEmpty(t, shown, "a weekly request carries the token of its displayed diff")
	assert.Equal(t, "15:00", old)

	// Staff change the Monday pickup after the reviewer opened the diff.
	upsertPickup(17)
	code, body := decide(shown)
	require.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, `"code":"students.care_request_plan_changed"`)
	pickups, err := testpkg.TenantQuery1(t, tenantCtx, tc.db, tc.resource.PickupScheduleService.GetStudentPickupSchedules, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	assert.Equal(t, "17:00", pickups[0].PickupTime.Format("15:04"), "the staff change survives the refused approval")

	reloaded, old := open()
	assert.NotEqual(t, shown, reloaded)
	assert.Equal(t, "17:00", old, "the reloaded diff shows the staff change")
	code, body = decide(reloaded)
	require.Equal(t, http.StatusOK, code, body)
}
