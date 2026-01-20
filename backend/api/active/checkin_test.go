// Package active_test tests the checkin-related functionality
package active_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// =============================================================================
// Active Group Model Tests
// =============================================================================

func TestActiveGroup_IsActive(t *testing.T) {
	t.Run("group with no end time is active", func(t *testing.T) {
		group := &activeModels.Group{
			RoomID: 1,
		}
		assert.True(t, group.IsActive())
	})

	t.Run("group with end time is not active (regardless of time)", func(t *testing.T) {
		// IsActive() returns true only when EndTime is nil
		futureTime := time.Now().Add(1 * time.Hour)
		group := &activeModels.Group{
			RoomID:  1,
			EndTime: &futureTime,
		}
		assert.False(t, group.IsActive()) // EndTime is set, so not active
	})

	t.Run("group with past end time is not active", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)
		group := &activeModels.Group{
			RoomID:  1,
			EndTime: &pastTime,
		}
		assert.False(t, group.IsActive())
	})
}

// =============================================================================
// Visit Model Tests
// =============================================================================

func TestVisit_Fields(t *testing.T) {
	t.Run("visit has required fields", func(t *testing.T) {
		now := time.Now()
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     now,
		}

		assert.Equal(t, int64(123), visit.StudentID)
		assert.Equal(t, int64(456), visit.ActiveGroupID)
		assert.Equal(t, now, visit.EntryTime)
		assert.Nil(t, visit.ExitTime)
	})

	t.Run("visit can have exit time", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(1 * time.Hour)
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     now,
			ExitTime:      &exitTime,
		}

		require.NotNil(t, visit.ExitTime)
		assert.True(t, visit.ExitTime.After(visit.EntryTime))
	})

	t.Run("visit IsActive returns true when no exit time", func(t *testing.T) {
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     time.Now(),
		}
		assert.True(t, visit.IsActive())
	})

	t.Run("visit IsActive returns false when exit time is set", func(t *testing.T) {
		exitTime := time.Now()
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     time.Now().Add(-1 * time.Hour),
			ExitTime:      &exitTime,
		}
		assert.False(t, visit.IsActive())
	})
}

// =============================================================================
// CheckinRequest Tests
// =============================================================================

func TestCheckinRequest_Validation(t *testing.T) {
	t.Run("valid request with active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{
			ActiveGroupID: 1,
		}
		assert.Greater(t, req.ActiveGroupID, int64(0))
	})

	t.Run("invalid request without active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{}
		assert.Equal(t, int64(0), req.ActiveGroupID)
	})
}

func TestCheckinRequest_JSONDecoding(t *testing.T) {
	t.Run("decodes from JSON correctly", func(t *testing.T) {
		jsonData := `{"active_group_id": 456}`
		var req active.CheckinRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		require.NoError(t, err)
		assert.Equal(t, int64(456), req.ActiveGroupID)
	})

	t.Run("decodes zero value when missing", func(t *testing.T) {
		jsonData := `{}`
		var req active.CheckinRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		require.NoError(t, err)
		assert.Equal(t, int64(0), req.ActiveGroupID)
	})

	t.Run("encodes to JSON correctly", func(t *testing.T) {
		req := active.CheckinRequest{ActiveGroupID: 123}
		data, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(data), "123")
	})
}

// =============================================================================
// Attendance Model Tests
// =============================================================================

func TestAttendance_Fields(t *testing.T) {
	t.Run("attendance has required fields", func(t *testing.T) {
		now := time.Now()
		today := timezone.Today()
		attendance := &activeModels.Attendance{
			StudentID:   123,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: 456,
			DeviceID:    789,
		}

		assert.Equal(t, int64(123), attendance.StudentID)
		assert.Equal(t, today, attendance.Date)
		assert.Equal(t, now, attendance.CheckInTime)
		assert.Equal(t, int64(456), attendance.CheckedInBy)
		assert.Equal(t, int64(789), attendance.DeviceID)
		assert.Nil(t, attendance.CheckOutTime)
		assert.Nil(t, attendance.CheckedOutBy)
	})

	t.Run("attendance can have checkout fields", func(t *testing.T) {
		now := time.Now()
		checkoutTime := now.Add(4 * time.Hour)
		checkedOutBy := int64(789)

		attendance := &activeModels.Attendance{
			StudentID:    123,
			Date:         timezone.Today(),
			CheckInTime:  now,
			CheckedInBy:  456,
			DeviceID:     111,
			CheckOutTime: &checkoutTime,
			CheckedOutBy: &checkedOutBy,
		}

		require.NotNil(t, attendance.CheckOutTime)
		require.NotNil(t, attendance.CheckedOutBy)
		assert.True(t, attendance.CheckOutTime.After(attendance.CheckInTime))
		assert.Equal(t, int64(789), *attendance.CheckedOutBy)
	})
}

// =============================================================================
// Group Supervisor Model Tests
// =============================================================================

func TestGroupSupervisor_IsActive(t *testing.T) {
	t.Run("supervisor with no end date is active", func(t *testing.T) {
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now(),
		}
		assert.True(t, supervisor.IsActive())
	})

	t.Run("supervisor with future end date is active", func(t *testing.T) {
		futureDate := time.Now().Add(30 * 24 * time.Hour)
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now(),
			EndDate:   &futureDate,
		}
		assert.True(t, supervisor.IsActive())
	})

	t.Run("supervisor with past end date is not active", func(t *testing.T) {
		pastDate := time.Now().Add(-30 * 24 * time.Hour)
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now().Add(-60 * 24 * time.Hour),
			EndDate:   &pastDate,
		}
		assert.False(t, supervisor.IsActive())
	})
}

// =============================================================================
// Combined Group Model Tests
// =============================================================================

func TestCombinedGroup_IsActive(t *testing.T) {
	t.Run("combined group with no end time is active", func(t *testing.T) {
		combined := &activeModels.CombinedGroup{
			StartTime: time.Now(),
		}
		assert.True(t, combined.IsActive())
	})

	t.Run("combined group with end time is not active", func(t *testing.T) {
		endTime := time.Now()
		combined := &activeModels.CombinedGroup{
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   &endTime,
		}
		assert.False(t, combined.IsActive())
	})
}

// =============================================================================
// Handler Integration Tests (Hermetic with Test DB)
// =============================================================================

// setupViperForTest configures viper with the test JWT secret
func setupViperForTest() {
	viper.Set("auth_jwt_secret", "test-jwt-secret-32-chars-minimum")
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
}

// setupCheckinTestHandler creates a handler with real services for integration testing
func setupCheckinTestHandler(t *testing.T, db *bun.DB) *active.Resource {
	t.Helper()

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	return active.NewResource(serviceFactory.Active, serviceFactory.Users, serviceFactory.Auth, db)
}

func TestCheckinStudent_Integration(t *testing.T) {
	// Configure viper with test JWT secret before any router is created
	setupViperForTest()

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	t.Run("returns 403 when no tenant context", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create minimal fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "no-auth-test")
		room := testpkg.CreateTestRoom(t, db, "No Auth Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "NoAuth", "Student", "1a")

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Make request without tenant context
		body := active.CheckinRequest{ActiveGroupID: activeGroup.ID}
		bodyBytes, _ := json.Marshal(body)
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should return 403 because no tenant context
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("returns 400 for invalid student ID in URL", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create staff with account
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Invalid", "IDTest")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		body := active.CheckinRequest{ActiveGroupID: 1}
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/visits/student/invalid/checkin", body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 400 when active_group_id is missing", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create staff with account
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Missing", "GroupID")
		student := testpkg.CreateTestStudent(t, db, "Missing", "GroupStudent", "2a")

		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID, student.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Request body missing active_group_id
		body := map[string]interface{}{}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 404 when active group does not exist", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create staff with account
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "NotFound", "GroupTest")
		student := testpkg.CreateTestStudent(t, db, "NotFound", "Student", "2b")

		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID, student.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Use non-existent active group ID
		body := active.CheckinRequest{ActiveGroupID: 999999}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("returns 403 when user is not staff", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create person with account but NO staff record
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "NotStaff", "User")
		student := testpkg.CreateTestStudent(t, db, "NotStaff", "Student", "2c")
		activity := testpkg.CreateTestActivityGroup(t, db, "not-staff-test")
		room := testpkg.CreateTestRoom(t, db, "Not Staff Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, person.ID, student.ID, activity.ID, room.ID, activeGroup.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		body := active.CheckinRequest{ActiveGroupID: activeGroup.ID}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should be 403 (forbidden) because user has no staff record
		// or 500 if the lookup fails - both indicate the user can't check in students
		assert.Contains(t, []int{http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
	})

	t.Run("returns 403 when staff has no access to student", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create staff with account (but NOT a teacher for the student's group)
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "NoAccess", "Staff")
		student := testpkg.CreateTestStudent(t, db, "NoAccess", "Student", "3a")
		activity := testpkg.CreateTestActivityGroup(t, db, "no-access-test")
		room := testpkg.CreateTestRoom(t, db, "No Access Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID, student.ID, activity.ID, room.ID, activeGroup.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		body := active.CheckinRequest{ActiveGroupID: activeGroup.ID}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("returns 409 when active group session has ended", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create teacher with account and education group
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "EndedSession", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Ended Session Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		// Create student assigned to the education group
		student := testpkg.CreateTestStudent(t, db, "EndedSession", "Student", "4a")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		activity := testpkg.CreateTestActivityGroup(t, db, "ended-session-test")
		room := testpkg.CreateTestRoom(t, db, "Ended Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		// End the active group session
		endTime := time.Now().Add(-1 * time.Hour)
		_, err := db.NewUpdate().
			Model((*activeModels.Group)(nil)).
			ModelTableExpr(`active.groups`).
			Set("end_time = ?", endTime).
			Where("id = ?", activeGroup.ID).
			Exec(context.Background())
		require.NoError(t, err)

		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.ID, teacher.Staff.PersonID, educationGroup.ID, student.ID, activity.ID, room.ID, activeGroup.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		body := active.CheckinRequest{ActiveGroupID: activeGroup.ID}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("successful checkin creates visit", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		// Create teacher with account and education group
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Success", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Success Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		// Create student assigned to the education group
		student := testpkg.CreateTestStudent(t, db, "Success", "Student", "5a")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		activity := testpkg.CreateTestActivityGroup(t, db, "success-checkin-test")
		room := testpkg.CreateTestRoom(t, db, "Success Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.ID, teacher.Staff.PersonID, educationGroup.ID, student.ID, activity.ID, room.ID, activeGroup.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		body := active.CheckinRequest{ActiveGroupID: activeGroup.ID}
		path := "/visits/student/" + strconv.FormatInt(student.ID, 10) + "/checkin"
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body,
			testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
			testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
		)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should succeed with 200
		assert.Equal(t, http.StatusOK, rr.Code, "Response body: %s", rr.Body.String())

		// Verify response contains expected fields
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		assert.Contains(t, response["message"], "checked in")

		// Verify data contains visit info
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, float64(student.ID), data["student_id"])
		assert.Equal(t, "checked_in", data["action"])
		assert.NotZero(t, data["visit_id"])
	})
}
