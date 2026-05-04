package rooms_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// GET /api/rooms/{id}/students-present
// =============================================================================
//
// These tests cover the GDPR-redaction surface of the new endpoint:
//
//  1. Admin → full names regardless of scope
//  2. all_staff scope → full names for any authenticated staff
//  3. group_supervisors_only scope → unredacted for supervised, null for the rest
//  4. Empty room → total_count = visible_count = 0
//  5. Invalid room id in URL → 400
//
// Per the plan, redacted rows still appear (with null PII fields) so that
// total_count and the SSE row-diff key (student_id) stay consistent.

// setStudentDataScopeRooms overrides gdpr.student_data_scope for tenant 1 and
// registers a cleanup. Mirrors the helper in api/students/authorization_test.go
// — keeping it local to rooms_test avoids exporting test internals.
func setStudentDataScopeRooms(t *testing.T, tc *testContext, scope string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	err := tc.services.Settings.SetValue(ctx, configModel.KeyStudentDataScope, scope, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyStudentDataScope, nil, nil)
	})
}

// studentsInRoomFixture wires up the standard scenario used by most of the
// tests below: one room, one active group taking place in that room, two
// students from two different education groups, both currently checked-in.
type studentsInRoomFixture struct {
	roomID                 int64
	activityGroupID        int64
	activeGroupID          int64
	supervisedEduGroupID   int64
	supervisedEduGroupName string
	unrelatedEduGroupID    int64
	supervisedStudentID    int64
	unrelatedStudentID     int64
	supervisedStudentName  struct{ first, last string }
	unrelatedStudentName   struct{ first, last string }
	cleanup                func()
}

func setupStudentsInRoomFixture(t *testing.T, tc *testContext) *studentsInRoomFixture {
	t.Helper()

	room := testpkg.CreateTestRoom(t, tc.db, "Lesezimmer")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "Lesegruppe")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	supervisedGroup := testpkg.CreateTestEducationGroup(t, tc.db, "SupervisedClass")
	unrelatedGroup := testpkg.CreateTestEducationGroup(t, tc.db, "UnrelatedClass")

	supervisedStudent := testpkg.CreateTestStudent(t, tc.db, "Anna", "Schmidt", "1a")
	unrelatedStudent := testpkg.CreateTestStudent(t, tc.db, "Bob", "Mueller", "1b")

	testpkg.AssignStudentToGroup(t, tc.db, supervisedStudent.ID, supervisedGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, unrelatedStudent.ID, unrelatedGroup.ID)

	now := time.Now().UTC()
	supervisedVisit := testpkg.CreateTestVisit(t, tc.db, supervisedStudent.ID, activeGroup.ID, now.Add(-10*time.Minute), nil)
	unrelatedVisit := testpkg.CreateTestVisit(t, tc.db, unrelatedStudent.ID, activeGroup.ID, now.Add(-5*time.Minute), nil)

	fx := &studentsInRoomFixture{
		roomID:                 room.ID,
		activityGroupID:        activityGroup.ID,
		activeGroupID:          activeGroup.ID,
		supervisedEduGroupID:   supervisedGroup.ID,
		supervisedEduGroupName: supervisedGroup.Name,
		unrelatedEduGroupID:    unrelatedGroup.ID,
		supervisedStudentID:    supervisedStudent.ID,
		unrelatedStudentID:     unrelatedStudent.ID,
	}
	fx.supervisedStudentName.first, fx.supervisedStudentName.last = "Anna", "Schmidt"
	fx.unrelatedStudentName.first, fx.unrelatedStudentName.last = "Bob", "Mueller"

	fx.cleanup = func() {
		testpkg.CleanupActivityFixtures(
			t, tc.db,
			supervisedVisit.ID, unrelatedVisit.ID,
			activeGroup.ID,
			supervisedStudent.ID, unrelatedStudent.ID,
			supervisedGroup.ID, unrelatedGroup.ID,
			activityGroup.ID, room.ID,
		)
	}
	return fx
}

// readStudentsPresentResponse decodes the standard response envelope used by
// this codebase ({ "data": ..., "message": ... }) into the typed payload.
func readStudentsPresentResponse(t *testing.T, body []byte) roomsAPI.StudentsPresentInRoomResponse {
	t.Helper()
	var env struct {
		Data roomsAPI.StudentsPresentInRoomResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env), "Body: %s", string(body))
	return env.Data
}

func TestListStudentsPresent_Admin_SeesEveryone(t *testing.T) {
	tc := setupTestContext(t)
	fx := setupStudentsInRoomFixture(t, tc)
	defer fx.cleanup()

	// Default scope is group_supervisors_only — admins should bypass it
	// regardless because of admin:* in their permissions.
	setStudentDataScopeRooms(t, tc, configModel.StudentDataScopeGroupSupervisorsOnly)

	router := setupRouter(tc.resource.ListStudentsPresent, "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", fx.roomID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*", "rooms:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	resp := readStudentsPresentResponse(t, rr.Body.Bytes())
	assert.Equal(t, fx.roomID, resp.RoomID)
	assert.Equal(t, 2, resp.TotalCount)
	assert.Equal(t, 2, resp.VisibleCount)
	require.Len(t, resp.Students, 2)
	for _, s := range resp.Students {
		assert.True(t, s.HasFullAccess, "admin should see every row unredacted")
		require.NotNil(t, s.FirstName)
		require.NotNil(t, s.LastName)
		require.NotNil(t, s.ActiveGroupID)
		require.NotNil(t, s.EntryTime)
	}
}

func TestListStudentsPresent_AllStaffScope_SeesEveryone(t *testing.T) {
	tc := setupTestContext(t)
	fx := setupStudentsInRoomFixture(t, tc)
	defer fx.cleanup()

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

	setStudentDataScopeRooms(t, tc, configModel.StudentDataScopeAllStaff)

	router := setupRouter(tc.resource.ListStudentsPresent, "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", fx.roomID), nil)
	rr := executeWithAuth(router, req, testutil.TeacherTestClaims(int(account.ID)), []string{"rooms:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	resp := readStudentsPresentResponse(t, rr.Body.Bytes())
	assert.Equal(t, 2, resp.TotalCount)
	assert.Equal(t, 2, resp.VisibleCount, "all_staff scope grants full access to any verified staff")
	require.Len(t, resp.Students, 2)
	for _, s := range resp.Students {
		assert.True(t, s.HasFullAccess)
	}
}

func TestListStudentsPresent_GroupSupervisorsOnly_RedactsNonSupervised(t *testing.T) {
	tc := setupTestContext(t)
	fx := setupStudentsInRoomFixture(t, tc)
	defer fx.cleanup()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Super", "Visor")
	testpkg.CreateTestGroupTeacher(t, tc.db, fx.supervisedEduGroupID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

	setStudentDataScopeRooms(t, tc, configModel.StudentDataScopeGroupSupervisorsOnly)

	router := setupRouter(tc.resource.ListStudentsPresent, "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", fx.roomID), nil)
	rr := executeWithAuth(router, req, testutil.TeacherTestClaims(int(account.ID)), []string{"rooms:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	resp := readStudentsPresentResponse(t, rr.Body.Bytes())
	assert.Equal(t, 2, resp.TotalCount, "true count must include redacted rows")
	assert.Equal(t, 1, resp.VisibleCount, "only the supervised student should be unredacted")
	require.Len(t, resp.Students, 2)

	byID := make(map[int64]roomsAPI.StudentPresentResponse, len(resp.Students))
	for _, s := range resp.Students {
		byID[s.StudentID] = s
	}

	supervised := byID[fx.supervisedStudentID]
	require.True(t, supervised.HasFullAccess, "supervised student must be unredacted")
	require.NotNil(t, supervised.FirstName)
	assert.Equal(t, fx.supervisedStudentName.first, *supervised.FirstName)
	require.NotNil(t, supervised.LastName)
	assert.Equal(t, fx.supervisedStudentName.last, *supervised.LastName)
	require.NotNil(t, supervised.ActiveGroupID)
	assert.Equal(t, fx.activeGroupID, *supervised.ActiveGroupID)
	require.NotNil(t, supervised.EntryTime)
	require.NotNil(t, supervised.GroupName, "group_name must be the student's Stammgruppe, not the activity")
	assert.Equal(t, fx.supervisedEduGroupName, *supervised.GroupName)

	unrelated := byID[fx.unrelatedStudentID]
	require.False(t, unrelated.HasFullAccess, "non-supervised student must be redacted")
	assert.Nil(t, unrelated.FirstName, "first_name must be null for redacted rows")
	assert.Nil(t, unrelated.LastName, "last_name must be null for redacted rows")
	assert.Nil(t, unrelated.ActiveGroupID, "active_group_id must be null for redacted rows")
	assert.Nil(t, unrelated.GroupName, "group_name must be null for redacted rows")
	assert.Nil(t, unrelated.EntryTime, "entry_time must be null for redacted rows")
	assert.Equal(t, fx.unrelatedStudentID, unrelated.StudentID, "student_id must survive redaction so SSE can diff rows")
}

func TestListStudentsPresent_EmptyRoom_ReturnsZeroCount(t *testing.T) {
	tc := setupTestContext(t)

	room := testpkg.CreateTestRoom(t, tc.db, "EmptyRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	router := setupRouter(tc.resource.ListStudentsPresent, "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*", "rooms:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	resp := readStudentsPresentResponse(t, rr.Body.Bytes())
	assert.Equal(t, room.ID, resp.RoomID)
	assert.Equal(t, 0, resp.TotalCount)
	assert.Equal(t, 0, resp.VisibleCount)
	assert.Empty(t, resp.Students)
	assert.False(t, resp.AsOf.IsZero(), "AsOf must always be populated so the client can render a freshness label")
}

func TestListStudentsPresent_InvalidID_Returns400(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.ListStudentsPresent, "id")
	req := testutil.NewRequest("GET", "/not-a-number", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*", "rooms:read"})

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}
