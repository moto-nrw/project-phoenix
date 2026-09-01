package students_test

import (
	"context"
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
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Room A has an active group with two students currently checked-in.
	roomA := testpkg.CreateTestRoom(t, tc.db, "RoomFilterA")
	activityA := testpkg.CreateTestActivityGroup(t, tc.db, "RoomFilterActivityA")
	activeGroupA := testpkg.CreateTestActiveGroup(t, tc.db, activityA.ID, roomA.ID)

	studentInRoomA1 := testpkg.CreateTestStudent(t, tc.db, "InRoomA", "Student1", "RFA1")
	studentInRoomA2 := testpkg.CreateTestStudent(t, tc.db, "InRoomA", "Student2", "RFA2")
	studentNotInAnyRoom := testpkg.CreateTestStudent(t, tc.db, "NoRoom", "Student", "RFNR")

	now := time.Now().UTC()
	testpkg.CreateTestVisit(t, tc.db, studentInRoomA1.ID, activeGroupA.ID, now.Add(-10*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, studentInRoomA2.ID, activeGroupA.ID, now.Add(-5*time.Minute), nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d", roomA.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
	t.Parallel()

	tc := setupStudentsRoute(t)

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
	testpkg.CreateTestVisit(t, tc.db, studentInGroupX.ID, activeGroupA.ID, now.Add(-10*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, studentInGroupY.ID, activeGroupA.ID, now.Add(-5*time.Minute), nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d&group_id=%d", roomA.ID, groupX.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
	t.Parallel()

	tc := setupStudentsRoute(t)

	emptyRoom := testpkg.CreateTestRoom(t, tc.db, "EmptyFilterRoom")
	// Create an unrelated student so "all students" would be non-empty if the
	// filter silently failed open.
	testpkg.CreateTestStudent(t, tc.db, "Bystander", "Student", "BYS1")

	req := testutil.NewRequest("GET", fmt.Sprintf("/?room_id=%d", emptyRoom.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data, "empty-room filter must short-circuit to []")
}

func TestListStudents_LocationStateTransit_ReturnsCheckedInStudentsWithoutActiveVisit(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	staff := testpkg.CreateTestStaff(t, tc.db, "TransitFilter", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "transit-filter-device")
	room := testpkg.CreateTestRoom(t, tc.db, "TransitFilterRoom")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "TransitFilterActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)

	transitStudent := testpkg.CreateTestStudent(t, tc.db, "Transit", "Student", "TFS1")
	inRoomStudent := testpkg.CreateTestStudent(t, tc.db, "InRoom", "Student", "TFS2")
	absentStudent := testpkg.CreateTestStudent(t, tc.db, "Absent", "Student", "TFS3")

	now := time.Now().UTC()
	testpkg.CreateTestAttendance(t, tc.db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestAttendance(t, tc.db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-15*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, inRoomStudent.ID, activeGroup.ID, now.Add(-10*time.Minute), nil)

	req := testutil.NewRequest("GET", "/?location_state=transit", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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

	_, hasTransit := ids[transitStudent.ID]
	_, hasInRoom := ids[inRoomStudent.ID]
	_, hasAbsent := ids[absentStudent.ID]

	assert.True(t, hasTransit, "checked-in student without active visit must appear")
	assert.False(t, hasInRoom, "student with active room visit must NOT appear")
	assert.False(t, hasAbsent, "student without open attendance must NOT appear")
}

func TestListStudents_LocationStatePresent_ReturnsStudentsWithOpenAttendance(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	staff := testpkg.CreateTestStaff(t, tc.db, "PresentFilter", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "present-filter-device")
	room := testpkg.CreateTestRoom(t, tc.db, "PresentFilterRoom")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "PresentFilterActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)

	transitStudent := testpkg.CreateTestStudent(t, tc.db, "PresentTransit", "Student", "PFS1")
	inRoomStudent := testpkg.CreateTestStudent(t, tc.db, "PresentRoom", "Student", "PFS2")
	checkedOutStudent := testpkg.CreateTestStudent(t, tc.db, "PresentCheckedOut", "Student", "PFS3")
	absentStudent := testpkg.CreateTestStudent(t, tc.db, "PresentAbsent", "Student", "PFS4")

	now := time.Now().UTC()
	testpkg.CreateTestAttendance(t, tc.db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestAttendance(t, tc.db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-15*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	testpkg.CreateTestAttendance(t, tc.db, checkedOutStudent.ID, staff.ID, device.ID, now.Add(-15*time.Minute), &checkOutTime)
	testpkg.CreateTestVisit(t, tc.db, inRoomStudent.ID, activeGroup.ID, now.Add(-10*time.Minute), nil)

	req := testutil.NewRequest("GET", "/?location_state=present", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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

	_, hasTransit := ids[transitStudent.ID]
	_, hasInRoom := ids[inRoomStudent.ID]
	_, hasCheckedOut := ids[checkedOutStudent.ID]
	_, hasAbsent := ids[absentStudent.ID]

	assert.True(t, hasTransit, "checked-in student without active visit must appear")
	assert.True(t, hasInRoom, "checked-in student with active visit must appear")
	assert.False(t, hasCheckedOut, "checked-out student must NOT appear")
	assert.False(t, hasAbsent, "student without attendance must NOT appear")
}

func TestListStudents_LocationStateTransitWithGroupID_IntersectsFilters(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, tc.db, "TransitGroupFilter", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "transit-group-filter-device")
	targetGroup := testpkg.CreateTestEducationGroup(t, tc.db, "TransitGroupFilterTarget")
	otherGroup := testpkg.CreateTestEducationGroup(t, tc.db, "TransitGroupFilterOther")

	matchingTransitStudent := testpkg.CreateTestStudent(t, tc.db, "MatchingTransit", "Student", "TGFS1")
	otherGroupTransitStudent := testpkg.CreateTestStudent(t, tc.db, "OtherGroupTransit", "Student", "TGFS2")
	matchingAbsentStudent := testpkg.CreateTestStudent(t, tc.db, "MatchingAbsent", "Student", "TGFS3")

	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("group_id = ?", targetGroup.ID).
		Where("id = ?", matchingTransitStudent.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("group_id = ?", otherGroup.ID).
		Where("id = ?", otherGroupTransitStudent.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("group_id = ?", targetGroup.ID).
		Where("id = ?", matchingAbsentStudent.ID).
		Exec(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	testpkg.CreateTestAttendance(t, tc.db, matchingTransitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestAttendance(t, tc.db, otherGroupTransitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?location_state=transit&group_id=%d", targetGroup.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	require.Len(t, resp.Data, 1)
	assert.Equal(t, matchingTransitStudent.ID, resp.Data[0].ID)
}

func TestListStudents_LocationStateTransit_InvalidFilters(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "unknown location state",
			path: "/?location_state=room",
		},
		{
			name: "transit cannot be combined with room",
			path: "/?location_state=transit&room_id=42",
		},
		{
			name: "present cannot be combined with room",
			path: "/?location_state=present&room_id=42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testutil.NewRequest("GET", tt.path, nil)
			rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

			assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
		})
	}
}

func TestListStudents_LocationStateTransit_EmptyReturnsEmpty(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	bystander := testpkg.CreateTestStudent(t, tc.db, "TransitEmpty", "Bystander", "TEB1")

	req := testutil.NewRequest("GET", "/?location_state=transit", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
	_, hasBystander := ids[bystander.ID]
	assert.False(t, hasBystander, "student without open attendance must not appear")
}

func TestListStudents_LocationStateTransitWithGroupID_NoIntersectionReturnsEmpty(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, tc.db, "TransitNoMatch", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "transit-no-match-device")
	studentGroup := testpkg.CreateTestEducationGroup(t, tc.db, "TransitNoMatchStudentGroup")
	filterGroup := testpkg.CreateTestEducationGroup(t, tc.db, "TransitNoMatchFilterGroup")
	transitStudent := testpkg.CreateTestStudent(t, tc.db, "TransitNoMatch", "Student", "TNMS1")

	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("group_id = ?", studentGroup.ID).
		Where("id = ?", transitStudent.ID).
		Exec(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	testpkg.CreateTestAttendance(t, tc.db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/?location_state=transit&group_id=%d", filterGroup.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data)
}
