// Package active_test tests the HTTP handlers for the active API
package active_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Hermetic Integration Tests with Real Database
// =============================================================================

func TestCheckinStudent_Integration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	// Setup services
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	// Create resource with real services
	resource := active.NewResource(serviceFactory.Active, serviceFactory.Users, nil)

	t.Run("returns error for invalid student ID format", func(t *testing.T) {
		// Get the router from resource
		r := resource.Router()

		// Request with invalid student ID (non-numeric)
		reqBody := `{"active_group_id": 1}`
		req := httptest.NewRequest("POST", "/students/invalid/checkin", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		// Add JWT context (required for auth)
		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error when active_group_id is missing", func(t *testing.T) {
		// Create test fixtures
		student := testpkg.CreateTestStudent(t, db, "Checkin", "Test", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		r := resource.Router()

		// Request without active_group_id
		reqBody := `{}`
		req := httptest.NewRequest("POST", "/students/"+strconv.FormatInt(student.ID, 10)+"/checkin", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for non-existent active group", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Checkin", "NoGroup", "2a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		r := resource.Router()

		// Request with non-existent active group ID
		reqBody := `{"active_group_id": 999999}`
		req := httptest.NewRequest("POST", "/students/"+strconv.FormatInt(student.ID, 10)+"/checkin", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should return not found or bad request
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest)
	})
}

func TestGetStudentAttendanceStatus_Integration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err)

	resource := active.NewResource(serviceFactory.Active, serviceFactory.Users, nil)

	t.Run("returns not_checked_in for student without attendance", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Status", "Test", "3a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		r := resource.Router()

		req := httptest.NewRequest("GET", "/students/"+strconv.FormatInt(student.ID, 10)+"/attendance-status", nil)

		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].(map[string]interface{})
		if ok {
			assert.Equal(t, "not_checked_in", data["status"])
		}
	})

	t.Run("returns error for invalid student ID", func(t *testing.T) {
		r := resource.Router()

		req := httptest.NewRequest("GET", "/students/invalid/attendance-status", nil)

		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

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
}

// =============================================================================
// Attendance Model Tests
// =============================================================================

func TestAttendance_Fields(t *testing.T) {
	t.Run("attendance has required fields", func(t *testing.T) {
		now := time.Now()
		today := time.Now().UTC().Truncate(24 * time.Hour)
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
			Date:         time.Now().UTC().Truncate(24 * time.Hour),
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

