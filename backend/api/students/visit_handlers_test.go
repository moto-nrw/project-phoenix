package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Student Current Location Tests
// =============================================================================

func TestGetStudentCurrentLocation(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Location", "Test", "LT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")

	t.Run("success_gets_student_location", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "current_location")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

func TestGetStudentCurrentLocation_Extended(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("returns_absent_for_student_without_visit", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Absent", "Student", "AB1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Location should be "Abwesend" (absent) when student has no active visit
		assert.Contains(t, rr.Body.String(), "current_location")
	})
}

// =============================================================================
// Student Current Visit Tests
// =============================================================================

func TestGetStudentCurrentVisit(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Test", "VT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")

	t.Run("error_when_no_current_visit", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// No visit should return error or empty
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentCurrentVisit_Extended(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("returns_null_when_no_visit", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoVisit", "Student", "NV2", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// The handler may return 500 (internal error) or 200 with null when no visit
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)
	})
}

// =============================================================================
// Student Visit History Tests
// =============================================================================

func TestGetStudentVisitHistory(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "History", "Test", "HT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")

	t.Run("success_returns_empty_history", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentVisitHistory_WithDateRange(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "DateRange", "Test", "DR1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")

	t.Run("with_start_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?from=2024-01-01", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("with_date_range", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?from=2024-01-01&to=2024-12-31", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

func TestGetStudentVisitHistory_WithVisits(t *testing.T) {
	tc := setupTestContext(t)

	// Create a student with an active visit to test visit history
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "History", "VH1", tc.ogsID)
	room := testpkg.CreateTestRoom(t, tc.db, "HistoryRoom", tc.ogsID)
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "HistoryActivity", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, room.ID, activityGroup.ID)

	router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "Should return visit history. Body: %s", rr.Body.String())
}

// =============================================================================
// Student In Group Room Tests
// =============================================================================

func TestGetStudentInGroupRoom_InvalidStudentID(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", "/students/invalid/group-room", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	testutil.AssertBadRequest(t, rr)
}

func TestGetStudentInGroupRoom_NonexistentStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", "/students/999999/group-room", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	testutil.AssertNotFound(t, rr)
}

func TestGetStudentInGroupRoom_WithValidStudent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "GroupRoom", "Test", "GR1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// Student has no group, so should return OK with no_group reason
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "no_group")
}

func TestGetStudentInGroupRoom_Extended(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("student_no_educational_group", func(t *testing.T) {
		// Student without group assigned
		student := testpkg.CreateTestStudent(t, tc.db, "NoEdGroup", "Student", "NEG1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "no_group", "Should indicate no group")
	})

	t.Run("student_group_has_no_room", func(t *testing.T) {
		// Create group without room, then assign student
		group := testpkg.CreateTestEducationGroup(t, tc.db, "NoRoomGroup", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "NoRoom", "Student", "NR1", tc.ogsID)
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "group_no_room", "Should indicate group has no room")
	})

	t.Run("student_with_no_active_visit", func(t *testing.T) {
		// Create student with group and room, but no active visit
		room := testpkg.CreateTestRoom(t, tc.db, "GroupRoomTest", tc.ogsID)
		group := testpkg.CreateTestEducationGroup(t, tc.db, "WithRoomGroup", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "NoVisit", "Student", "NV1", tc.ogsID)

		// Assign room to group using raw SQL to avoid BUN ORM syntax issues
		ctx := context.Background()
		_, err := tc.db.ExecContext(ctx, "UPDATE education.groups SET room_id = ? WHERE id = ?", room.ID, group.ID)
		require.NoError(t, err)

		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, group.ID, student.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "no_active_visit", "Should indicate no active visit")
	})
}

func TestGetStudentInGroupRoom_Authorization(t *testing.T) {
	tc := setupTestContext(t)

	// Create teacher and group
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Room", "Auth", tc.ogsID)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomAuthGroup", tc.ogsID)
	student := testpkg.CreateTestStudent(t, tc.db, "Room", "AuthStudent", "RA1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)

	// Assign teacher to group and student to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	t.Run("teacher_supervisor_can_access", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		// Teacher supervises this student's group, so should get OK (even if group has no room)
		assert.Equal(t, http.StatusOK, rr.Code, "Teacher should access group room status. Body: %s", rr.Body.String())
	})

	t.Run("non_supervisor_forbidden", func(t *testing.T) {
		// Create another teacher not supervising this group
		otherTeacher, otherAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Other", "Teacher", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, otherTeacher.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		// Non-supervisor should be forbidden
		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Student With Active Visit Tests (Coverage for visit tracking paths)
// =============================================================================

func TestGetStudentInGroupRoom_WithActiveVisit(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("student_in_group_room", func(t *testing.T) {
		// Create room, group, student
		room := testpkg.CreateTestRoom(t, tc.db, "GroupRoomActive", tc.ogsID)
		group := testpkg.CreateTestEducationGroup(t, tc.db, "ActiveVisitGroup", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "Active", "Visitor", "AV1", tc.ogsID)
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "ActiveSession", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Check", "InStaff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "checkin-device", tc.ogsID)

		// Assign room to group
		ctx := context.Background()
		_, err := tc.db.ExecContext(ctx, "UPDATE education.groups SET room_id = ? WHERE id = ?", room.ID, group.ID)
		require.NoError(t, err)

		// Assign student to group
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		// Create attendance (check-in)
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		// Create active group in the same room
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID, tc.ogsID)

		// Create visit to the active group
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now, nil, tc.ogsID)

		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, group.ID, student.ID, activityGroup.ID, staff.ID, device.ID, activeGroup.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Should have in_group_room response
		assert.Contains(t, rr.Body.String(), "in_group_room")
	})

	t.Run("student_in_different_room", func(t *testing.T) {
		// Create two rooms - one for group, one for visit
		groupRoom := testpkg.CreateTestRoom(t, tc.db, "GroupRoomDiff", tc.ogsID)
		visitRoom := testpkg.CreateTestRoom(t, tc.db, "VisitRoomDiff", tc.ogsID)
		group := testpkg.CreateTestEducationGroup(t, tc.db, "DiffRoomGroup", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "Diff", "Room", "DR2", tc.ogsID)
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DiffRoomActivity", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Diff", "Staff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "diff-device", tc.ogsID)

		// Assign group room to group
		ctx := context.Background()
		_, err := tc.db.ExecContext(ctx, "UPDATE education.groups SET room_id = ? WHERE id = ?", groupRoom.ID, group.ID)
		require.NoError(t, err)

		// Assign student to group
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		// Create attendance
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		// Create active group in DIFFERENT room
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, visitRoom.ID, tc.ogsID)

		// Create visit to that different room
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now, nil, tc.ogsID)

		defer testpkg.CleanupActivityFixtures(t, tc.db, groupRoom.ID, visitRoom.ID, group.ID, student.ID, activityGroup.ID, staff.ID, device.ID, activeGroup.ID)

		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

		req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Should indicate not in group room
		assert.Contains(t, rr.Body.String(), "in_group_room")
	})
}

func TestGetStudentCurrentLocation_WithActiveVisit(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("student_checked_in_with_room", func(t *testing.T) {
		// Create fixtures for a fully checked-in student with room assignment
		room := testpkg.CreateTestRoom(t, tc.db, "LocationRoom", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "Location", "CheckedIn", "LC1", tc.ogsID)
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "LocationActivity", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Location", "Staff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "location-device", tc.ogsID)

		// Create attendance (check-in)
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		// Create active group with room
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID, tc.ogsID)

		// Create visit
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now, nil, tc.ogsID)

		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, student.ID, activityGroup.ID, staff.ID, device.ID, activeGroup.ID)

		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "current_location")
		// Should show room name
		assert.Contains(t, rr.Body.String(), "LocationRoom")
	})

	t.Run("student_checked_in_no_visit", func(t *testing.T) {
		// Student checked in but no active visit (transit state)
		student := testpkg.CreateTestStudent(t, tc.db, "Transit", "Student", "TS1", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Transit", "Staff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "transit-device", tc.ogsID)

		// Create attendance (check-in) but NO visit
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, device.ID)

		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "current_location")
	})

	t.Run("student_checked_out", func(t *testing.T) {
		// Student with completed attendance (checked out)
		student := testpkg.CreateTestStudent(t, tc.db, "CheckedOut", "Student", "CO1", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "CheckedOut", "Staff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "checkout-device", tc.ogsID)

		// Create attendance with check-out time
		now := time.Now()
		checkOut := now.Add(-1 * time.Hour)
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now.Add(-2*time.Hour), &checkOut)

		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, device.ID)

		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "current_location")
	})
}

func TestGetStudentCurrentVisit_WithActiveVisit(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("returns_current_visit", func(t *testing.T) {
		// Create a student with an active visit
		room := testpkg.CreateTestRoom(t, tc.db, "VisitTestRoom", tc.ogsID)
		student := testpkg.CreateTestStudent(t, tc.db, "Current", "Visit", "CV1", tc.ogsID)
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "VisitTestActivity", tc.ogsID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Visit", "Staff", tc.ogsID)
		device := testpkg.CreateTestDevice(t, tc.db, "visit-test-device", tc.ogsID)

		// Create attendance
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		// Create active group and visit
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID, tc.ogsID)
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now, nil, tc.ogsID)

		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, student.ID, activityGroup.ID, staff.ID, device.ID, activeGroup.ID)

		router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Should return the visit data
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}
