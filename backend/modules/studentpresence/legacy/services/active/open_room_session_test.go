package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	activeSvc "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRoomPresence is the narrow surface the open-room move workflow binds to
// (#3066). Asserting it here proves the retained service really provides it.
type openRoomPresence interface {
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*activeModels.Group, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
}

func openRoomService(t *testing.T, db *bun.DB) (activeSvc.Service, openRoomPresence) {
	t.Helper()
	svc := setupActiveService(t, db)
	presence, ok := svc.(openRoomPresence)
	require.True(t, ok, "the retained active service must provide the open-room operations the workflow binds")
	return svc, presence
}

// presentChild checks a child in (open attendance) and places it in a session.
func presentChild(t *testing.T, db *bun.DB, label string, staffID, deviceID, sessionID int64) int64 {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, "Kind", label, "3a")
	now := time.Now()
	testpkg.CreateTestAttendance(t, db, student.ID, staffID, deviceID, now.Add(-time.Hour), nil)
	testpkg.CreateTestVisit(t, db, student.ID, sessionID, now.Add(-30*time.Minute), nil)
	return student.ID
}

func openVisitSessions(t *testing.T, presence *studentpresence.Module, studentIDs []int64) map[int64]int64 {
	t.Helper()
	visits, err := presence.ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true})
	require.NoError(t, err)
	sessions := make(map[int64]int64, len(visits))
	for _, visit := range visits {
		_, duplicate := sessions[visit.StudentID]
		require.False(t, duplicate, "child %d has more than one open visit", visit.StudentID)
		sessions[visit.StudentID] = visit.ActiveGroupID
	}
	return sessions
}

func assertStillAttending(t *testing.T, presence *studentpresence.Module, studentIDs []int64) {
	t.Helper()
	attendance, err := presence.ListAttendance(testpkg.Ctx(t), studentpresence.AttendanceFilter{StudentIDs: studentIDs})
	require.NoError(t, err)
	require.Len(t, attendance, len(studentIDs))
	for _, row := range attendance {
		assert.Nil(t, row.CheckOutTime, "closing a room stay must not check child %d out of the OGS", row.StudentID)
	}
}

func TestEnsureOpenRoomSessionRunsBesideAnActivitySession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, presence := openRoomService(t, db)
	ctx := testpkg.Ctx(t)

	gym := testpkg.CreateTestRoom(t, db, "Turnhalle Raumsitzung")
	football := testpkg.CreateTestActivityGroup(t, db, "Fußball Raumsitzung")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Raumaufenthalt Raumsitzung")
	footballSession := testpkg.CreateTestActiveGroup(t, db, football.ID, gym.ID)

	first, err := presence.EnsureOpenRoomSession(ctx, gym.ID, roomActivity.ID)
	require.NoError(t, err, "a running activity must not block the room session")
	assert.NotEqual(t, footballSession.ID, first.ID, "the room session is not the activity session")
	assert.Nil(t, first.DeviceID)
	require.NotNil(t, first.GroupID)
	assert.Equal(t, roomActivity.ID, *first.GroupID)
	assert.Equal(t, gym.ID, first.RoomID)

	second, err := presence.EnsureOpenRoomSession(ctx, gym.ID, roomActivity.ID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "the room keeps a single room session")
}

func TestEnsureOpenRoomSessionJoinsTheKioskSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, presence := openRoomService(t, db)
	ctx := testpkg.Ctx(t)

	yard := testpkg.CreateTestRoom(t, db, "Schulhof Kiosk")
	play := testpkg.CreateTestActivityGroup(t, db, "Schulhof Freispiel Kiosk")
	device := testpkg.CreateTestDevice(t, db, "schulhof-kiosk-device")
	kiosk := testpkg.CreateTestActiveGroup(t, db, play.ID, yard.ID)
	_, err := db.NewUpdate().
		Table("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", kiosk.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(ctx)
	require.NoError(t, err)

	session, err := presence.EnsureOpenRoomSession(ctx, yard.ID, play.ID)
	require.NoError(t, err)
	assert.Equal(t, kiosk.ID, session.ID, "a phone move must join the running kiosk session")
	require.NotNil(t, session.DeviceID)
	assert.Equal(t, device.ID, *session.DeviceID)
}

func TestKioskStartJoinsAPhoneCreatedRoomStay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc, presence := openRoomService(t, db)
	ctx := testpkg.Ctx(t)

	yard := testpkg.CreateTestRoom(t, db, "Schulhof Phone zuerst")
	play := testpkg.CreateTestActivityGroup(t, db, "Schulhof Freispiel Phone zuerst")
	markActivitySystem(t, db, play.ID)
	stay, err := presence.EnsureOpenRoomSession(ctx, yard.ID, play.ID)
	require.NoError(t, err)
	assert.Nil(t, stay.DeviceID)

	staff := testpkg.CreateTestStaff(t, db, "Kiosk", "Start")
	device := testpkg.CreateTestDevice(t, db, "schulhof-after-phone")
	conflict, err := svc.CheckActivityConflict(ctx, play.ID, device.ID)
	require.NoError(t, err)
	assert.False(t, conflict.HasConflict, "preflight must not treat a joinable independent stay as a conflict")
	started, err := svc.StartActivitySessionWithSupervisors(ctx, play.ID, device.ID, []int64{staff.ID}, &yard.ID)
	require.NoError(t, err, "a kiosk start must join the phone-created stay")
	assert.Equal(t, stay.ID, started.ID)
	require.NotNil(t, started.DeviceID)
	assert.Equal(t, device.ID, *started.DeviceID)
}

func TestKioskStartConflictsWithAPlannerSessionInTheSameRoom(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	gym := testpkg.CreateTestRoom(t, db, "Turnhalle Planer")
	football := testpkg.CreateTestActivityGroup(t, db, "Fußball Planer")
	planner := testpkg.CreateTestActiveGroup(t, db, football.ID, gym.ID)
	assert.Nil(t, planner.DeviceID, "planner and app starts are device-less")

	staff := testpkg.CreateTestStaff(t, db, "Kiosk", "Fußball")
	device := testpkg.CreateTestDevice(t, db, "turnhalle-kiosk-planer")
	conflict, err := svc.CheckActivityConflict(ctx, football.ID, device.ID)
	require.NoError(t, err)
	assert.True(t, conflict.HasConflict, "preflight must keep a planner session as a conflict")
	_, err = svc.StartActivitySessionWithSupervisors(ctx, football.ID, device.ID, []int64{staff.ID}, &gym.ID)
	require.ErrorIs(t, err, activeSvc.ErrSessionConflict)

	var still activeModels.Group
	require.NoError(t, db.NewSelect().
		Model(&still).
		ModelTableExpr(`active.groups AS "group"`).
		Where("id = ?", planner.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Scan(ctx))
	assert.Nil(t, still.EndTime, "the planner session must keep running")
	assert.Nil(t, still.DeviceID, "the kiosk must not attach to the planner session")
}

func TestOpenRoomMoveNeedsNoSupervisionAtTheDestination(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc, openRoom := openRoomService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	groupRoom := testpkg.CreateTestRoom(t, db, "Gruppenraum Ziel")
	homework := testpkg.CreateTestActivityGroup(t, db, "Hausaufgaben Ziel")
	source := testpkg.CreateTestActiveGroup(t, db, homework.ID, groupRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "Quelle", "Aufsicht")
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, source.ID, "supervisor")
	device := testpkg.CreateTestDevice(t, db, "open-room-destination")
	child := presentChild(t, db, "Ziel", staff.ID, device.ID, source.ID)

	craftRoom := testpkg.CreateTestRoom(t, db, "Werkraum Ziel")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Raumaufenthalt Ziel")
	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, craftRoom.ID, roomActivity.ID)
	require.NoError(t, err)
	auth := activeSvc.StudentMoveAuthorization{StaffID: staff.ID}

	// The ordinary push move still requires a supervised destination ...
	_, err = svc.MoveStudentsToActiveGroupAuthorized(ctx, []int64{child}, roomSession.ID, auth)
	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Equal(t, source.ID, openVisitSessions(t, presence, []int64{child})[child], "a rejected move leaves the child where it was")

	// ... the move into a released room's session does not.
	result, err := openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, auth)
	require.NoError(t, err)
	assert.Equal(t, []int64{child}, result.Moved)
	assert.Equal(t, roomSession.ID, openVisitSessions(t, presence, []int64{child})[child])
	assertStillAttending(t, presence, []int64{child})
}

func TestOpenRoomMoveKeepsTheSourceSideRights(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, openRoom := openRoomService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	groupRoom := testpkg.CreateTestRoom(t, db, "Gruppenraum Rechte")
	homework := testpkg.CreateTestActivityGroup(t, db, "Hausaufgaben Rechte")
	source := testpkg.CreateTestActiveGroup(t, db, homework.ID, groupRoom.ID)
	supervisor := testpkg.CreateTestStaff(t, db, "Andere", "Aufsicht")
	testpkg.CreateTestGroupSupervisor(t, db, supervisor.ID, source.ID, "supervisor")
	outsider := testpkg.CreateTestStaff(t, db, "Ohne", "Aufsicht")
	device := testpkg.CreateTestDevice(t, db, "open-room-rights")
	child := presentChild(t, db, "Rechte", supervisor.ID, device.ID, source.ID)

	craftRoom := testpkg.CreateTestRoom(t, db, "Werkraum Rechte")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Raumaufenthalt Rechte")
	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, craftRoom.ID, roomActivity.ID)
	require.NoError(t, err)

	// A staff member who does not supervise the child's current place cannot
	// move it anywhere, a released room included.
	_, err = openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, activeSvc.StudentMoveAuthorization{StaffID: outsider.ID})
	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Equal(t, source.ID, openVisitSessions(t, presence, []int64{child})[child])
}

func TestOpenRoomMoveRepeatedRecordsOneStay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, openRoom := openRoomService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	groupRoom := testpkg.CreateTestRoom(t, db, "Gruppenraum Wiederholung")
	homework := testpkg.CreateTestActivityGroup(t, db, "Hausaufgaben Wiederholung")
	source := testpkg.CreateTestActiveGroup(t, db, homework.ID, groupRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "Wiederholung", "Aufsicht")
	device := testpkg.CreateTestDevice(t, db, "open-room-repeat")
	child := presentChild(t, db, "Wiederholung", staff.ID, device.ID, source.ID)

	craftRoom := testpkg.CreateTestRoom(t, db, "Werkraum Wiederholung")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Raumaufenthalt Wiederholung")
	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, craftRoom.ID, roomActivity.ID)
	require.NoError(t, err)

	first, err := openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, activeSvcBypassAuth)
	require.NoError(t, err)
	assert.Equal(t, []int64{child}, first.Moved)

	second, err := openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, activeSvcBypassAuth)
	require.NoError(t, err)
	assert.Empty(t, second.Moved, "a repeated move must not record a second stay")
	assert.Equal(t, []int64{child}, second.Unchanged)

	visits, err := presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{child}})
	require.NoError(t, err)
	assert.Len(t, visits, 2, "one closed source visit and exactly one room stay")
	assert.Equal(t, roomSession.ID, openVisitSessions(t, presence, []int64{child})[child])
}

// TestEndingAnActivityLeavesIndependentRoomStays is the specification's
// eight-plus-three scenario (#3062): eight children are at football, three
// stay independently in the same released gym. One of the three moved out of
// football before it ended. Ending football sends only its eight remaining
// children to Unterwegs; the global daily close then ends the room stays too,
// without checking anybody out of the OGS.
func TestEndingAnActivityLeavesIndependentRoomStays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc, openRoom := openRoomService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	gym := testpkg.CreateTestRoom(t, db, "Turnhalle Acht plus drei")
	football := testpkg.CreateTestActivityGroup(t, db, "Fußball Acht plus drei")
	footballSession := testpkg.CreateTestActiveGroup(t, db, football.ID, gym.ID)
	schoolyardRoom := testpkg.CreateTestRoom(t, db, "Hof Acht plus drei")
	playActivity := testpkg.CreateTestActivityGroup(t, db, "Freispiel Acht plus drei")
	elsewhere := testpkg.CreateTestActiveGroup(t, db, playActivity.ID, schoolyardRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "Acht", "Plus Drei")
	device := testpkg.CreateTestDevice(t, db, "open-room-eight-plus-three")

	footballers := make([]int64, 0, 8)
	for i := range 8 {
		footballers = append(footballers, presentChild(t, db, "Fußball"+string(rune('A'+i)), staff.ID, device.ID, footballSession.ID))
	}
	leftFootball := presentChild(t, db, "Verlassen", staff.ID, device.ID, footballSession.ID)
	independents := []int64{
		leftFootball,
		presentChild(t, db, "RaumA", staff.ID, device.ID, elsewhere.ID),
		presentChild(t, db, "RaumB", staff.ID, device.ID, elsewhere.ID),
	}

	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Raumaufenthalt Acht plus drei")
	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, gym.ID, roomActivity.ID)
	require.NoError(t, err)
	moved, err := openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, independents, roomSession.ID, activeSvcBypassAuth)
	require.NoError(t, err)
	require.ElementsMatch(t, independents, moved.Moved)

	all := append(append([]int64{}, footballers...), independents...)
	before := openVisitSessions(t, presence, all)
	for _, id := range footballers {
		require.Equal(t, footballSession.ID, before[id])
	}
	for _, id := range independents {
		require.Equal(t, roomSession.ID, before[id], "the independent stay is not participation in football")
	}

	// Football ends: only its eight remaining children become Unterwegs.
	require.NoError(t, svc.EndActivitySession(ctx, footballSession.ID))
	afterFootball := openVisitSessions(t, presence, all)
	for _, id := range footballers {
		_, stillInRoom := afterFootball[id]
		assert.False(t, stillInRoom, "football child %d must become Unterwegs", id)
	}
	for _, id := range independents {
		assert.Equal(t, roomSession.ID, afterFootball[id], "independent child %d must stay in the gym", id)
	}
	assertStillAttending(t, presence, all)

	// The global daily close ends the room stays as well, not the attendance.
	_, err = svc.EndDailySessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, openVisitSessions(t, presence, all), "the daily close must not keep yesterday's room stays")
	assertStillAttending(t, presence, all)
}

// TestStartingAnActivityLeavesIndependentRoomStays is the reverse of the
// eight-plus-three scenario: the independent stay is already in the released
// gym when football starts. Occupancy must not 409, and the stay must not be
// pulled into football.
func TestStartingAnActivityLeavesIndependentRoomStays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc, openRoom := openRoomService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	gym := testpkg.CreateTestRoom(t, db, "Turnhalle Start danach")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "Offener Raum Start danach")
	markActivitySystem(t, db, roomActivity.ID)
	staff := testpkg.CreateTestStaff(t, db, "Start", "Danach")
	device := testpkg.CreateTestDevice(t, db, "open-room-start-after")
	elsewhereRoom := testpkg.CreateTestRoom(t, db, "Hof Start danach")
	play := testpkg.CreateTestActivityGroup(t, db, "Freispiel Start danach")
	elsewhere := testpkg.CreateTestActiveGroup(t, db, play.ID, elsewhereRoom.ID)
	child := presentChild(t, db, "Unabhängig", staff.ID, device.ID, elsewhere.ID)

	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, gym.ID, roomActivity.ID)
	require.NoError(t, err)
	moved, err := openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, activeSvcBypassAuth)
	require.NoError(t, err)
	require.Equal(t, []int64{child}, moved.Moved)

	football := testpkg.CreateTestActivityGroup(t, db, "Fußball Start danach")
	footballDevice := testpkg.CreateTestDevice(t, db, "open-room-football-start")
	footballSession, err := svc.StartActivitySessionWithSupervisors(ctx, football.ID, footballDevice.ID, []int64{staff.ID}, &gym.ID)
	require.NoError(t, err, "an independent stay must not occupy the room against a later activity")
	require.NotEqual(t, roomSession.ID, footballSession.ID)

	assert.Equal(t, roomSession.ID, openVisitSessions(t, presence, []int64{child})[child], "starting football must not absorb the independent stay")
	require.NoError(t, svc.EndActivitySession(ctx, footballSession.ID))
	assert.Equal(t, roomSession.ID, openVisitSessions(t, presence, []int64{child})[child], "ending football must leave the independent stay")
	assertStillAttending(t, presence, []int64{child})
}

func markActivitySystem(t *testing.T, db *bun.DB, activityID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", activityID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}
