// Tests for the issue #1439 staff-presence auto-stamp: kiosk-driven
// attendance toggles and supervision takeovers best-effort-open the acting
// staff member's work session so they show as "Anwesend".
package presence_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workSessionRow is the part of a work session row the auto-stamp asserts.
type workSessionRow struct {
	Status string
	Source string
}

// todayWorkSession loads today's work session row for the staff member, or nil.
func todayWorkSession(t *testing.T, db *testpkg.DB, staffID int64) *workSessionRow {
	t.Helper()
	session := new(workSessionRow)
	err := db.NewSelect().
		TableExpr(`active.work_sessions AS "work_session"`).
		ColumnExpr(`"work_session"."status", "work_session"."source"`).
		Where(`"work_session"."staff_id" = ?`, staffID).
		Where(`"work_session"."date" = ?`, calendar.TodayDate()).
		Limit(1).
		Scan(context.Background(), &session.Status, &session.Source)
	if err != nil {
		return nil
	}
	return session
}

// TestToggleStudentAttendance_IoTAutoOpensWorkSession verifies that a
// kiosk-driven attendance toggle opens the acting staff member's work session.
func TestToggleStudentAttendance_IoTAutoOpensWorkSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)

	student := testpkg.CreateTestStudent(t, db, "AutoStamp", "Student", "3a")
	staff := testpkg.CreateTestStaff(t, db, "AutoStamp", "Supervisor")
	dev := testpkg.CreateTestDevice(t, db, "autostamp-device-1")

	ctx := api.WithIoTAttendanceRequest(testpkg.Ctx(t))

	result, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, dev.ID, true)
	require.NoError(t, err)
	assert.Equal(t, "checked_in", result.Action)

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "kiosk toggle must auto-open the staff work session")
	assert.Equal(t, workforce.WorkSessionStatusPresent, session.Status)
	assert.Equal(t, workforce.WorkSessionSourceNFC, session.Source)
}

// TestToggleStudentAttendance_WebOpensWorkSessionWithAppSource verifies that
// a web-side toggle (no IoT device context) also stamps the acting staff
// member, recorded with the app source channel — this is the binary-mode
// "Betreuer markiert Kind als anwesend" path (issue #1439).
func TestToggleStudentAttendance_WebOpensWorkSessionWithAppSource(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)

	student := testpkg.CreateTestStudent(t, db, "WebStamp", "Student", "3b")
	staff := testpkg.CreateTestStaff(t, db, "WebStamp", "Supervisor")
	dev := testpkg.CreateTestDevice(t, db, "autostamp-device-2")

	ctx := testpkg.Ctx(t)

	_, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, dev.ID, true)
	require.NoError(t, err)

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "web toggle must auto-open the staff work session")
	assert.Equal(t, workforce.WorkSessionSourceApp, session.Source)
}

// TestCreateGroupSupervisor_AutoOpensWorkSession verifies that assigning a
// supervision starting today via the web app opens the staff work session.
func TestCreateGroupSupervisor_AutoOpensWorkSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, "AutoStamp Activity")
	room := testpkg.CreateTestRoom(t, db, "AutoStamp Room")
	staff := testpkg.CreateTestStaff(t, db, "AutoStamp", "WebSupervisor")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	ctx := testpkg.Ctx(t)

	supervisor := &ports.GroupSupervisor{
		StaffID:   staff.ID,
		GroupID:   activeGroup.ID,
		Role:      "Supervisor",
		StartDate: calendar.TodayDate(),
	}
	require.NoError(t, service.CreateGroupSupervisor(ctx, supervisor))

	session := todayWorkSession(t, db, staff.ID)
	require.NotNil(t, session, "web supervision takeover must auto-open the staff work session")
	assert.Equal(t, workforce.WorkSessionSourceApp, session.Source)
}

func TestCreateGroupSupervisor_SkipsWorkSessionForExternalCaregiver(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, "External Activity")
	room := testpkg.CreateTestRoom(t, db, "External Room")
	staff := testpkg.CreateTestStaff(t, db, "External", "Caregiver")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	require.NoError(t, service.CreateGroupSupervisor(testpkg.Ctx(t), &ports.GroupSupervisor{
		StaffID: staff.ID, GroupID: activeGroup.ID, Role: "additional_supervisor",
		StartDate: calendar.TodayDate(), SkipPresenceStamp: true,
	}))

	assert.Nil(t, todayWorkSession(t, db, staff.ID), "external caregivers do not record working time")
}

// createAccountBackedExternal creates an external caregiver with an account:
// a staff member with a login and an active guest profile (#4573).
func createAccountBackedExternal(t *testing.T, db *testpkg.DB, lastName string) int64 {
	t.Helper()
	staff, _ := testpkg.CreateTestStaffWithAccount(t, db, "Extern", lastName)
	guest := &testpkg.Guest{TenantID: testpkg.Tenant(t), StaffID: staff.ID, ActivityExpertise: "Musik"}
	_, err := db.NewInsert().Model(guest).ModelTableExpr(`users.guests`).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return staff.ID
}

// An account does not make an external caregiver record working time
// (#4573): joining a supervision opens no block for them, even when the
// caller asks for the stamp. Workforce decides it, so every stamping path
// is covered; a regular colleague in the same supervision still gets one.
func TestCreateGroupSupervisor_SkipsWorkSessionForAccountBackedExternal(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, "External Account Activity")
	room := testpkg.CreateTestRoom(t, db, "External Account Room")
	externalID := createAccountBackedExternal(t, db, "WebSupervisor")
	colleague := testpkg.CreateTestStaff(t, db, "Regular", "WebSupervisor")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	for _, staffID := range []int64{externalID, colleague.ID} {
		require.NoError(t, service.CreateGroupSupervisor(testpkg.Ctx(t), &ports.GroupSupervisor{
			StaffID: staffID, GroupID: activeGroup.ID, Role: "Supervisor", StartDate: calendar.TodayDate(),
		}))
	}

	assert.Nil(t, todayWorkSession(t, db, externalID), "external caregivers do not record working time")
	require.NotNil(t, todayWorkSession(t, db, colleague.ID), "the regular colleague is stamped")
}

// The kiosk path stamps through the same Workforce check.
func TestToggleStudentAttendance_IoTSkipsWorkSessionForAccountBackedExternal(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)

	student := testpkg.CreateTestStudent(t, db, "AutoStampExtern", "Student", "3a")
	externalID := createAccountBackedExternal(t, db, "Kiosk")
	dev := testpkg.CreateTestDevice(t, db, "autostamp-device-external")

	result, err := service.ToggleStudentAttendance(api.WithIoTAttendanceRequest(testpkg.Ctx(t)), student.ID, externalID, dev.ID, true)
	require.NoError(t, err)
	assert.Equal(t, "checked_in", result.Action)

	assert.Nil(t, todayWorkSession(t, db, externalID), "external caregivers do not record working time")
}
