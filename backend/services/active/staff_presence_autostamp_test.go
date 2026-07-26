// Tests for the issue #1439 staff-presence auto-stamp: kiosk-driven
// attendance toggles and supervision takeovers best-effort-open the acting
// staff member's work session so they show as "Anwesend".
package active_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// todayWorkSession loads today's work session row for the staff member, or nil.
func todayWorkSession(t *testing.T, db *bun.DB, staffID int64) *active.WorkSession {
	t.Helper()
	session := new(active.WorkSession)
	err := db.NewSelect().
		Model(session).
		ModelTableExpr(`active.work_sessions AS "work_session"`).
		Where(`"work_session"."staff_id" = ?`, staffID).
		Where(`"work_session"."date" = ?`, timezone.TodayDate()).
		Limit(1).
		Scan(context.Background())
	if err != nil {
		return nil
	}
	return session
}

// TestToggleStudentAttendance_IoTAutoOpensWorkSession verifies that a
// kiosk-driven attendance toggle opens the acting staff member's work session.
func TestToggleStudentAttendance_IoTAutoOpensWorkSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)

	student := testpkg.CreateTestStudent(t, db, "AutoStamp", "Student", "3a")
	staff := testpkg.CreateTestStaff(t, db, "AutoStamp", "Supervisor")
	dev := testpkg.CreateTestDevice(t, db, "autostamp-device-1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, dev.ID)

	ctx := context.WithValue(testpkg.TenantContext(1), device.CtxIsIoTDevice, true)

	result, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, dev.ID, true)
	require.NoError(t, err)
	assert.Equal(t, "checked_in", result.Action)

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "kiosk toggle must auto-open the staff work session")
	assert.Equal(t, active.WorkSessionStatusPresent, session.Status)
	assert.Equal(t, active.WorkSessionSourceNFC, session.Source)
}

// TestToggleStudentAttendance_WebOpensWorkSessionWithAppSource verifies that
// a web-side toggle (no IoT device context) also stamps the acting staff
// member, recorded with the app source channel — this is the binary-mode
// "Betreuer markiert Kind als anwesend" path (issue #1439).
func TestToggleStudentAttendance_WebOpensWorkSessionWithAppSource(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)

	student := testpkg.CreateTestStudent(t, db, "WebStamp", "Student", "3b")
	staff := testpkg.CreateTestStaff(t, db, "WebStamp", "Supervisor")
	dev := testpkg.CreateTestDevice(t, db, "autostamp-device-2")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, dev.ID)

	ctx := testpkg.TenantContext(1)

	_, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, dev.ID, true)
	require.NoError(t, err)

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "web toggle must auto-open the staff work session")
	assert.Equal(t, active.WorkSessionSourceApp, session.Source)
}

// TestCreateGroupSupervisor_AutoOpensWorkSession verifies that assigning a
// supervision starting today via the web app opens the staff work session.
func TestCreateGroupSupervisor_AutoOpensWorkSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, "AutoStamp Activity")
	room := testpkg.CreateTestRoom(t, db, "AutoStamp Room")
	staff := testpkg.CreateTestStaff(t, db, "AutoStamp", "WebSupervisor")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, room.ID, staff.ID, activeGroup.ID)

	ctx := testpkg.TenantContext(1)

	supervisor := &active.GroupSupervisor{
		StaffID:   staff.ID,
		GroupID:   activeGroup.ID,
		Role:      "Supervisor",
		StartDate: timezone.TodayDate(),
	}
	require.NoError(t, service.CreateGroupSupervisor(ctx, supervisor))

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "web supervision takeover must auto-open the staff work session")
	assert.Equal(t, active.WorkSessionSourceApp, session.Source)
}
