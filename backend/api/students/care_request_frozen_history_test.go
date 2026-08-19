package students_test

// Hermetic router test for #2430: deciding a care-schedule change request
// through the production route freezes the alt → neu diff, and the history
// route keeps serving that frozen state after the live data changes. The full
// middleware chain runs (Verifier → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware), exactly as the real server does.

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
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestCareRequestHistory_ServesFrozenDecisionDiff(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	// The approve path stamps the acting staff resolved from the JWT account,
	// so the deciding admin needs a real staff record behind their account.
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Paula", "Planerin")

	tenantCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	upsertPickup := func(hour, minute int) {
		require.NoError(t, tc.services.PickupSchedule.UpsertStudentPickupSchedule(
			tenantCtx,
			&scheduleModels.StudentPickupSchedule{
				StudentID:  chain.StudentID,
				Weekday:    1,
				PickupTime: time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC),
				CreatedBy:  staff.ID,
			},
		))
	}

	// Live plan before the request: Monday pickup 15:00.
	upsertPickup(15, 0)

	// The guardian requests Monday pickup 16:00.
	pending, err := tc.services.CareRequests.CreateRequest(
		tenantCtx, chain.StudentID, chain.AccountID,
		map[string]any{"weekdays": []any{
			map[string]any{"weekday": 1, "pickup": "16:00"},
		}},
	)
	require.NoError(t, err)

	// Approve through the production decide route.
	decideReq, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("/care-schedule-change-requests/%d/decide", pending.ID),
		strings.NewReader(`{"approve":true}`))
	require.NoError(t, err)
	decideReq.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, decideReq, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// The live data moves on AFTER the decision.
	upsertPickup(17, 0)

	// The history must still replay the frozen decision-time comparison.
	histReq, err := http.NewRequest(http.MethodGet, "/care-schedule-change-requests/history", nil)
	require.NoError(t, err)
	rr = authExec(t, tc, histReq, testutil.AdminTestClaims(int(staffAccount.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var body struct {
		Data struct {
			Items []struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				Requested []struct {
					Label string `json:"label"`
					New   string `json:"new"`
				} `json:"requested"`
				Diff []struct {
					Label string `json:"label"`
					Old   string `json:"old"`
					New   string `json:"new"`
				} `json:"diff"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	var found bool
	for _, item := range body.Data.Items {
		if item.ID != fmt.Sprintf("%d", pending.ID) {
			continue
		}
		found = true
		assert.Equal(t, scheduleModels.CareRequestStatusApproved, item.Status)
		require.Len(t, item.Diff, 1, "the decided row must carry the frozen diff")
		assert.Equal(t, "Montag · Abholzeit", item.Diff[0].Label)
		assert.Equal(t, "15:00", item.Diff[0].Old, "old side is the decision-time value, not the current one")
		assert.Equal(t, "16:00", item.Diff[0].New)
		assert.NotEmpty(t, item.Requested, "the payload summary fallback stays on the wire")
	}
	require.True(t, found, "decided request must appear in the history")
}
