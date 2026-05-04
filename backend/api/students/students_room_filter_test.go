package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListStudents_RoomFilter verifies that ?room_id={id} narrows the list to
// students currently checked-in to any active group in that room. The filter
// is the backend half of the "In Kindersuche öffnen" link from the room
// detail page (#1323).
func TestListStudents_RoomFilter_ReturnsOnlyStudentsInRoom(t *testing.T) {
	tc := setupTestContext(t)

	// Room A has an active group with two students currently checked-in.
	roomA := testpkg.CreateTestRoom(t, tc.db, "RoomFilterA")
	activityA := testpkg.CreateTestActivityGroup(t, tc.db, "RoomFilterActivityA")
	activeGroupA := testpkg.CreateTestActiveGroup(t, tc.db, activityA.ID, roomA.ID)

	studentInRoomA1 := testpkg.CreateTestStudent(t, tc.db, "InRoomA", "Student1", "RFA1")
	studentInRoomA2 := testpkg.CreateTestStudent(t, tc.db, "InRoomA", "Student2", "RFA2")
	studentNotInAnyRoom := testpkg.CreateTestStudent(t, tc.db, "NoRoom", "Student", "RFNR")

	now := time.Now().UTC()
	visitA1 := testpkg.CreateTestVisit(t, tc.db, studentInRoomA1.ID, activeGroupA.ID, now.Add(-10*time.Minute), nil)
	visitA2 := testpkg.CreateTestVisit(t, tc.db, studentInRoomA2.ID, activeGroupA.ID, now.Add(-5*time.Minute), nil)

	defer testpkg.CleanupActivityFixtures(
		t, tc.db,
		visitA1.ID, visitA2.ID,
		activeGroupA.ID,
		studentInRoomA1.ID, studentInRoomA2.ID, studentNotInAnyRoom.ID,
		activityA.ID, roomA.ID,
	)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")
	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d", roomA.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	ids := make(map[int64]struct{}, len(resp.Data))
	for _, s := range resp.Data {
		ids[s.ID] = struct{}{}
	}

	_, hasA1 := ids[studentInRoomA1.ID]
	_, hasA2 := ids[studentInRoomA2.ID]
	_, hasOutsider := ids[studentNotInAnyRoom.ID]

	assert.True(t, hasA1, "student in room A must appear in filtered list")
	assert.True(t, hasA2, "student in room A must appear in filtered list")
	assert.False(t, hasOutsider, "student outside the room must NOT appear")
}

// TestListStudents_RoomFilter_IntersectsWithGroupFilter covers the case where
// the user opens Kindersuche from a room and then narrows by group. The result
// must satisfy BOTH filters — otherwise the list contradicts the active group
// chip in the UI (regression from the initial #1323 implementation).
func TestListStudents_RoomFilter_IntersectsWithGroupFilter(t *testing.T) {
	tc := setupTestContext(t)

	roomA := testpkg.CreateTestRoom(t, tc.db, "RoomFilterIntersect")
	activityA := testpkg.CreateTestActivityGroup(t, tc.db, "RoomFilterIntersectActivity")
	activeGroupA := testpkg.CreateTestActiveGroup(t, tc.db, activityA.ID, roomA.ID)

	groupX := testpkg.CreateTestEducationGroup(t, tc.db, "RoomFilterEduGroupX")
	groupY := testpkg.CreateTestEducationGroup(t, tc.db, "RoomFilterEduGroupY")

	studentInGroupX := testpkg.CreateTestStudent(t, tc.db, "Intersect", "InGroupX", "RFIX")
	studentInGroupY := testpkg.CreateTestStudent(t, tc.db, "Intersect", "InGroupY", "RFIY")
	testpkg.AssignStudentToGroup(t, tc.db, studentInGroupX.ID, groupX.ID)
	testpkg.AssignStudentToGroup(t, tc.db, studentInGroupY.ID, groupY.ID)

	now := time.Now().UTC()
	visitX := testpkg.CreateTestVisit(t, tc.db, studentInGroupX.ID, activeGroupA.ID, now.Add(-10*time.Minute), nil)
	visitY := testpkg.CreateTestVisit(t, tc.db, studentInGroupY.ID, activeGroupA.ID, now.Add(-5*time.Minute), nil)

	defer testpkg.CleanupActivityFixtures(
		t, tc.db,
		visitX.ID, visitY.ID,
		activeGroupA.ID,
		studentInGroupX.ID, studentInGroupY.ID,
		activityA.ID, roomA.ID,
		groupX.ID, groupY.ID,
	)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")
	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d&group_id=%d", roomA.ID, groupX.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	ids := make(map[int64]struct{}, len(resp.Data))
	for _, s := range resp.Data {
		ids[s.ID] = struct{}{}
	}

	_, hasX := ids[studentInGroupX.ID]
	_, hasY := ids[studentInGroupY.ID]

	assert.True(t, hasX, "student in room A AND group X must appear")
	assert.False(t, hasY, "student in room A but group Y must NOT appear when group_id=X")
}

// TestListStudents_RoomFilter_EmptyRoomReturnsEmpty covers the case where the
// room has no active groups (or none with open visits). The endpoint should
// 200 with an empty data array, not error or fall back to "all students".
func TestListStudents_RoomFilter_EmptyRoomReturnsEmpty(t *testing.T) {
	tc := setupTestContext(t)

	emptyRoom := testpkg.CreateTestRoom(t, tc.db, "EmptyFilterRoom")
	// Create an unrelated student so "all students" would be non-empty if the
	// filter silently failed open.
	bystander := testpkg.CreateTestStudent(t, tc.db, "Bystander", "Student", "BYS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, bystander.ID, emptyRoom.ID)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")
	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d", emptyRoom.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data, "empty-room filter must short-circuit to []")
}
