package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Active Domain Fixtures
// ============================================================================

// CreateTestAttendance creates a real attendance record in the database
// This requires a student, staff, and device to already exist
//
// Note: The Date field is set to today's local date (not derived from checkInTime)
// to match the repository's GetStudentCurrentStatus query which always queries
// for today's date using local timezone. This ensures tests work correctly
// regardless of when they run (e.g., 00:40 CET is still the same calendar day locally).
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *active.Attendance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use today's date in local time (school operates in local timezone)
	// Repository queries use: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	attendance := &active.Attendance{
		StudentID:    studentID,
		Date:         today,
		CheckInTime:  checkInTime,
		CheckOutTime: checkOutTime,
		CheckedInBy:  staffID,
		DeviceID:     deviceID,
	}

	err := db.NewInsert().
		Model(attendance).
		ModelTableExpr(`active.attendance`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test attendance record")

	return attendance
}
