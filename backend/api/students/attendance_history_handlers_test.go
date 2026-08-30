package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enableAttendanceLog turns on the attendance log feature for tenant 1
// via a real DB setting override. Cleanup is deferred automatically.
func enableAttendanceLog(t *testing.T, tc *testContext) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	err := tc.services.Settings.SetValue(ctx, configModel.KeyAttendanceLogEnabled, true, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyAttendanceLogEnabled, nil, nil)
	})
}

func TestGetStudentAttendanceHistory_FeatureDisabled(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	student := testpkg.CreateTestStudent(t, tc.db, "DisabledFeat", "Student", "1a")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusForbidden, rr.Code, "should return 403 when feature is disabled. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "feature_disabled")
}

func TestGetStudentAttendanceHistory_FeatureEnabled_ReturnsData(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "EnabledFeat", "Student", "2b")
	staff := testpkg.CreateTestStaff(t, tc.db, "EnabledFeat", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "enabled-feat-dev")

	// Create today's attendance
	checkIn := timezone.Today().Add(8 * time.Hour)
	checkOut := timezone.Today().Add(15 * time.Hour)
	testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, checkIn, &checkOut)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data struct {
			StudentID string `json:"student_id"`
			Days      []struct {
				Date                string `json:"date"`
				RoomDetailAvailable bool   `json:"room_detail_available"`
				Attendance          *struct {
					CheckInTime     string `json:"check_in_time"`
					DurationMinutes *int   `json:"duration_minutes"`
				} `json:"attendance"`
			} `json:"days"`
			Clamped bool `json:"clamped"`
			Caps    struct {
				AttendanceDays int `json:"attendance_days"`
				RoomDetailDays int `json:"room_detail_days"`
			} `json:"caps"`
		} `json:"data"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%d", student.ID), body.Data.StudentID)
	require.GreaterOrEqual(t, len(body.Data.Days), 1)
	assert.NotNil(t, body.Data.Days[0].Attendance, "today's attendance should be present")
	assert.NotEmpty(t, body.Data.Days[0].Attendance.CheckInTime)
	assert.False(t, body.Data.Clamped, "default range should not be clamped")
	assert.Equal(t, 30, body.Data.Caps.AttendanceDays, "default attendance cap")
	assert.Equal(t, 7, body.Data.Caps.RoomDetailDays, "default room cap")
}

func TestGetStudentAttendanceHistory_InvalidStudentID(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	req := testutil.NewRequest("GET", "/invalid/attendance-history", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetStudentAttendanceHistory_StudentNotFound(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	req := testutil.NewRequest("GET", "/999999/attendance-history", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetStudentAttendanceHistory_WritesAuditLog(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Audit", "Student", "3a")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// Verify audit row was written
	var count int
	err := tc.db.NewSelect().
		TableExpr("audit.data_access_log").
		Where("student_id = ?", student.ID).
		Where("resource_type = ?", "attendance_history").
		ColumnExpr("count(*)").
		Scan(context.Background(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should have written one audit log entry")

	// Cleanup audit entries for this student
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().
			TableExpr("audit.data_access_log").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})
}

func TestGetStudentAttendanceHistory_RangeClampedWhenExceedingCap(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Clamped", "Student", "2c")

	// Request 90-day range (cap is 30 by default)
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history?start=2026-01-01T00:00:00Z&end=2026-04-06T23:59:59Z", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data struct {
			Clamped bool `json:"clamped"`
		} `json:"data"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.True(t, body.Data.Clamped, "response should be clamped when range exceeds cap")
}

func TestGetStudentAttendanceHistory_FutureEndClampsToToday(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t, fixedCalendarClock)
	enableAttendanceLog(t, tc)
	student := testpkg.CreateTestStudent(t, tc.db, "FutureEnd", "Student", "2c")

	today := timezone.NewDate(2026, 8, 24).BerlinMidnight()
	start := today.AddDate(0, 0, -1)
	end := today.AddDate(0, 0, 2)
	req := testutil.NewRequest("GET", fmt.Sprintf(
		"/%d/attendance-history?start=%s&end=%s",
		student.ID, start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339),
	), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data struct {
			Days []struct {
				Date string `json:"date"`
			} `json:"days"`
			Range struct {
				End time.Time `json:"end"`
			} `json:"range"`
			Clamped bool `json:"clamped"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.True(t, body.Data.Clamped)
	assert.False(t, body.Data.Range.End.After(timezone.NewDate(2026, 8, 24).EndOfDay()))
	for _, day := range body.Data.Days {
		assert.LessOrEqual(t, day.Date, timezone.NewDate(2026, 8, 24).String())
	}
}

func TestGetStudentAttendanceHistory_FullyFutureRangeRejected(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)
	student := testpkg.CreateTestStudent(t, tc.db, "FutureOnly", "Student", "2c")

	today := timezone.Today()
	start := today.AddDate(0, 0, 1)
	end := today.AddDate(0, 0, 2)
	req := testutil.NewRequest("GET", fmt.Sprintf(
		"/%d/attendance-history?start=%s&end=%s",
		student.ID, start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339),
	), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "start must not be in the future")
}

func TestGetStudentAttendanceHistory_StaffCanAccessAnyStudent(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	// #2329: the history is readable by any verified staff member of the tenant,
	// regardless of which group the child belongs to.
	student := testpkg.CreateTestStudent(t, tc.db, "ScopeAll", "Student", "3b")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "ScopeAll", "Staff")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "any staff member with users:read may read the history. Body: %s", rr.Body.String())
}

func TestGetStudentAttendanceHistory_UnlinkedAccountForbidden(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	// An account holding users:read without a staff record (guest, guardian) is
	// not staff and must stay out — users:read alone never unlocks the history.
	student := testpkg.CreateTestStudent(t, tc.db, "Unlinked", "Student", "3c")
	account := testpkg.CreateTestAccount(t, tc.db, "history-unlinked@example.com")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
}

func TestGetStudentAttendanceHistory_InvalidDateRange(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "DateRange", "Student", "1c")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history?start=2026-04-10T00:00:00Z&end=2026-04-01T00:00:00Z", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code, "start > end should be rejected")
}

func TestGetStudentAttendanceHistory_EmptyResult(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Empty", "Student", "4a")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "empty result is still 200")

	var body struct {
		Data struct {
			Days []any `json:"days"`
		} `json:"data"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Empty(t, body.Data.Days, "should return empty days array")
}
