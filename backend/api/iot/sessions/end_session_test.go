package sessions_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The kiosk close goes through the session end workflow: one request ends the
// presence session (visits, supervision, group) and completes the mirrored
// timetable instance with its attendance, all inside the request transaction
// (#2697). A second close finds no running session.
func TestEndSession_ClosesPresenceAndMirroredInstance(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)
	db := ctx.db

	testDevice := testpkg.CreateTestDevice(t, db, "sessions-test-device-end-1")
	activity := testpkg.CreateTestActivityGroup(t, db, "End Session Activity")
	room := testpkg.CreateTestRoom(t, db, "End Session Room")
	staff := testpkg.CreateTestStaff(t, db, "EndSession", "Supervisor")
	router := testutil.NewTenantRouter(db)
	router.Mount("/", ctx.resource.Router())

	startRR := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/start", map[string]interface{}{
		"activity_id":    activity.ID,
		"room_id":        room.ID,
		"supervisor_ids": []int64{staff.ID},
	}, testutil.WithDeviceContext(testDevice)))
	require.Equal(t, http.StatusOK, startRR.Code, startRR.Body.String())
	startData, ok := testutil.ParseJSONResponse(t, startRR.Body.Bytes())["data"].(map[string]interface{})
	require.True(t, ok, "start response carries the session")
	groupID := int64(startData["active_group_id"].(float64))

	present := testpkg.CreateTestStudent(t, db, "Present", "Child", "2a")
	expected := testpkg.CreateTestStudent(t, db, "Expected", "Child", "2a")
	checkedIn := time.Now().Add(-30 * time.Minute)
	visit := testpkg.CreateTestVisit(t, db, present.ID, groupID, checkedIn, nil)
	activityID := activity.ID
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Status: "active", ActivityGroupID: &activityID, ActiveGroupID: &groupID, IsSpontaneous: true, Title: "Mirrored kiosk session",
	})
	presentRow := testpkg.CreateTestInstanceStudent(t, db, instance.ID, present.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &checkedIn})
	expectedRow := testpkg.CreateTestInstanceStudent(t, db, instance.ID, expected.ID, "expected")

	// Another school's running block with the same shape: the close must not
	// finalize its attendance or complete its instance.
	var foreignInstanceID, foreignRowID int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignActivity := testpkg.CreateTestActivityGroup(t, db, "Foreign Session Activity")
		foreignRoom := testpkg.CreateTestRoom(t, db, "Foreign Session Room")
		foreignGroup := testpkg.CreateTestActiveGroup(t, db, foreignActivity.ID, foreignRoom.ID)
		foreignStudent := testpkg.CreateTestStudent(t, db, "Foreign", "Child", "2a")
		foreignActivityID, foreignGroupID := foreignActivity.ID, foreignGroup.ID
		foreignInstance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), foreignRoom.ID, testpkg.ActivityInstanceOpts{
			Status: "active", ActivityGroupID: &foreignActivityID, ActiveGroupID: &foreignGroupID, IsSpontaneous: true, Title: "Foreign kiosk session",
		})
		foreignInstanceID = foreignInstance.ID
		foreignRowID = testpkg.CreateTestInstanceStudent(t, db, foreignInstance.ID, foreignStudent.ID, "expected").ID
	})

	endRR := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/end", nil, testutil.WithDeviceContext(testDevice)))
	testutil.AssertSuccessResponse(t, endRR, http.StatusOK)
	endData, ok := testutil.ParseJSONResponse(t, endRR.Body.Bytes())["data"].(map[string]interface{})
	require.True(t, ok, "end response carries the session summary")
	assert.Equal(t, float64(groupID), endData["active_group_id"])
	assert.Equal(t, "ended", endData["status"])

	currentRR := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", "/current", nil, testutil.WithDeviceContext(testDevice)))
	testutil.AssertSuccessResponse(t, currentRR, http.StatusOK)
	currentData, ok := testutil.ParseJSONResponse(t, currentRR.Body.Bytes())["data"].(map[string]interface{})
	require.True(t, ok, "current response carries the device state")
	assert.Equal(t, false, currentData["is_active"], "the device has no running session any more")
	require.NotNil(t, testpkg.VisitExitTime(t, db, visit.ID), "the child's visit was checked out")
	assert.Equal(t, "completed", testpkg.InstanceStatus(t, db, instance.ID), "the mirrored instance completed with the session")
	presentAfter := testpkg.InstanceStudentByID(t, db, presentRow.ID)
	assert.NotNil(t, presentAfter.CheckedOutAt, "the present child's slot check-out was stamped")
	expectedAfter := testpkg.InstanceStudentByID(t, db, expectedRow.ID)
	assert.True(t, expectedAfter.Status == "absent" || (expectedAfter.Status == "expected" && expectedAfter.NotScheduled),
		"an expected child is finalized at the close: absent, or kept expected only as a marked non-booking; got status %q not_scheduled=%v",
		expectedAfter.Status, expectedAfter.NotScheduled)

	assert.Equal(t, "active", testpkg.InstanceStatus(t, db, foreignInstanceID), "the other school's block keeps running")
	foreignAfter := testpkg.InstanceStudentByID(t, db, foreignRowID)
	assert.Equal(t, "expected", foreignAfter.Status)
	assert.False(t, foreignAfter.NotScheduled, "the other school's attendance is untouched")

	againRR := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/end", nil, testutil.WithDeviceContext(testDevice)))
	testutil.AssertBadRequest(t, againRR)
}
