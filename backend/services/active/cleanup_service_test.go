package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCleanupResult_Structure tests the CleanupResult structure
func TestCleanupResult_Structure(t *testing.T) {
	now := time.Now()
	result := &CleanupResult{
		StartedAt:         now,
		CompletedAt:       now.Add(time.Minute),
		StudentsProcessed: 10,
		RecordsDeleted:    50,
		Errors:            make([]CleanupError, 0),
		Success:           true,
	}

	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 10, result.StudentsProcessed)
	assert.Equal(t, int64(50), result.RecordsDeleted)
	assert.Empty(t, result.Errors)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt))
}

// TestCleanupResult_WithErrors tests CleanupResult with errors
func TestCleanupResult_WithErrors(t *testing.T) {
	now := time.Now()
	result := &CleanupResult{
		StartedAt:         now,
		CompletedAt:       now.Add(time.Minute),
		StudentsProcessed: 5,
		RecordsDeleted:    30,
		Errors: []CleanupError{
			{StudentID: 100, Error: "deletion failed", Timestamp: now},
			{StudentID: 101, Error: "database error", Timestamp: now},
		},
		Success: false,
	}

	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 2)
	assert.Equal(t, int64(100), result.Errors[0].StudentID)
	assert.Equal(t, "deletion failed", result.Errors[0].Error)
}

// TestCleanupError_Structure tests the CleanupError structure
func TestCleanupError_Structure(t *testing.T) {
	now := time.Now()
	err := CleanupError{
		StudentID: 123,
		Error:     "deletion failed: database connection lost",
		Timestamp: now,
	}

	assert.Equal(t, int64(123), err.StudentID)
	assert.Equal(t, "deletion failed: database connection lost", err.Error)
	assert.False(t, err.Timestamp.IsZero())
	assert.True(t, err.Timestamp.Equal(now))
}

// TestRetentionStats_Structure tests the RetentionStats structure
func TestRetentionStats_Structure(t *testing.T) {
	now := time.Now()
	stats := &RetentionStats{
		TotalExpiredVisits:   100,
		StudentsAffected:     25,
		OldestExpiredVisit:   &now,
		ExpiredVisitsByMonth: map[string]int64{"2024-01": 30, "2024-02": 70},
	}

	assert.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.TotalExpiredVisits)
	assert.Equal(t, 25, stats.StudentsAffected)
	assert.NotNil(t, stats.OldestExpiredVisit)
	assert.Len(t, stats.ExpiredVisitsByMonth, 2)
	assert.Equal(t, int64(30), stats.ExpiredVisitsByMonth["2024-01"])
	assert.Equal(t, int64(70), stats.ExpiredVisitsByMonth["2024-02"])
}

// TestRetentionStats_EmptyMonths tests RetentionStats with no monthly data
func TestRetentionStats_EmptyMonths(t *testing.T) {
	stats := &RetentionStats{
		TotalExpiredVisits:   0,
		StudentsAffected:     0,
		OldestExpiredVisit:   nil,
		ExpiredVisitsByMonth: make(map[string]int64),
	}

	assert.Equal(t, int64(0), stats.TotalExpiredVisits)
	assert.Equal(t, 0, stats.StudentsAffected)
	assert.Nil(t, stats.OldestExpiredVisit)
	assert.Empty(t, stats.ExpiredVisitsByMonth)
}

// TestCleanupPreview_Structure tests the CleanupPreview structure
func TestCleanupPreview_Structure(t *testing.T) {
	now := time.Now()
	preview := &CleanupPreview{
		StudentVisitCounts: map[int64]int{100: 10, 101: 15, 102: 5},
		TotalVisits:        30,
		OldestVisit:        &now,
	}

	assert.NotNil(t, preview)
	assert.Equal(t, int64(30), preview.TotalVisits)
	assert.Len(t, preview.StudentVisitCounts, 3)
	assert.Equal(t, 10, preview.StudentVisitCounts[100])
	assert.Equal(t, 15, preview.StudentVisitCounts[101])
	assert.Equal(t, 5, preview.StudentVisitCounts[102])
	assert.NotNil(t, preview.OldestVisit)
}

// TestCleanupPreview_Empty tests empty CleanupPreview
func TestCleanupPreview_Empty(t *testing.T) {
	preview := &CleanupPreview{
		StudentVisitCounts: make(map[int64]int),
		TotalVisits:        0,
		OldestVisit:        nil,
	}

	assert.Equal(t, int64(0), preview.TotalVisits)
	assert.Empty(t, preview.StudentVisitCounts)
	assert.Nil(t, preview.OldestVisit)
}

// TestAttendanceCleanupResult_Structure tests the AttendanceCleanupResult structure
func TestAttendanceCleanupResult_Structure(t *testing.T) {
	now := time.Now()
	result := &AttendanceCleanupResult{
		StartedAt:        now,
		CompletedAt:      now.Add(time.Minute),
		RecordsClosed:    15,
		StudentsAffected: 8,
		OldestRecordDate: &now,
		Success:          true,
		Errors:           make([]string, 0),
	}

	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 15, result.RecordsClosed)
	assert.Equal(t, 8, result.StudentsAffected)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.OldestRecordDate)
}

// TestAttendanceCleanupResult_WithErrors tests AttendanceCleanupResult with errors
func TestAttendanceCleanupResult_WithErrors(t *testing.T) {
	now := time.Now()
	result := &AttendanceCleanupResult{
		StartedAt:        now,
		CompletedAt:      now.Add(time.Minute),
		RecordsClosed:    10,
		StudentsAffected: 5,
		OldestRecordDate: nil,
		Success:          false,
		Errors: []string{
			"Failed to close attendance record 123: database error",
			"Failed to create audit record: permission denied",
		},
	}

	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0], "database error")
	assert.Contains(t, result.Errors[1], "permission denied")
}

// TestAttendanceCleanupPreview_Structure tests the AttendanceCleanupPreview structure
func TestAttendanceCleanupPreview_Structure(t *testing.T) {
	now := time.Now()
	preview := &AttendanceCleanupPreview{
		TotalRecords:   20,
		StudentRecords: map[int64]int{100: 5, 101: 15},
		OldestRecord:   &now,
		RecordsByDate:  map[string]int{"2024-01-15": 10, "2024-01-16": 10},
	}

	assert.NotNil(t, preview)
	assert.Equal(t, 20, preview.TotalRecords)
	assert.Len(t, preview.StudentRecords, 2)
	assert.Equal(t, 5, preview.StudentRecords[100])
	assert.Equal(t, 15, preview.StudentRecords[101])
	assert.Len(t, preview.RecordsByDate, 2)
	assert.Equal(t, 10, preview.RecordsByDate["2024-01-15"])
	assert.NotNil(t, preview.OldestRecord)
}

// TestAttendanceCleanupPreview_Empty tests empty preview
func TestAttendanceCleanupPreview_Empty(t *testing.T) {
	preview := &AttendanceCleanupPreview{
		TotalRecords:   0,
		StudentRecords: make(map[int64]int),
		OldestRecord:   nil,
		RecordsByDate:  make(map[string]int),
	}

	assert.Equal(t, 0, preview.TotalRecords)
	assert.Empty(t, preview.StudentRecords)
	assert.Empty(t, preview.RecordsByDate)
	assert.Nil(t, preview.OldestRecord)
}

// TestBatchResult_Structure tests the internal batchResult structure
func TestBatchResult_Structure(t *testing.T) {
	result := batchResult{
		processed: 10,
		deleted:   50,
		errors:    []CleanupError{{StudentID: 1, Error: "test error", Timestamp: time.Now()}},
	}

	assert.Equal(t, 10, result.processed)
	assert.Equal(t, int64(50), result.deleted)
	assert.Len(t, result.errors, 1)
	assert.Equal(t, int64(1), result.errors[0].StudentID)
}

// TestBatchResult_Empty tests empty batch result
func TestBatchResult_Empty(t *testing.T) {
	result := batchResult{
		processed: 0,
		deleted:   0,
		errors:    make([]CleanupError, 0),
	}

	assert.Equal(t, 0, result.processed)
	assert.Equal(t, int64(0), result.deleted)
	assert.Empty(t, result.errors)
}

// TestStudentWithConsent_Structure tests the internal studentWithConsent structure
func TestStudentWithConsent_Structure(t *testing.T) {
	student := studentWithConsent{
		StudentID:         100,
		DataRetentionDays: 30,
	}

	assert.Equal(t, int64(100), student.StudentID)
	assert.Equal(t, 30, student.DataRetentionDays)
}

// TestStudentWithConsent_DifferentRetentions tests various retention periods
func TestStudentWithConsent_DifferentRetentions(t *testing.T) {
	tests := []struct {
		name      string
		retention int
	}{
		{"minimum 7 days", 7},
		{"default 30 days", 30},
		{"medium 90 days", 90},
		{"long 365 days", 365},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			student := studentWithConsent{
				StudentID:         1,
				DataRetentionDays: tt.retention,
			}
			assert.Equal(t, tt.retention, student.DataRetentionDays)
		})
	}
}

// TestScheduledCheckoutResult_Structure tests the ScheduledCheckoutResult structure
func TestScheduledCheckoutResult_Structure(t *testing.T) {
	now := time.Now()
	result := &ScheduledCheckoutResult{
		ProcessedAt:       now,
		CheckoutsExecuted: 10,
		VisitsEnded:       8,
		AttendanceUpdated: 8,
		Errors:            make([]string, 0),
		Success:           true,
	}

	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 10, result.CheckoutsExecuted)
	assert.Equal(t, 8, result.VisitsEnded)
	assert.Equal(t, 8, result.AttendanceUpdated)
	assert.Empty(t, result.Errors)
}

// TestScheduledCheckoutResult_WithErrors tests ScheduledCheckoutResult with errors
func TestScheduledCheckoutResult_WithErrors(t *testing.T) {
	now := time.Now()
	result := &ScheduledCheckoutResult{
		ProcessedAt:       now,
		CheckoutsExecuted: 5,
		VisitsEnded:       3,
		AttendanceUpdated: 3,
		Errors:            []string{"Failed to end visit for student 100"},
		Success:           false,
	}

	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "student 100")
}

// TestCleanupResult_Duration tests calculating cleanup duration
func TestCleanupResult_Duration(t *testing.T) {
	start := time.Now()
	end := start.Add(5 * time.Minute)

	result := &CleanupResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	duration := result.CompletedAt.Sub(result.StartedAt)
	assert.Equal(t, 5*time.Minute, duration)
}

// TestAttendanceCleanupResult_Duration tests calculating attendance cleanup duration
func TestAttendanceCleanupResult_Duration(t *testing.T) {
	start := time.Now()
	end := start.Add(2 * time.Minute)

	result := &AttendanceCleanupResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	duration := result.CompletedAt.Sub(result.StartedAt)
	assert.Equal(t, 2*time.Minute, duration)
}

// TestCleanupErrorAggregation tests aggregating cleanup errors
func TestCleanupErrorAggregation(t *testing.T) {
	now := time.Now()
	errors := []CleanupError{
		{StudentID: 100, Error: "error 1", Timestamp: now},
		{StudentID: 101, Error: "error 2", Timestamp: now.Add(time.Second)},
		{StudentID: 102, Error: "error 3", Timestamp: now.Add(2 * time.Second)},
	}

	result := &CleanupResult{
		Errors:  errors,
		Success: false,
	}

	assert.Len(t, result.Errors, 3)

	// Test that we can iterate and access all errors
	studentIDs := make([]int64, 0, len(result.Errors))
	for _, err := range result.Errors {
		studentIDs = append(studentIDs, err.StudentID)
	}

	assert.Contains(t, studentIDs, int64(100))
	assert.Contains(t, studentIDs, int64(101))
	assert.Contains(t, studentIDs, int64(102))
}

// TestRetentionStatsMonthlyBreakdown tests accessing monthly breakdown
func TestRetentionStatsMonthlyBreakdown(t *testing.T) {
	stats := &RetentionStats{
		ExpiredVisitsByMonth: map[string]int64{
			"2024-01": 100,
			"2024-02": 150,
			"2024-03": 75,
			"2024-04": 50,
		},
	}

	// Calculate total from monthly stats
	var total int64
	for _, count := range stats.ExpiredVisitsByMonth {
		total += count
	}

	assert.Equal(t, int64(375), total)
	assert.Len(t, stats.ExpiredVisitsByMonth, 4)
}
