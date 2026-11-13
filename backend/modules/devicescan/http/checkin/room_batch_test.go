package checkin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Batched room metadata must not turn the room's capacity or response count
// into a count of only the selected session, or reuse the prewrite occupancy.
func TestDeviceCheckin_RoomChangeCountsOtherSessions(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{2, 3} {
		t.Run(fmt.Sprintf("capacity_%d", capacity), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			k := setupCheckinRoute(t)
			staff, _, _ := k.staff(t, "Room", "Batch")
			source := testpkg.CreateTestRoom(t, k.db, "Batch Source")
			activity := testpkg.CreateTestActivityGroup(t, k.db, "Batch Source Activity")
			sourceSession := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, source.ID)
			target := testpkg.CreateTestRoomWithCapacity(t, k.db, "Batch Target", capacity)
			for i := range 2 {
				activity := testpkg.CreateTestActivityGroup(t, k.db, fmt.Sprintf("Batch Activity %d", i))
				session := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, target.ID)
				peer := testpkg.CreateTestStudent(t, k.db, fmt.Sprintf("Peer%d", i), "Batch", "1a")
				testpkg.CreateTestVisit(t, k.db, peer.ID, session.ID, time.Now().Add(-time.Hour), nil)
			}
			tag, student, _ := k.studentCard(t, "Moving", "Batch", "1a")
			old := testpkg.CreateTestVisit(t, k.db, student, sourceSession.ID, time.Now().Add(-time.Hour), nil)
			rr := k.call(t, "POST", "/checkin", checkinBody(tag, target.ID), k.device(t, "room-batch"), staff)
			persisted, err := k.presence(t).FindVisit(testpkg.Ctx(t), old.ID)
			require.NoError(t, err)
			visits, err := k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{student}, OpenOnly: true})
			require.NoError(t, err)
			require.Len(t, visits, 1)
			if capacity == 2 {
				require.Equal(t, 409, rr.Code, rr.Body.String())
				body := testutil.ParseJSONResponse(t, rr.Body.Bytes())
				assert.Equal(t, "iot.room_capacity_exceeded", body["code"])
				assert.Equal(t, "Room capacity exceeded", body["detail"])
				details, ok := body["details"].(map[string]any)
				require.True(t, ok)
				assert.EqualValues(t, 2, details["current_occupancy"])
				assert.EqualValues(t, 2, details["max_capacity"])
				assert.EqualValues(t, target.ID, details["room_id"])
				assert.Equal(t, target.Name, details["room_name"])
				assert.Nil(t, persisted.ExitTime, "refused transfer restores the source stay")
				assert.Equal(t, old.ID, visits[0].ID)
				assert.Equal(t, sourceSession.ID, visits[0].ActiveGroupID)
				return
			}
			testutil.AssertSuccessResponse(t, rr, 200)
			data := responseData(t, rr.Body.Bytes())
			assert.Equal(t, "transferred", data["action"])
			assert.Equal(t, source.Name, data["previous_room"])
			assert.Equal(t, target.Name, data["room_name"])
			assert.EqualValues(t, visits[0].ID, data["visit_id"])
			assert.EqualValues(t, 3, data["active_students"], "postwrite room count includes both sessions and the arrival")
			assert.EqualValues(t, target.ID, data["active_students_room_id"])
			assert.NotNil(t, persisted.ExitTime)
			assert.NotEqual(t, old.ID, visits[0].ID)
		})
	}
}
