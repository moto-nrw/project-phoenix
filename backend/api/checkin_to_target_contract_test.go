package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// checkCheckInToTarget drives the app's check-in straight into a room or an
// activity (#6953) through the assembled production router: both routes need
// the check-in and the move right, write attendance and room stay in one
// unit, and leave nothing behind when the placement is refused.
func checkCheckInToTarget(t *testing.T, graph *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureWebManualDevice(t, db)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Anmelden", "Team")
	colleague := testpkg.CreateTestStaff(t, db, "Anmelden", "Aufsicht")
	supervised := testpkg.CreateTestActiveGroup(t, db,
		testpkg.CreateTestActivityGroup(t, db, "Anmelden Hausaufgaben").ID,
		testpkg.CreateTestRoom(t, db, "Anmelden Klassenraum").ID)
	testpkg.CreateTestGroupSupervisor(t, db, colleague.ID, supervised.ID, "supervisor")
	unsupervised := testpkg.CreateTestActiveGroup(t, db,
		testpkg.CreateTestActivityGroup(t, db, "Anmelden Ohne Aufsicht").ID,
		testpkg.CreateTestRoom(t, db, "Anmelden Werkraum").ID)
	gym := testpkg.CreateTestOpenRoom(t, db, "Anmelden Turnhalle")

	token := func(permissions ...string) string {
		return testpkg.MintLiveSessionJWT(t, db, sessiontoken.AppClaims{
			ID: int(account.ID), Sub: account.Email, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Permissions: permissions,
		})
	}
	staff := token("users:checkin", "visits:update")
	post := func(path, body, bearer string) (int, string) {
		recorder := checkpointRequest(graph, checkpointScenario{Method: http.MethodPost, Path: path, Authenticated: true, Body: body}, bearer)
		return recorder.Code, recorder.Body.String()
	}
	openAttendance := func(studentID int64) int {
		count, err := db.NewSelect().Table("active.attendance").
			Where("student_id = ? AND check_out_time IS NULL", studentID).Count(context.Background())
		require.NoError(t, err)
		return int(count)
	}

	child := testpkg.CreateTestStudent(t, db, "Mila", "Anmelden", "2a").ID
	groupBody := fmt.Sprintf(`{"student_ids":[%d],"target_active_group_id":%d}`, child, supervised.ID)

	code, body := post("/api/active/visits/check-in-to-group", groupBody, token("visits:update"))
	require.Equal(t, http.StatusForbidden, code, "the check-in right is required: %s", body)
	assert.Zero(t, openAttendance(child))

	code, body = post("/api/active/visits/check-in-to-group",
		fmt.Sprintf(`{"student_ids":[%d],"target_active_group_id":%d}`, child, unsupervised.ID), staff)
	require.Equal(t, http.StatusForbidden, code, "a room without supervision is no target: %s", body)
	assert.Zero(t, openAttendance(child), "a refused placement leaves no check-in behind")

	code, body = post("/api/active/visits/check-in-to-group", groupBody, staff)
	require.Equal(t, http.StatusOK, code, body)
	var placed struct {
		Data struct {
			Moved     []int64 `json:"moved"`
			CheckedIn []int64 `json:"checked_in"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &placed))
	assert.Equal(t, []int64{child}, placed.Data.CheckedIn)
	assert.Equal(t, []int64{child}, placed.Data.Moved)
	assert.Equal(t, 1, openAttendance(child))
	assert.Equal(t, []int64{supervised.ID}, openVisitSessions(t, db, child), "the child is in the room, not in transit")

	yardChild := testpkg.CreateTestStudent(t, db, "Jonas", "Anmelden", "2a").ID
	code, body = post("/api/active/visits/check-in-to-room",
		fmt.Sprintf(`{"student_ids":[%d],"target_room_id":%d}`, yardChild, gym.ID), staff)
	require.Equal(t, http.StatusOK, code, body)
	var stay struct {
		Data struct {
			CheckedIn     []int64 `json:"checked_in"`
			ActiveGroupID int64   `json:"active_group_id"`
			RoomID        int64   `json:"room_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &stay))
	assert.Equal(t, []int64{yardChild}, stay.Data.CheckedIn)
	assert.Equal(t, gym.ID, stay.Data.RoomID)
	assert.Equal(t, []int64{stay.Data.ActiveGroupID}, openVisitSessions(t, db, yardChild),
		"a released room needs no supervision")
}
