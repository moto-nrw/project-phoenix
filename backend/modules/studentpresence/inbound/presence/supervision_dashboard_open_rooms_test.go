package presence_test

// The shared open-room view over its real HTTP surface (#3065).
//
// The promises under test are the ones a caregiver actually depends on: a
// released room is reachable while empty and while somebody else supervises
// it, everything running in it converges into one list with each child once,
// and none of that reaches across the tenant boundary.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type wireOpenRoomStudent struct {
	StudentID     string `json:"student_id"`
	StudentName   string `json:"student_name"`
	SchoolClass   string `json:"school_class"`
	ActivityName  string `json:"activity_name"`
	ActiveGroupID string `json:"active_group_id"`
	Independent   bool   `json:"independent"`
}

type wireOpenRoom struct {
	RoomID              string                `json:"room_id"`
	Name                string                `json:"name"`
	IsUserSupervising   bool                  `json:"is_user_supervising"`
	ActiveGroupIDs      []string              `json:"active_group_ids"`
	HasOccupyingSession bool                  `json:"has_occupying_session"`
	StudentCount        int                   `json:"student_count"`
	Students            []wireOpenRoomStudent `json:"students"`
}

type openRoomEnvelope struct {
	Data struct {
		OpenRooms []wireOpenRoom `json:"open_rooms"`
	} `json:"data"`
}

// openRooms reads the shared view off the real endpoint, keyed by room name so
// a test can name the room it means without carrying ids around.
func openRooms(t *testing.T, router testutil.Router, accountID int64) map[string]wireOpenRoom {
	t.Helper()

	req := testutil.NewRequest("GET", "/active/supervision-dashboard", nil)
	rr := testutil.ExecuteWithAuthPermissions(
		t, router, req, testutil.TeacherTestClaims(int(accountID)), dashboardPerms,
	)
	require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var envelope openRoomEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))

	byName := make(map[string]wireOpenRoom, len(envelope.Data.OpenRooms))
	for _, room := range envelope.Data.OpenRooms {
		byName[room.Name] = room
	}
	return byName
}

// TestOpenRooms_ReachableWithoutOwnSupervision is the central promise: a
// caregiver who supervises nothing still finds every released room, empty ones
// included. The navigation this replaces derived its rooms from running
// supervisions and could show neither.
func TestOpenRooms_ReachableWithoutOwnSupervision(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomBystander", "Leader")
	empty := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomEmpty")
	busy := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomBusy")

	// Somebody else's session, in a room the caller has nothing to do with.
	otherTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomOther", "Leader")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomFussball")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, busy.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, otherTeacher.Staff.ID, activeGroup.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, tc.db, "OpenRoom", "Kind", "OR1")
	testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)

	rooms := openRooms(t, router, account.ID)

	require.Contains(t, rooms, empty.Name, "a released room with nothing running is still reachable")
	assert.Empty(t, rooms[empty.Name].ActiveGroupIDs)
	assert.Zero(t, rooms[empty.Name].StudentCount)
	assert.Empty(t, rooms[empty.Name].Students)
	assert.False(t, rooms[empty.Name].IsUserSupervising)

	require.Contains(t, rooms, busy.Name)
	assert.Equal(t, 1, rooms[busy.Name].StudentCount,
		"the child is visible to a caregiver who supervises nothing here")
	assert.False(t, rooms[busy.Name].IsUserSupervising,
		"seeing a shared room is not supervising it")
}

// TestOpenRooms_MergeParallelSessionsOfOneRoom is the criterion the navigation
// this replaces broke: two offerings in one room produced one tab each, so the
// room was on screen twice and neither entry showed everyone in it.
//
// Each child appearing exactly once needs no extra arrangement here — the open
// visit is unique per child — so what this pins is the merge itself and the
// offering that travels with each child.
func TestOpenRooms_MergeParallelSessionsOfOneRoom(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomMerge", "Leader")
	hall := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomSporthalle")

	football := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomFussball")
	dance := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomTanzen")
	footballSession := testpkg.CreateTestActiveGroup(t, tc.db, football.ID, hall.ID)
	danceSession := testpkg.CreateTestActiveGroup(t, tc.db, dance.ID, hall.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, footballSession.ID, "supervisor")

	early := testpkg.CreateTestStudent(t, tc.db, "OpenRoomMerge", "Fruehkind", "OM1")
	late := testpkg.CreateTestStudent(t, tc.db, "OpenRoomMerge", "Spaetkind", "OM2")
	now := time.Now()
	testpkg.CreateTestVisit(t, tc.db, early.ID, footballSession.ID, now.Add(-90*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, late.ID, danceSession.ID, now.Add(-20*time.Minute), nil)

	rooms := openRooms(t, router, account.ID)

	room, found := rooms[hall.Name]
	require.True(t, found)
	assert.ElementsMatch(t,
		[]string{strconv.FormatInt(footballSession.ID, 10), strconv.FormatInt(danceSession.ID, 10)},
		room.ActiveGroupIDs, "both sessions feed the one shared view")
	assert.True(t, room.IsUserSupervising, "the caller supervises one of the sessions here")

	require.Len(t, room.Students, 2,
		"both sessions' children are in the one list, each exactly once")
	assert.Equal(t, 2, room.StudentCount)

	byID := map[string]string{}
	for _, student := range room.Students {
		byID[student.StudentID] = student.ActivityName
	}
	assert.Equal(t, football.Name, byID[strconv.FormatInt(early.ID, 10)],
		"the offering a child is recorded under travels with them")
	assert.Equal(t, dance.Name, byID[strconv.FormatInt(late.ID, 10)],
		"and the second session's children keep theirs")
}

// TestOpenRooms_MarkIndependentStaysApartFromTheActivity: a child moved into
// a released room stays in the room's own session under a system activity
// (#3066). The shared view lists that child as independent, without an
// activity name, next to the children of the activity running in the room.
func TestOpenRooms_MarkIndependentStaysApartFromTheActivity(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomIndependent", "Leader")
	hall := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomIndependentHall")

	football := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomIndependentFussball")
	roomActivity := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomIndependentStay")
	_, err := tc.db.NewUpdate().
		Table("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", roomActivity.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	footballSession := testpkg.CreateTestActiveGroup(t, tc.db, football.ID, hall.ID)
	roomSession := testpkg.CreateTestActiveGroup(t, tc.db, roomActivity.ID, hall.ID)

	player := testpkg.CreateTestStudent(t, tc.db, "OpenRoomIndependent", "Spielkind", "OI1")
	visitor := testpkg.CreateTestStudent(t, tc.db, "OpenRoomIndependent", "Raumkind", "OI2")
	now := time.Now()
	testpkg.CreateTestVisit(t, tc.db, player.ID, footballSession.ID, now.Add(-40*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, visitor.ID, roomSession.ID, now.Add(-10*time.Minute), nil)

	room, found := openRooms(t, router, account.ID)[hall.Name]
	require.True(t, found)
	require.Len(t, room.Students, 2)

	byID := map[string]wireOpenRoomStudent{}
	for _, student := range room.Students {
		byID[student.StudentID] = student
	}
	playing := byID[strconv.FormatInt(player.ID, 10)]
	assert.False(t, playing.Independent, "a child in the activity takes part in it")
	assert.Equal(t, football.Name, playing.ActivityName)

	staying := byID[strconv.FormatInt(visitor.ID, 10)]
	assert.True(t, staying.Independent, "a child in the room's own session stays independently")
	assert.Empty(t, staying.ActivityName, "the system activity is not shown as an offering")
	assert.True(t, room.HasOccupyingSession,
		"the activity session still occupies the room beside independent stays")
}

// TestOpenRooms_DeviceOwnedSystemSessionIsASupervision: a kiosk owns
// Schulhof Freispiel. Children in that session take part in it; they must
// not be relabeled as independent stays in Offene Räume.
func TestOpenRooms_DeviceOwnedSystemSessionIsASupervision(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomKiosk", "Leader")
	yard := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomKioskYard")
	yardActivity := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomKioskFreispiel")
	_, err := tc.db.NewUpdate().
		Table("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", yardActivity.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	session := testpkg.CreateTestActiveGroup(t, tc.db, yardActivity.ID, yard.ID)
	device := testpkg.CreateTestDevice(t, tc.db, "OpenRoomKiosk")
	_, err = tc.db.NewUpdate().
		Table("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", session.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	child := testpkg.CreateTestStudent(t, tc.db, "OpenRoomKiosk", "Hofkind", "OK1")
	testpkg.CreateTestVisit(t, tc.db, child.ID, session.ID, time.Now().Add(-20*time.Minute), nil)

	room, found := openRooms(t, router, account.ID)[yard.Name]
	require.True(t, found)
	require.Len(t, room.Students, 1)
	assert.False(t, room.Students[0].Independent,
		"a kiosk-owned system session is a supervision, not an independent stay")
	assert.Equal(t, yardActivity.Name, room.Students[0].ActivityName)
	assert.True(t, room.HasOccupyingSession,
		"a kiosk-owned system session occupies the room")
}

// TestOpenRooms_IndependentStaysAloneDoNotOccupy: children moved into a
// released room create a device-less Offener-Raum session. That session
// must not occupy the room for Spontanes Angebot.
func TestOpenRooms_IndependentStaysAloneDoNotOccupy(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomStayOnly", "Leader")
	hall := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomStayOnlyHall")
	roomActivity := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomStayOnlyStay")
	_, err := tc.db.NewUpdate().
		Table("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", roomActivity.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	roomSession := testpkg.CreateTestActiveGroup(t, tc.db, roomActivity.ID, hall.ID)
	visitor := testpkg.CreateTestStudent(t, tc.db, "OpenRoomStayOnly", "Raumkind", "OS1")
	testpkg.CreateTestVisit(t, tc.db, visitor.ID, roomSession.ID, time.Now().Add(-10*time.Minute), nil)

	room, found := openRooms(t, router, account.ID)[hall.Name]
	require.True(t, found)
	require.Len(t, room.Students, 1)
	assert.True(t, room.Students[0].Independent)
	assert.Empty(t, room.Students[0].ActivityName)
	assert.False(t, room.HasOccupyingSession,
		"independent stays share the room and must not mark it occupied")
}

// TestOpenRooms_EmptyActivitySessionOccupies: an activity with no children
// still occupies the room. Occupancy is session-level, not inferred from
// whether every listed child is independent.
func TestOpenRooms_EmptyActivitySessionOccupies(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomEmptyOcc", "Leader")
	hall := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomEmptyOccHall")
	football := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomEmptyOccFussball")
	testpkg.CreateTestActiveGroup(t, tc.db, football.ID, hall.ID)

	room, found := openRooms(t, router, account.ID)[hall.Name]
	require.True(t, found)
	assert.Zero(t, room.StudentCount)
	assert.True(t, room.HasOccupyingSession,
		"an empty activity session still occupies the room")
}

// TestOpenRooms_StopAtTheTenantBoundary: a released room is reachable for
// every caregiver OF THAT SCHOOL. "Everyone" never crosses the tenant line.
func TestOpenRooms_StopAtTheTenantBoundary(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomIso", "Leader")
	own := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomIsoOwn")

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, tc.db, otherTenantID)
	foreign := testpkg.CreateTestOpenRoomForTenant(t, tc.db, otherTenantID, "OpenRoomIsoForeign")

	rooms := openRooms(t, router, account.ID)

	assert.Contains(t, rooms, own.Name)
	assert.NotContains(t, rooms, foreign.Name,
		"a released room of another school must never appear in this school's shared view")
}

// TestOpenRooms_WithdrawnReleaseEndsTheSharedEntryOnly: taking the release
// back removes the permanent shared room, but the stay recorded in it is
// untouched — it stays reachable through its own session.
func TestOpenRooms_WithdrawnReleaseEndsTheSharedEntryOnly(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomWithdraw", "Leader")
	room := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomWithdrawRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomWithdrawActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, tc.db, "OpenRoomWithdraw", "Kind", "OW1")
	testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)

	require.Contains(t, openRooms(t, router, account.ID), room.Name)

	_, err := tc.db.NewUpdate().
		Table("facilities.rooms").
		Set("is_open_room = FALSE").
		Where("id = ?", room.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	assert.NotContains(t, openRooms(t, router, account.ID), room.Name,
		"the permanent shared availability ends with the release")

	envelope := dashboardExec(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
	require.Len(t, envelope.Data.Visits, 1,
		"the stay recorded in that room is untouched and stays reachable through its session")
}

type wireOpenRoomBlock struct {
	InstanceID     string `json:"instance_id"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	IsUserAssigned bool   `json:"is_user_assigned"`
	CanOperate     bool   `json:"can_operate"`
}

type wireOpenRoomSession struct {
	ActiveGroupID     string             `json:"active_group_id"`
	Title             string             `json:"title"`
	Independent       bool               `json:"independent"`
	IsUserSupervising bool               `json:"is_user_supervising"`
	CanAssign         bool               `json:"can_assign"`
	StudentCount      int                `json:"student_count"`
	Block             *wireOpenRoomBlock `json:"block"`
}

type openRoomSessionsEnvelope struct {
	Data struct {
		OpenRooms []struct {
			Name         string                `json:"name"`
			StudentCount int                   `json:"student_count"`
			Sessions     []wireOpenRoomSession `json:"sessions"`
		} `json:"open_rooms"`
	} `json:"data"`
}

// openRoomSessions reads one released room's sessions off the real endpoint
// for a teacher or an admin account, keyed by active group id.
func openRoomSessions(t *testing.T, router testutil.Router, accountID int64, admin bool, perms []string, roomName string) (int, map[string]wireOpenRoomSession) {
	t.Helper()

	req := testutil.NewRequest("GET", "/active/supervision-dashboard", nil)
	claims := testutil.TeacherTestClaims(int(accountID))
	if admin {
		claims = testutil.AdminTestClaims(int(accountID))
	}
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, perms)
	require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var envelope openRoomSessionsEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	for _, room := range envelope.Data.OpenRooms {
		if room.Name != roomName {
			continue
		}
		byID := make(map[string]wireOpenRoomSession, len(room.Sessions))
		for _, session := range room.Sessions {
			byID[session.ActiveGroupID] = session
		}
		return room.StudentCount, byID
	}
	t.Fatalf("released room %q missing from the dashboard", roomName)
	return 0, nil
}

// startBlockIn runs a timetable block in the room the way a planner start
// leaves it: the template's live session and today's active instance that
// bridges to it.
func startBlockIn(t *testing.T, db *testpkg.DB, roomID int64, title, start, end string) (activeGroupID, instanceID int64) {
	t.Helper()

	template := testpkg.CreateTestActivityGroup(t, db, title)
	session := testpkg.CreateTestActiveGroup(t, db, template.ID, roomID)
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), roomID, testpkg.ActivityInstanceOpts{
		Status:          "active",
		ActivityGroupID: &template.ID,
		ActiveGroupID:   &session.ID,
		StartHHMM:       start,
		EndHHMM:         end,
		Title:           title,
	})
	return session.ID, instance.ID
}

// markSystemActivity turns a test activity into a system activity, as the
// Schulhof Freispiel and the open-room stay activity are.
func markSystemActivity(t *testing.T, db *testpkg.DB, activityID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		Table("activities.groups").
		Set("is_system = TRUE").
		Where("id = ? AND tenant_id = ?", activityID, testpkg.Tenant(t)).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// idText renders an id the way the wire carries it.
func idText(value int64) string { return strconv.FormatInt(value, 10) }

// TestOpenRooms_SessionsCarryTheBlockRoster is #3281's contract: a released
// room lists every running session with what its block roster needs — the
// instance, the plan window, whether the caller is planned on it, supervises
// it, and may operate it — so the page never cross-references a second
// source to find the caller's blocks.
func TestOpenRooms_SessionsCarryTheBlockRoster(t *testing.T) {
	t.Parallel()
	tc, router := setupDashboardContext(t)

	caller, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomBlocks", "Caller")
	colleague, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomBlocks", "Colleague")
	yard := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomBlocksYard")

	// Planned on it, not (yet) supervising: the caller has not started it.
	plannedSession, plannedInstance := startBlockIn(t, tc.db, yard.ID, "OpenRoomBlocksGT1", "13:00", "14:00")
	testpkg.CreateTestInstanceStaff(t, tc.db, plannedInstance, caller.Staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestInstanceStaff(t, tc.db, plannedInstance, colleague.Staff.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestGroupSupervisor(t, tc.db, colleague.Staff.ID, plannedSession, "supervisor")

	// Supervising without being planned: added as a further supervisor.
	supervisedSession, supervisedInstance := startBlockIn(t, tc.db, yard.ID, "OpenRoomBlocksGT2", "13:30", "14:30")
	testpkg.CreateTestInstanceStaff(t, tc.db, supervisedInstance, colleague.Staff.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestGroupSupervisor(t, tc.db, caller.Staff.ID, supervisedSession, "supervisor")

	// Somebody else's block; the caller is planned on it but marked absent.
	foreignSession, foreignInstance := startBlockIn(t, tc.db, yard.ID, "OpenRoomBlocksGT3", "14:00", "15:00")
	testpkg.CreateTestInstanceStaff(t, tc.db, foreignInstance, colleague.Staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestInstanceStaff(t, tc.db, foreignInstance, caller.Staff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestGroupSupervisor(t, tc.db, colleague.Staff.ID, foreignSession, "supervisor")

	// A kiosk-owned Freispiel: a supervision without a timetable block.
	kioskActivity := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomBlocksFreispiel")
	markSystemActivity(t, tc.db, kioskActivity.ID)
	kioskSession := testpkg.CreateTestActiveGroup(t, tc.db, kioskActivity.ID, yard.ID)
	device := testpkg.CreateTestDevice(t, tc.db, "OpenRoomBlocksKiosk")
	_, err := tc.db.NewUpdate().
		Table("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", kioskSession.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	// The room's own session: independent stays, even with a spontaneous
	// instance behind it (the web "Beaufsichtigen" start leaves exactly that).
	stayActivity := testpkg.CreateTestActivityGroup(t, tc.db, "OpenRoomBlocksStay")
	markSystemActivity(t, tc.db, stayActivity.ID)
	staySession := testpkg.CreateTestActiveGroup(t, tc.db, stayActivity.ID, yard.ID)
	testpkg.CreateTestActivityInstance(t, tc.db, testpkg.TodayDate(), yard.ID, testpkg.ActivityInstanceOpts{
		Status:          "active",
		ActivityGroupID: &stayActivity.ID,
		ActiveGroupID:   &staySession.ID,
		IsSpontaneous:   true,
		Title:           "Schulhof",
	})

	now := time.Now()
	player := testpkg.CreateTestStudent(t, tc.db, "OpenRoomBlocks", "Spielkind", "OB1")
	foreignChild := testpkg.CreateTestStudent(t, tc.db, "OpenRoomBlocks", "Fremdkind", "OB2")
	kioskChild := testpkg.CreateTestStudent(t, tc.db, "OpenRoomBlocks", "Kioskkind", "OB3")
	stayChild := testpkg.CreateTestStudent(t, tc.db, "OpenRoomBlocks", "Raumkind", "OB4")
	testpkg.CreateTestVisit(t, tc.db, player.ID, plannedSession, now.Add(-40*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, foreignChild.ID, foreignSession, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, kioskChild.ID, kioskSession.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, stayChild.ID, staySession.ID, now.Add(-10*time.Minute), nil)

	count, sessions := openRoomSessions(t, router, account.ID, false, dashboardPerms, yard.Name)

	require.Len(t, sessions, 5, "every running session in the room is listed")
	assert.Equal(t, 4, count, "the room count is the sum of its sessions")

	planned := sessions[idText(plannedSession)]
	require.NotNil(t, planned.Block)
	assert.Equal(t, idText(plannedInstance), planned.Block.InstanceID)
	assert.Equal(t, "OpenRoomBlocksGT1", planned.Title)
	assert.Equal(t, "13:00", planned.Block.StartTime)
	assert.Equal(t, "14:00", planned.Block.EndTime)
	assert.True(t, planned.Block.IsUserAssigned, "the caller is planned on this block")
	assert.True(t, planned.Block.CanOperate, "a planned caller operates the block before starting it")
	assert.False(t, planned.IsUserSupervising)
	assert.False(t, planned.CanAssign, "adding supervisors needs a supervision, not a plan entry")
	assert.Equal(t, 1, planned.StudentCount)

	supervised := sessions[idText(supervisedSession)]
	require.NotNil(t, supervised.Block)
	assert.False(t, supervised.Block.IsUserAssigned)
	assert.True(t, supervised.Block.CanOperate, "a supervisor operates the block")
	assert.True(t, supervised.IsUserSupervising)
	assert.True(t, supervised.CanAssign)
	assert.Zero(t, supervised.StudentCount)

	foreign := sessions[idText(foreignSession)]
	require.NotNil(t, foreign.Block)
	assert.False(t, foreign.Block.IsUserAssigned, "an absent plan entry is no assignment")
	assert.False(t, foreign.Block.CanOperate, "a block the caller neither plans nor supervises is read-only (#3167)")
	assert.False(t, foreign.IsUserSupervising)
	assert.False(t, foreign.CanAssign)
	assert.Equal(t, 1, foreign.StudentCount)

	kiosk := sessions[idText(kioskSession.ID)]
	assert.Nil(t, kiosk.Block, "a kiosk session has no timetable block")
	assert.False(t, kiosk.Independent)
	assert.Equal(t, kioskActivity.Name, kiosk.Title)
	assert.Equal(t, 1, kiosk.StudentCount)

	stay := sessions[idText(staySession.ID)]
	assert.Nil(t, stay.Block, "the room's own session is never a block, whatever started it")
	assert.True(t, stay.Independent)
	assert.Empty(t, stay.Title, "the system activity is not shown as an offering")
	assert.Equal(t, 1, stay.StudentCount)

	t.Run("an admin operates every block", func(t *testing.T) {
		_, asAdmin := openRoomSessions(t, router, account.ID, true, dashboardPerms, yard.Name)
		for _, sessionID := range []int64{plannedSession, supervisedSession, foreignSession} {
			block := asAdmin[idText(sessionID)].Block
			require.NotNil(t, block)
			assert.True(t, block.CanOperate, "session %d", sessionID)
			assert.True(t, asAdmin[idText(sessionID)].CanAssign, "session %d", sessionID)
		}
	})

	t.Run("without schedules:read the blocks stay unnamed", func(t *testing.T) {
		_, withoutSchedules := openRoomSessions(t, router, account.ID, false, []string{"groups:read", "users:read"}, yard.Name)
		require.Len(t, withoutSchedules, 5, "the sessions and their children stay visible")
		for _, session := range withoutSchedules {
			assert.Nil(t, session.Block, "session %s", session.ActiveGroupID)
		}
		assert.Equal(t, 1, withoutSchedules[idText(plannedSession)].StudentCount)
	})
}

// TestOpenRooms_BlockCostDoesNotGrowWithRoomsOrBlocks: the shared view reads
// its sessions, visits and blocks in bulk. One room with one block and three
// rooms with five blocks cost the same statements.
func TestOpenRooms_BlockCostDoesNotGrowWithRoomsOrBlocks(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupDashboardContext(t)

	caller, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OpenRoomCost", "Caller")
	addBlock := func(roomID int64, n int) {
		session, instance := startBlockIn(t, tc.db, roomID, fmt.Sprintf("OpenRoomCostBlock%d", n), "13:00", "14:00")
		testpkg.CreateTestInstanceStaff(t, tc.db, instance, caller.Staff.ID, testpkg.InstanceStaffOpts{})
		testpkg.CreateTestGroupSupervisor(t, tc.db, caller.Staff.ID, session, "supervisor")
		child := testpkg.CreateTestStudent(t, tc.db, "OpenRoomCost", fmt.Sprintf("Kind%d", n), "OC1")
		testpkg.CreateTestVisit(t, tc.db, child.ID, session, time.Now().Add(-15*time.Minute), nil)
	}

	yard := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomCostYard")
	addBlock(yard.ID, 1)

	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() int {
		counter.Reset()
		rr := dashboardExecRaw(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
		require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())
		return counter.Total()
	}
	small := run()

	hall := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomCostHall")
	gym := testpkg.CreateTestOpenRoom(t, tc.db, "OpenRoomCostGym")
	addBlock(yard.ID, 2)
	addBlock(yard.ID, 3)
	addBlock(hall.ID, 4)
	addBlock(gym.ID, 5)
	large := run()

	t.Logf("open-room cost: 1 room/1 block → %d statements, 3 rooms/5 blocks → %d statements", small, large)
	assert.Equal(t, small, large, "the shared view reads in bulk, not per room or per block")
	testpkg.AssertQueryBudget(t, "api.active.supervision_dashboard.open_room_blocks", counter.Queries())
}
