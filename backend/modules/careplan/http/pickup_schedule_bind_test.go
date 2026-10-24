package careplanhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickupScheduleRequest_Bind(t *testing.T) {
	t.Parallel()

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

func TestBulkPickupScheduleRequest_Bind(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)
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

func TestBulkArrivalScheduleRequest_BindRejectsLongNotesWithoutTime(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	longNotes := string(make([]byte, 501))
	arrival := &BulkArrivalScheduleRequest{
		Schedules: []ArrivalScheduleRequestItem{{
			Weekday:         1,
			ExpectedArrival: "",
			Notes:           &longNotes,
		}},
	}

	err := arrival.Bind(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notes cannot exceed 500 characters")
}

func TestPickupExceptionRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        testpkg.StrPtr("Doctor appointment"),
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("missing exception_date", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "",
			PickupTime:    &pickupTime,
			Reason:        testpkg.StrPtr("Test reason"),
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
			Reason:        testpkg.StrPtr("Test reason"),
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid exception_date format")
	})

	t.Run("nil pickup_time is valid (absent student)", func(t *testing.T) {
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    nil,
			Reason:        testpkg.StrPtr("Student is sick"),
		}
		err := r.Bind(req)
		require.NoError(t, err, "nil pickup_time should be valid for absent students")
	})

	t.Run("empty pickup_time is valid (absent student)", func(t *testing.T) {
		emptyTime := ""
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &emptyTime,
			Reason:        testpkg.StrPtr("Student is sick"),
		}
		err := r.Bind(req)
		require.NoError(t, err, "empty pickup_time should be valid for absent students")
	})

	t.Run("invalid pickup_time format", func(t *testing.T) {
		invalidTime := "invalid"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &invalidTime,
			Reason:        testpkg.StrPtr("Test reason"),
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pickup_time format")
	})

	t.Run("empty reason is valid (reason is optional)", func(t *testing.T) {
		pickupTime := "12:00"
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        nil,
		}
		err := r.Bind(req)
		require.NoError(t, err, "empty reason should be valid as reason is now optional")
	})

	t.Run("reason too long", func(t *testing.T) {
		pickupTime := "12:00"
		longReason := string(make([]byte, 256))
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        &longReason,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason cannot exceed 255 characters")
	})
}

func TestBulkPickupTimeRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("valid request", func(t *testing.T) {
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1, 2, 3},
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("valid request with date", func(t *testing.T) {
		date := "2026-01-27"
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &date,
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})

	t.Run("empty student_ids", func(t *testing.T) {
		r := &BulkEffectiveTimeRequest{
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
		r := &BulkEffectiveTimeRequest{
			StudentIDs: ids,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_ids array cannot exceed 500 items")
	})

	t.Run("invalid date format", func(t *testing.T) {
		invalidDate := "27-01-2026"
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &invalidDate,
		}
		err := r.Bind(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid date format")
	})

	t.Run("empty date is valid", func(t *testing.T) {
		emptyDate := ""
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1, 2, 3},
			Date:       &emptyDate,
		}
		err := r.Bind(req)
		require.NoError(t, err)
	})
}

func TestParseTimeOnly(t *testing.T) {
	t.Parallel()

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

	t.Run("end of day", func(t *testing.T) {
		result, err := parseTimeOnly("23:59")
		require.NoError(t, err)
		assert.Equal(t, 23, result.Hour())
		assert.Equal(t, 59, result.Minute())
	})

	for _, invalid := range []string{"invalid", "25:00", "12:60", "12-30", ""} {
		t.Run("rejects "+invalid, func(t *testing.T) {
			_, err := parseTimeOnly(invalid)
			require.Error(t, err)
		})
	}
}

func TestMapScheduleToResponse(t *testing.T) {
	t.Parallel()

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

	t.Run("maps projected offering provenance", func(t *testing.T) {
		offeringID := int64(42)
		row := createTestScheduleModel(1, 1, "14:30", nil)
		row.Source = careplan.ScheduleSourceCareOffering
		row.CareOfferingID = &offeringID
		row.CareOfferingName = "Ganztag bis 14:30 Uhr"

		resp := mapScheduleToResponse(row)

		require.NotNil(t, resp.CareOfferingID)
		assert.Equal(t, "42", *resp.CareOfferingID)
		assert.Equal(t, "Ganztag bis 14:30 Uhr", resp.CareOfferingName)
	})
}

func TestMapExceptionToResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps exception with pickup time", func(t *testing.T) {
		exception := createTestExceptionModel(1, "2026-02-15", "12:00", "Doctor appointment")
		resp := mapExceptionToResponse(exception)

		assert.Equal(t, exception.ID, resp.ID)
		assert.Equal(t, exception.StudentID, resp.StudentID)
		assert.Equal(t, "2026-02-15", resp.ExceptionDate)
		assert.NotNil(t, resp.PickupTime)
		assert.Equal(t, "12:00", *resp.PickupTime)
		require.NotNil(t, resp.Reason)
		assert.Equal(t, "Doctor appointment", *resp.Reason)
	})

	t.Run("maps exception without pickup time (absent)", func(t *testing.T) {
		exception := createTestExceptionModelAbsent(1, "2026-02-15", "Student is sick")
		resp := mapExceptionToResponse(exception)

		assert.Equal(t, "2026-02-15", resp.ExceptionDate)
		assert.Nil(t, resp.PickupTime)
		require.NotNil(t, resp.Reason)
		assert.Equal(t, "Student is sick", *resp.Reason)
	})
}

func createTestScheduleModel(studentID int64, weekday int, pickupTime string, notes *string) *careplan.PickupSchedule {
	parsedTime, _ := parseTimeOnly(pickupTime)
	return &careplan.PickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: parsedTime,
		Notes:      notes,
		CreatedBy:  1,
	}
}

func createTestExceptionModel(studentID int64, date, pickupTime, reason string) *careplan.PickupException {
	parsedDate, _ := calendar.ParseDate(date)
	parsedTime, _ := parseTimeOnly(pickupTime)
	return &careplan.PickupException{
		StudentID:     studentID,
		ExceptionDate: careplan.Date(parsedDate),
		PickupTime:    &parsedTime,
		Reason:        &reason,
		CreatedBy:     1,
	}
}

func createTestExceptionModelAbsent(studentID int64, date, reason string) *careplan.PickupException {
	parsedDate, _ := calendar.ParseDate(date)
	return &careplan.PickupException{
		StudentID:     studentID,
		ExceptionDate: careplan.Date(parsedDate),
		PickupTime:    nil,
		Reason:        &reason,
		CreatedBy:     1,
	}
}

func TestPickupScheduleRequest_Bind_EdgeCases(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("reason_boundary_255_chars", func(t *testing.T) {
		pickupTime := "12:00"
		exactReason := string(make([]byte, 255))
		r := &PickupExceptionRequest{
			ExceptionDate: "2026-02-15",
			PickupTime:    &pickupTime,
			Reason:        &exactReason,
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
				Reason:        testpkg.StrPtr("Test"),
			}
			err := r.Bind(req)
			assert.Error(t, err, "Date %s should be invalid", date)
		}
	})
}

func TestMapScheduleToResponse_WeekdayNames(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)

	t.Run("exactly_500_student_ids", func(t *testing.T) {
		ids := make([]int64, 500)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		r := &BulkEffectiveTimeRequest{
			StudentIDs: ids,
		}
		err := r.Bind(req)
		require.NoError(t, err, "exactly 500 student IDs should be allowed")
	})

	t.Run("single_student_id", func(t *testing.T) {
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1},
		}
		err := r.Bind(req)
		require.NoError(t, err, "single student ID should be allowed")
	})

	t.Run("nil_date_is_valid", func(t *testing.T) {
		r := &BulkEffectiveTimeRequest{
			StudentIDs: []int64{1, 2},
			Date:       nil,
		}
		err := r.Bind(req)
		require.NoError(t, err, "nil date should be valid")
	})
}

func TestMapScheduleToResponse_ResponseFormat(t *testing.T) {
	t.Parallel()

	t.Run("formats created_at and updated_at as RFC3339", func(t *testing.T) {
		now := time.Now()
		studentID := int64(12345)
		createdBy := int64(67890)
		schedID := int64(42)
		sched := &careplan.PickupSchedule{
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
		sched := &careplan.PickupSchedule{
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
	t.Parallel()

	t.Run("formats timestamps as RFC3339", func(t *testing.T) {
		now := time.Now()
		studentID := int64(12345)
		createdBy := int64(67890)
		excID := int64(42)
		pickupTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		reason := "Test"
		exc := &careplan.PickupException{
			StudentID:     studentID,
			ExceptionDate: careplan.Date(calendar.NewDate(2026, 2, 15)),
			PickupTime:    &pickupTime,
			Reason:        &reason,
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

func TestPickupScheduleDateRange(t *testing.T) {
	t.Parallel()

	t.Run("accepts inclusive week", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?from=2026-08-17&to=2026-08-21", nil)
		from, to, ok, err := pickupScheduleDateRange(r)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "2026-08-17", from.String())
		assert.Equal(t, "2026-08-21", to.String())
	})

	t.Run("requires both bounds", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?from=2026-08-17", nil)
		_, _, _, err := pickupScheduleDateRange(r)
		require.Error(t, err)
	})

	t.Run("rejects oversized range", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?from=2026-08-01&to=2026-09-02", nil)
		_, _, _, err := pickupScheduleDateRange(r)
		require.Error(t, err)
	})
}

func TestBuildPickupDataResponse_PreservesExplicitEmptyDate(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 8, 18)
	response := buildPickupDataResponse(&careplan.StudentPickupData{
		EffectiveSchedules: []careplan.DatedPickupSchedule{{Date: date}},
	})

	require.Len(t, response.EffectiveSchedules, 1)
	assert.Equal(t, "2026-08-18", response.EffectiveSchedules[0].Date)
	assert.Nil(t, response.EffectiveSchedules[0].Schedule)
}

func TestResetOfferingPickupRequest_Bind(t *testing.T) {
	t.Parallel()

	t.Run("accepts matching date and weekday", func(t *testing.T) {
		req := &ResetOfferingPickupRequest{Weekday: 1, Date: "2026-08-17"}
		err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))
		require.NoError(t, err)
		assert.Equal(t, "2026-08-17", req.date.String())
	})

	t.Run("rejects a mismatched date", func(t *testing.T) {
		req := &ResetOfferingPickupRequest{Weekday: 2, Date: "2026-08-17"}
		err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))
		require.Error(t, err)
	})
}

func TestPickupResetErrorRenderer_NoOfferingIsConflict(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	require.NoError(t, render.Render(w, r, pickupResetErrorRenderer(ErrPickupResetNoOffering)))

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"students.pickup_reset_requires_offering"`)
}
