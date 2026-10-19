package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	ctx, cancel := context.WithTimeout(Ctx(tb), 5*time.Second)
	defer cancel()
	visit := studentpresence.Visit{TenantID: tenantID, StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: entryTime, ExitTime: exitTime}
	err := db.NewRaw(`INSERT INTO active.visits (tenant_id, student_id, active_group_id, entry_time, exit_time)
 VALUES (?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`, tenantID, studentID, activeGroupID, entryTime, exitTime).
		Scan(ctx, &visit.ID, &visit.CreatedAt, &visit.UpdatedAt)
	require.NoError(tb, err, "Failed to create test visit")
	return &visit
}

// CreateTestAttendance records a school stay for today's local calendar date,
// independently of checkInTime. Student, staff and device must already exist.
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	return CreateTestAttendanceForDate(tb, db, studentID, staffID, deviceID, calendar.TodayDate(), checkInTime, checkOutTime)
}

// CreateTestAttendanceForDate records a school stay on an explicit calendar day.
func CreateTestAttendanceForDate(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, date calendar.Date, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	tb.Helper()
	return CreateTestAttendanceForTenant(tb, db, Tenant(tb), studentID, staffID, deviceID, date, checkInTime, checkOutTime)
}

// CreateTestAttendanceForTenant records a school stay in an explicit fixture tenant.
func CreateTestAttendanceForTenant(tb testing.TB, db *bun.DB, tenantID, studentID, staffID, deviceID int64, date calendar.Date, checkInTime time.Time, checkOutTime *time.Time) *studentpresence.Attendance {
	tb.Helper()
	ctx, cancel := context.WithTimeout(Ctx(tb), 5*time.Second)
	defer cancel()
	attendance := studentpresence.Attendance{TenantID: tenantID, StudentID: studentID, Date: date.String(), CheckInTime: checkInTime, CheckOutTime: checkOutTime, CheckedInBy: staffID, DeviceID: deviceID}
	err := db.NewRaw(`INSERT INTO active.attendance (tenant_id, student_id, date, check_in_time, check_out_time, checked_in_by, device_id)
 VALUES (?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0)) RETURNING id, created_at, updated_at`, tenantID, studentID, date, checkInTime, checkOutTime, staffID, deviceID).
		Scan(ctx, &attendance.ID, &attendance.CreatedAt, &attendance.UpdatedAt)
	require.NoError(tb, err, "Failed to create test attendance record")
	return &attendance
}

// CheckOutTestAttendanceByStaff closes the student's school stay of date as
// the web checkout does: booked by the staff member, without a device.
func CheckOutTestAttendanceByStaff(tb testing.TB, db *bun.DB, studentID int64, date string, at time.Time, staffID int64) {
	tb.Helper()
	_, err := db.NewUpdate().Table("active.attendance").
		Set("check_out_time = ?", at).Set("checked_out_by = ?", staffID).Set("yard_since = NULL").Set("updated_at = NOW()").
		Where("tenant_id = ?", Tenant(tb)).Where("student_id = ?", studentID).Where("date = ?", date).Where("check_out_time IS NULL").Exec(Ctx(tb))
	require.NoError(tb, err, "Failed to check out test attendance")
}

// EndedActiveGroup is an active group and one supervisor whose session
// already ended, as an activity completion snapshot leaves them.
type EndedActiveGroup struct {
	GroupID      int64
	SupervisorID int64
}

// EndTestActiveGroup ends the group and supervisor rows outside any measured
// or tenant-scoped context so recovery tests can restore them.
func EndTestActiveGroup(tb testing.TB, db *bun.DB, ended EndedActiveGroup) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewUpdate().Table("active.groups").Set("end_time = now()").Where("id = ?", ended.GroupID).Exec(ctx)
	require.NoError(tb, err, "Failed to end test active group")
	_, err = db.NewUpdate().Table("active.group_supervisors").Set("end_date = CURRENT_DATE").Where("id = ?", ended.SupervisorID).Exec(ctx)
	require.NoError(tb, err, "Failed to end test group supervisor")
}

// CreateTestEndedActiveGroup creates staff, activity, room, an active group
// with one supervisor, and ends both rows.
func CreateTestEndedActiveGroup(tb testing.TB, db *bun.DB, label string) EndedActiveGroup {
	tb.Helper()
	staff := CreateTestStaff(tb, db, "Recovery", label)
	activity := CreateTestActivityGroup(tb, db, "Recovery "+label)
	room := CreateTestRoom(tb, db, "Recovery "+label)
	group := CreateTestActiveGroup(tb, db, activity.ID, room.ID)
	supervisor := CreateTestGroupSupervisor(tb, db, staff.ID, group.ID, "supervisor")
	ended := EndedActiveGroup{GroupID: group.ID, SupervisorID: supervisor.ID}
	EndTestActiveGroup(tb, db, ended)
	return ended
}

// ActiveGroupEnded reports whether the group and supervisor rows are ended.
func ActiveGroupEnded(tb testing.TB, db *bun.DB, ended EndedActiveGroup) (groupEnded, supervisorEnded bool) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(tb, db.NewSelect().TableExpr("active.groups").ColumnExpr("end_time IS NOT NULL").Where("id = ?", ended.GroupID).Scan(ctx, &groupEnded))
	require.NoError(tb, db.NewSelect().TableExpr("active.group_supervisors").ColumnExpr("end_date IS NOT NULL").Where("id = ?", ended.SupervisorID).Scan(ctx, &supervisorEnded))
	return groupEnded, supervisorEnded
}

// CreateTestScheduledCheckout records one pending scheduled checkout for the
// fixture tenant and returns its row ID. The presence capability has no write
// operation for this table, so the row is inserted directly with the tenant
// column set. A student can hold only one pending checkout (partial unique
// index), so call it at most once per student.
func CreateTestScheduledCheckout(tb testing.TB, db *bun.DB, studentID, staffID int64, scheduledFor time.Time) int64 {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO active.scheduled_checkouts (tenant_id, student_id, scheduled_by, scheduled_for, status)
		VALUES (?, ?, ?, ?, 'pending')
		RETURNING id
	`, Tenant(tb), studentID, staffID, scheduledFor).Scan(ctx, &id)
	require.NoError(tb, err, "Failed to create test scheduled checkout")
	return id
}
