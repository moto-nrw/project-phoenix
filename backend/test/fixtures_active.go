package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

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

// CreateTestActiveGroup creates a real active group (session) in the database.
// This requires an ActivityGroup (activities.groups) and Room to exist.
// Use this for testing session management and visit tracking.
func CreateTestActiveGroup(tb testing.TB, db *bun.DB, activityGroupID, roomID int64) *active.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	activeGroup := &active.Group{
		GroupID:        activityGroupID,
		RoomID:         roomID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group")

	return activeGroup
}

// CreateTestVisit creates a real visit record in the database.
// This requires a Student and ActiveGroup to already exist.
func CreateTestVisit(tb testing.TB, db *bun.DB, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *active.Visit {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	visit := &active.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     entryTime,
		ExitTime:      exitTime,
	}

	err := db.NewInsert().
		Model(visit).
		ModelTableExpr(`active.visits`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test visit")

	return visit
}

// CreateTestGroupSupervisor creates a real group supervisor record in the database.
// This requires a Staff and ActiveGroup to already exist.
func CreateTestGroupSupervisor(tb testing.TB, db *bun.DB, staffID, activeGroupID int64, role string) *active.GroupSupervisor {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   activeGroupID,
		Role:      role,
		StartDate: time.Now(),
	}

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`active.group_supervisors`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group supervisor")

	return supervisor
}
