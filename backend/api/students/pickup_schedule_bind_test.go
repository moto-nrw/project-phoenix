package students

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PickupScheduleRequest Bind Tests
// =============================================================================

func TestPickupScheduleRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "15:30",
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("valid request with notes", func(t *testing.T) {
		notes := "Parent pickup only"
		r := &PickupScheduleRequest{
			Weekday:    3,
			PickupTime: "14:00",
			Notes:      &notes,
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("weekday too low", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    0,
			PickupTime: "15:30",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "weekday must be between 1 (Monday) and 5 (Friday)")
	})

	t.Run("weekday too high", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    6,
			PickupTime: "15:30",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "weekday must be between 1 (Monday) and 5 (Friday)")
	})

	t.Run("missing pickup_time", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pickup_time is required")
	})

	t.Run("invalid pickup_time format", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "invalid",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pickup_time format")
	})

	t.Run("notes too long", func(t *testing.T) {
		longNotes := string(make([]byte, 501))
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "15:30",
			Notes:      &longNotes,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "notes cannot exceed 500 characters")
	})
}

// =============================================================================
// BulkPickupScheduleRequest Bind Tests
// =============================================================================

func TestBulkPickupScheduleRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 3, PickupTime: "14:00"},
			},
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("empty schedules", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedules array cannot be empty")
	})

	t.Run("invalid weekday in schedule", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 7, PickupTime: "14:00"},
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 1: weekday must be between")
	})

	t.Run("missing pickup_time in schedule", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: ""},
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 0: pickup_time is required")
	})

	t.Run("invalid pickup_time format in schedule", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "25:00"},
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 0: invalid pickup_time format")
	})

	t.Run("notes too long in schedule", func(t *testing.T) {
		longNotes := string(make([]byte, 501))
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30", Notes: &longNotes},
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 0: notes cannot exceed 500 characters")
	})
}

// =============================================================================
// PickupExceptionRequest Bind Tests
// =============================================================================

func TestPickupExceptionRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        "Doctor appointment",
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("missing exception_date", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "",
			PickupTime:    &pickupTime,
			Reason:        "Test reason",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exception_date is required")
	})

	t.Run("invalid exception_date format", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "15-02-2026",
			PickupTime:    &pickupTime,
			Reason:        "Test reason",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid exception_date format")
	})

	t.Run("nil pickup_time is valid (absent student)", func(t *testing.T) {
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    nil,
			Reason:        "Student is sick",
		}
		err := r.Bind(req)
		require.NoError(t, err, "nil pickup_time should be valid for absent students")
	})

	t.Run("empty pickup_time is valid (absent student)", func(t *testing.T) {
		emptyTime := ""
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &emptyTime,
			Reason:        "Student is sick",
		}
		err := r.Bind(req)
		require.NoError(t, err, "empty pickup_time should be valid for absent students")
	})

	t.Run("invalid pickup_time format", func(t *testing.T) {
		invalidTime := "invalid"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &invalidTime,
			Reason:        "Test reason",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pickup_time format")
	})

	t.Run("missing reason", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        "",
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason is required")
	})

	t.Run("reason too long", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        string(make([]byte, 256)),
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason cannot exceed 255 characters")
	})
}

// =============================================================================
// BulkPickupTimeRequest Bind Tests
// =============================================================================

func TestBulkPickupTimeRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1, 2, 3},
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("valid request with date", func(t *testing.T) {
		date := "2026-01-27"
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &date,
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("empty student_ids", func(t *testing.T) {
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_ids array cannot be empty")
	})

	t.Run("too many student_ids", func(t *testing.T) {
		ids := make([]int64, 501)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		r := &BulkPickupTimeRequest{
			StudentIDs: ids,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_ids array cannot exceed 500 items")
	})

	t.Run("invalid date format", func(t *testing.T) {
		invalidDate := "27-01-2026"
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &invalidDate,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid date format")
	})

	t.Run("empty date is valid", func(t *testing.T) {
		emptyDate := ""
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &emptyDate,
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestParseTimeOnly(t *testing.T) {
	t.Run("valid time", func(t *testing.T) {
		result, err := parseTimeOnly("15:30")
		require.NoError(t, err)
		assert.Equal(t, 15, result.Hour())
		assert.Equal(t, 30, result.Minute())
	})

	t.Run("midnight", func(t *testing.T) {
		result, err := parseTimeOnly("00:00")
		require.NoError(t, err)
		assert.Equal(t, 0, result.Hour())
		assert.Equal(t, 0, result.Minute())
	})

	t.Run("invalid time", func(t *testing.T) {
		_, err := parseTimeOnly("invalid")
		require.Error(t, err)
	})
}

// =============================================================================
// Response Mapping Tests
// =============================================================================

func TestMapScheduleToResponse(t *testing.T) {
	t.Run("maps schedule without notes", func(t *testing.T) {
		schedule := createTestScheduleModel(1, 1, "15:30", nil)
		resp := mapScheduleToResponse(schedule)

		assert.Equal(t, schedule.ID, resp.ID)
		assert.Equal(t, schedule.StudentID, resp.StudentID)
		assert.Equal(t, 1, resp.Weekday)
		assert.Equal(t, "Montag", resp.WeekdayName)
		assert.Equal(t, "15:30", resp.PickupTime)
		assert.Nil(t, resp.Notes)
	})

	t.Run("maps schedule with notes", func(t *testing.T) {
		notes := "Parent pickup only"
		schedule := createTestScheduleModel(1, 3, "14:00", &notes)
		resp := mapScheduleToResponse(schedule)

		assert.Equal(t, "14:00", resp.PickupTime)
		assert.NotNil(t, resp.Notes)
		assert.Equal(t, notes, *resp.Notes)
	})
}

func TestMapExceptionToResponse(t *testing.T) {
	t.Run("maps exception with pickup time", func(t *testing.T) {
		exception := createTestExceptionModel(1, "2026-02-15", "12:00", "Doctor appointment")
		resp := mapExceptionToResponse(exception)

		assert.Equal(t, exception.ID, resp.ID)
		assert.Equal(t, exception.StudentID, resp.StudentID)
		assert.Equal(t, "2026-02-15", resp.ExceptionDate)
		assert.NotNil(t, resp.PickupTime)
		assert.Equal(t, "12:00", *resp.PickupTime)
		assert.Equal(t, "Doctor appointment", resp.Reason)
	})

	t.Run("maps exception without pickup time (absent)", func(t *testing.T) {
		exception := createTestExceptionModelAbsent(1, "2026-02-15", "Student is sick")
		resp := mapExceptionToResponse(exception)

		assert.Equal(t, "2026-02-15", resp.ExceptionDate)
		assert.Nil(t, resp.PickupTime)
		assert.Equal(t, "Student is sick", resp.Reason)
	})
}

// =============================================================================
// Test Helpers
// =============================================================================

func createTestScheduleModel(studentID int64, weekday int, pickupTime string, notes *string) *schedule.StudentPickupSchedule {
	parsedTime, _ := parseTimeOnly(pickupTime)
	return &schedule.StudentPickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: parsedTime,
		Notes:      notes,
		CreatedBy:  1,
	}
}

func createTestExceptionModel(studentID int64, date, pickupTime, reason string) *schedule.StudentPickupException {
	parsedDate, _ := time.Parse("2006-01-02", date)
	parsedTime, _ := parseTimeOnly(pickupTime)
	return &schedule.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: parsedDate,
		PickupTime:    &parsedTime,
		Reason:        reason,
		CreatedBy:     1,
	}
}

func createTestExceptionModelAbsent(studentID int64, date, reason string) *schedule.StudentPickupException {
	parsedDate, _ := time.Parse("2006-01-02", date)
	return &schedule.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: parsedDate,
		PickupTime:    nil,
		Reason:        reason,
		CreatedBy:     1,
	}
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

func TestPickupScheduleRequest_Bind_EdgeCases(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("weekday_boundary_monday", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "08:00",
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("weekday_boundary_friday", func(t *testing.T) {
		r := &PickupScheduleRequest{
			Weekday:    5,
			PickupTime: "17:00",
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("time_format_variations", func(t *testing.T) {
		testCases := []struct {
			time    string
			isValid bool
		}{
			{"00:00", true},
			{"23:59", true},
			{"12:30", true},
			{"9:30", true},      // Go time.Parse accepts single-digit hours
			{"12:5", false},     // Missing leading zero
			{"24:00", false},    // Invalid hour
			{"12:60", false},    // Invalid minute
			{"12:30:00", false}, // Seconds not allowed
		}
		for _, tc := range testCases {
			r := &PickupScheduleRequest{
				Weekday:    1,
				PickupTime: tc.time,
			}
			err := r.Bind(req)
			if tc.isValid {
				assert.NoError(t, err, "Time %s should be valid", tc.time)
			} else {
				assert.Error(t, err, "Time %s should be invalid", tc.time)
			}
		}
	})

	t.Run("notes_boundary_500_chars", func(t *testing.T) {
		exactNotes := string(make([]byte, 500))
		r := &PickupScheduleRequest{
			Weekday:    1,
			PickupTime: "15:30",
			Notes:      &exactNotes,
		}
		err := r.Bind(req)
		require.NoError(t, err, "500 characters should be allowed")
	})
}

func TestBulkPickupScheduleRequest_Bind_EdgeCases(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("all_weekdays", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 2, PickupTime: "15:30"},
				{Weekday: 3, PickupTime: "15:30"},
				{Weekday: 4, PickupTime: "15:30"},
				{Weekday: 5, PickupTime: "15:30"},
			},
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("single_schedule", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 3, PickupTime: "12:00"},
			},
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("error_index_reporting", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 2, PickupTime: "15:30"},
				{Weekday: 6, PickupTime: "15:30"}, // Invalid at index 2
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 2:")
	})
}

func TestPickupExceptionRequest_Bind_EdgeCases(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("reason_boundary_255_chars", func(t *testing.T) {
		pickupTime := "12:00"
		exactReason := string(make([]byte, 255))
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        exactReason,
		}
		err := r.Bind(req)
		require.NoError(t, err, "255 characters should be allowed")
	})

	t.Run("date_format_variations", func(t *testing.T) {
		pickupTime := "12:00"
		invalidDates := []string{
			"2026/02/15", // Wrong separator
			"02-15-2026", // MM-DD-YYYY
			"15-02-2026", // DD-MM-YYYY
			"2026-2-15",  // Missing leading zero
			"2026-02-5",  // Missing leading zero
		}
		for _, date := range invalidDates {
			r := &PickupExceptionRequest{
				ExceptionDate: date,
				PickupTime:    &pickupTime,
				Reason:        "Test",
			}
			err := r.Bind(req)
			assert.Error(t, err, "Date %s should be invalid", date)
		}
	})
}

func TestMapScheduleToResponse_WeekdayNames(t *testing.T) {
	weekdays := []struct {
		day  int
		name string
	}{
		{1, "Montag"},
		{2, "Dienstag"},
		{3, "Mittwoch"},
		{4, "Donnerstag"},
		{5, "Freitag"},
	}

	for _, wd := range weekdays {
		t.Run(wd.name, func(t *testing.T) {
			schedule := createTestScheduleModel(1, wd.day, "15:30", nil)
			resp := mapScheduleToResponse(schedule)
			assert.Equal(t, wd.name, resp.WeekdayName)
		})
	}
}

func TestBulkPickupTimeRequest_Bind_EdgeCases(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("exactly_500_student_ids", func(t *testing.T) {
		ids := make([]int64, 500)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		r := &BulkPickupTimeRequest{
			StudentIDs: ids,
		}
		err := r.Bind(req)
		require.NoError(t, err, "exactly 500 student IDs should be allowed")
	})

	t.Run("single_student_id", func(t *testing.T) {
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1},
		}
		err := r.Bind(req)
		require.NoError(t, err, "single student ID should be allowed")
	})

	t.Run("nil_date_is_valid", func(t *testing.T) {
		r := &BulkPickupTimeRequest{
			StudentIDs: []int64{1, 2},
			Date:       nil,
		}
		err := r.Bind(req)
		require.NoError(t, err, "nil date should be valid")
	})
}

func TestMapScheduleToResponse_ResponseFormat(t *testing.T) {
	t.Run("formats created_at and updated_at as RFC3339", func(t *testing.T) {
		now := time.Now()
		studentID := int64(12345)
		createdBy := int64(67890)
		schedID := int64(42)
		sched := &schedule.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    1,
			PickupTime: time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC),
			CreatedBy:  createdBy,
		}
		sched.ID = schedID
		sched.CreatedAt = now
		sched.UpdatedAt = now

		resp := mapScheduleToResponse(sched)

		assert.Equal(t, schedID, resp.ID)
		assert.Equal(t, studentID, resp.StudentID)
		assert.Equal(t, "15:30", resp.PickupTime)
		assert.Equal(t, createdBy, resp.CreatedBy)
		assert.Equal(t, now.Format(time.RFC3339), resp.CreatedAt)
		assert.Equal(t, now.Format(time.RFC3339), resp.UpdatedAt)
	})

	t.Run("handles invalid weekday gracefully", func(t *testing.T) {
		studentID := int64(12345)
		createdBy := int64(67890)
		sched := &schedule.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    99, // Invalid weekday
			PickupTime: time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC),
			CreatedBy:  createdBy,
		}
		resp := mapScheduleToResponse(sched)

		assert.Equal(t, 99, resp.Weekday)
		// GetWeekdayName returns empty string for invalid weekdays
		assert.Equal(t, "", resp.WeekdayName)
	})
}

func TestMapExceptionToResponse_ResponseFormat(t *testing.T) {
	t.Run("formats timestamps as RFC3339", func(t *testing.T) {
		now := time.Now()
		studentID := int64(12345)
		createdBy := int64(67890)
		excID := int64(42)
		pickupTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		exc := &schedule.StudentPickupException{
			StudentID:     studentID,
			ExceptionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			PickupTime:    &pickupTime,
			Reason:        "Test",
			CreatedBy:     createdBy,
		}
		exc.ID = excID
		exc.CreatedAt = now
		exc.UpdatedAt = now

		resp := mapExceptionToResponse(exc)

		assert.Equal(t, excID, resp.ID)
		assert.Equal(t, studentID, resp.StudentID)
		assert.Equal(t, "2026-02-15", resp.ExceptionDate)
		assert.Equal(t, createdBy, resp.CreatedBy)
		assert.Equal(t, now.Format(time.RFC3339), resp.CreatedAt)
		assert.Equal(t, now.Format(time.RFC3339), resp.UpdatedAt)
	})
}

func TestBulkPickupScheduleRequest_Bind_AllValidationPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("schedule with notes too long", func(t *testing.T) {
		longNotes := string(make([]byte, 501))
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 2, PickupTime: "16:00", Notes: &longNotes},
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 1: notes cannot exceed 500 characters")
	})

	t.Run("reports correct error index", func(t *testing.T) {
		r := &BulkPickupScheduleRequest{
			Schedules: []PickupScheduleRequest{
				{Weekday: 1, PickupTime: "15:30"},
				{Weekday: 2, PickupTime: "16:00"},
				{Weekday: 0, PickupTime: "14:00"}, // Invalid weekday at index 2
			},
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule 2:")
	})
}

// =============================================================================
// Handler Accessor Method Tests
// =============================================================================

func TestHandlerAccessorMethods(t *testing.T) {
	// Create a minimal Resource for testing accessor methods
	rs := &Resource{}

	t.Run("GetStudentPickupSchedulesHandler returns handler", func(t *testing.T) {
		handler := rs.GetStudentPickupSchedulesHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("UpdateStudentPickupSchedulesHandler returns handler", func(t *testing.T) {
		handler := rs.UpdateStudentPickupSchedulesHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("CreateStudentPickupExceptionHandler returns handler", func(t *testing.T) {
		handler := rs.CreateStudentPickupExceptionHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("UpdateStudentPickupExceptionHandler returns handler", func(t *testing.T) {
		handler := rs.UpdateStudentPickupExceptionHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("DeleteStudentPickupExceptionHandler returns handler", func(t *testing.T) {
		handler := rs.DeleteStudentPickupExceptionHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("GetBulkPickupTimesHandler returns handler", func(t *testing.T) {
		handler := rs.GetBulkPickupTimesHandler()
		assert.NotNil(t, handler, "Handler should not be nil")
	})
}

// =============================================================================
// Parse Time Only Tests
// =============================================================================

func TestParseTimeOnly_Variations(t *testing.T) {
	t.Run("parses morning time", func(t *testing.T) {
		result, err := parseTimeOnly("08:30")
		require.NoError(t, err)
		assert.Equal(t, 8, result.Hour())
		assert.Equal(t, 30, result.Minute())
	})

	t.Run("parses afternoon time", func(t *testing.T) {
		result, err := parseTimeOnly("16:45")
		require.NoError(t, err)
		assert.Equal(t, 16, result.Hour())
		assert.Equal(t, 45, result.Minute())
	})

	t.Run("parses end of day time", func(t *testing.T) {
		result, err := parseTimeOnly("23:59")
		require.NoError(t, err)
		assert.Equal(t, 23, result.Hour())
		assert.Equal(t, 59, result.Minute())
	})

	t.Run("returns error for out-of-range hour", func(t *testing.T) {
		_, err := parseTimeOnly("25:00")
		require.Error(t, err)
	})

	t.Run("returns error for out-of-range minute", func(t *testing.T) {
		_, err := parseTimeOnly("12:60")
		require.Error(t, err)
	})

	t.Run("returns error for malformed time", func(t *testing.T) {
		_, err := parseTimeOnly("12-30")
		require.Error(t, err)
	})

	t.Run("returns error for empty string", func(t *testing.T) {
		_, err := parseTimeOnly("")
		require.Error(t, err)
	})
}

// =============================================================================
// PickupDataResponse Type Tests
// =============================================================================

func TestPickupDataResponse_Structure(t *testing.T) {
	t.Run("empty response has empty slices", func(t *testing.T) {
		resp := PickupDataResponse{
			Schedules:  []PickupScheduleResponse{},
			Exceptions: []PickupExceptionResponse{},
		}
		assert.Len(t, resp.Schedules, 0)
		assert.Len(t, resp.Exceptions, 0)
	})

	t.Run("response with data", func(t *testing.T) {
		schedResp := PickupScheduleResponse{
			ID:          1,
			StudentID:   100,
			Weekday:     1,
			WeekdayName: "Montag",
			PickupTime:  "15:30",
			CreatedBy:   5,
			CreatedAt:   "2026-01-27T10:00:00Z",
			UpdatedAt:   "2026-01-27T10:00:00Z",
		}
		excResp := PickupExceptionResponse{
			ID:            1,
			StudentID:     100,
			ExceptionDate: "2026-01-29",
			Reason:        "Doctor",
			CreatedBy:     5,
			CreatedAt:     "2026-01-27T10:00:00Z",
			UpdatedAt:     "2026-01-27T10:00:00Z",
		}
		resp := PickupDataResponse{
			Schedules:  []PickupScheduleResponse{schedResp},
			Exceptions: []PickupExceptionResponse{excResp},
		}
		assert.Len(t, resp.Schedules, 1)
		assert.Len(t, resp.Exceptions, 1)
		assert.Equal(t, int64(1), resp.Schedules[0].ID)
		assert.Equal(t, int64(1), resp.Exceptions[0].ID)
	})
}

// =============================================================================
// BulkPickupTimeResponse Type Tests
// =============================================================================

func TestBulkPickupTimeResponse_Structure(t *testing.T) {
	t.Run("response with pickup time", func(t *testing.T) {
		pickupTime := "14:30"
		resp := BulkPickupTimeResponse{
			StudentID:   100,
			Date:        "2026-01-27",
			WeekdayName: "Montag",
			PickupTime:  &pickupTime,
			IsException: false,
			Notes:       "Test notes",
		}
		assert.Equal(t, int64(100), resp.StudentID)
		assert.Equal(t, "2026-01-27", resp.Date)
		assert.NotNil(t, resp.PickupTime)
		assert.Equal(t, "14:30", *resp.PickupTime)
		assert.False(t, resp.IsException)
	})

	t.Run("response without pickup time (absent)", func(t *testing.T) {
		resp := BulkPickupTimeResponse{
			StudentID:   100,
			Date:        "2026-01-27",
			WeekdayName: "Montag",
			PickupTime:  nil,
			IsException: true,
			Reason:      "Student is sick",
		}
		assert.Equal(t, int64(100), resp.StudentID)
		assert.Nil(t, resp.PickupTime)
		assert.True(t, resp.IsException)
		assert.Equal(t, "Student is sick", resp.Reason)
	})
}
