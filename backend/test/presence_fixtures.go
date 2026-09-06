package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestVisit records a room stay. Student and active group must already exist.
func CreateTestVisit(tb testing.TB, db *bun.DB, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *studentpresence.Visit {
	tb.Helper()
	return CreateTestVisitForTenant(tb, db, Tenant(tb), studentID, activeGroupID, entryTime, exitTime)
}

// CreateTestVisitForTenant records a room stay in an explicit fixture tenant.
func CreateTestVisitForTenant(tb testing.TB, db *bun.DB, tenantID, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *studentpresence.Visit {
	tb.Helper()
	ctx, cancel := context.WithTimeout(tenant.WithTenantID(Ctx(tb), tenantID), 5*time.Second)
	defer cancel()
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(tb, err)
	visit, err := presence.RecordVisit(ctx, studentpresence.Visit{
		StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: entryTime, ExitTime: exitTime,
	})
	require.NoError(tb, err, "Failed to create test visit")
	return &visit
}

// CreateTestAttendance records a school stay for today's local calendar date,
// independently of checkInTime. Student, staff and device must already exist.
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	return CreateTestAttendanceForDate(tb, db, studentID, staffID, deviceID, timezone.TodayDate(), checkInTime, checkOutTime)
}

// CreateTestAttendanceForDate records a school stay on an explicit calendar day.
func CreateTestAttendanceForDate(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, date timezone.Date, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	tb.Helper()
	return CreateTestAttendanceForTenant(tb, db, Tenant(tb), studentID, staffID, deviceID, date, checkInTime, checkOutTime)
}

// CreateTestAttendanceForTenant records a school stay in an explicit fixture tenant.
func CreateTestAttendanceForTenant(tb testing.TB, db *bun.DB, tenantID, studentID, staffID, deviceID int64, date timezone.Date, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	tb.Helper()
	ctx, cancel := context.WithTimeout(tenant.WithTenantID(Ctx(tb), tenantID), 5*time.Second)
	defer cancel()
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(tb, err)
	attendance, err := presence.RecordAttendance(ctx, studentpresence.Attendance{
		StudentID: studentID, Date: date.String(), CheckInTime: checkInTime, CheckOutTime: checkOutTime,
		CheckedInBy: staffID, DeviceID: deviceID,
	})
	require.NoError(tb, err, "Failed to create test attendance record")
	return &attendance
}
