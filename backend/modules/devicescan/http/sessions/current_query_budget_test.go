package sessions_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentSessionPositiveQueryBudget(t *testing.T) {
	t.Parallel()
	tc := setupSessionsRoute(t)
	device := testpkg.CreateTestDevice(t, tc.db, "current-positive-budget")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Polling activity")
	room := testpkg.CreateTestRoom(t, tc.db, "Polling room")
	staff := testpkg.CreateTestStaff(t, tc.db, "Polling", "Supervisor")
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/", tc.resource.Router())
	start := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/start", map[string]any{
		"activity_id": activity.ID, "room_id": room.ID, "supervisor_ids": []int64{staff.ID},
	}, testutil.WithDeviceContext(device)))
	require.Equal(t, 200, start.Code, start.Body.String())
	groupID := int64(testutil.ParseJSONResponse(t, start.Body.Bytes())["data"].(map[string]any)["active_group_id"].(float64))
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{0, 3} {
		for range n {
			student := testpkg.CreateTestStudent(t, tc.db, "Polling", "Child", "3a")
			testpkg.CreateTestVisit(t, tc.db, student.ID, groupID, time.Now(), nil)
		}
		request := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil, testutil.WithDeviceContext(device))
		request = request.WithContext(counter.Context(request.Context()))
		counter.Reset()
		response := testutil.ExecuteRequest(router, request)
		require.Equal(t, 200, response.Code, response.Body.String())
		data := testutil.ParseJSONResponse(t, response.Body.Bytes())["data"].(map[string]any)
		require.Equal(t, true, data["is_active"])
		assert.Equal(t, float64(groupID), data["active_group_id"])
		assert.Equal(t, activity.Name, data["activity_name"])
		assert.Equal(t, room.Name, data["room_name"])
		assert.Equal(t, float64(n), data["active_students"])
		assert.Equal(t, float64(n), data["active_students_revision"])
		supervisors := data["supervisors"].([]any)
		require.Len(t, supervisors, 1)
		assert.Equal(t, map[string]any{
			"staff_id": float64(staff.ID), "first_name": "Polling", "last_name": "Supervisor",
			"display_name": "Polling Supervisor", "role": "supervisor",
		}, supervisors[0])
		t.Logf("students=%d statements=%d", n, counter.Total())
		querybudgets.AssertQueryBudget(t, "api.iot.session.current.positive", counter.Queries())
	}
}
