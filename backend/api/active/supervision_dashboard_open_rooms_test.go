package active_test

// The shared open-room view over its real HTTP surface (#3065).
//
// The promises under test are the ones a caregiver actually depends on: a
// released room is reachable while empty and while somebody else supervises
// it, everything running in it converges into one list with each child once,
// and none of that reaches across the tenant boundary.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
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
}

type wireOpenRoom struct {
	RoomID            string                `json:"room_id"`
	Name              string                `json:"name"`
	IsUserSupervising bool                  `json:"is_user_supervising"`
	ActiveGroupIDs    []string              `json:"active_group_ids"`
	StudentCount      int                   `json:"student_count"`
	Students          []wireOpenRoomStudent `json:"students"`
}

type openRoomEnvelope struct {
	Data struct {
		OpenRooms []wireOpenRoom `json:"open_rooms"`
	} `json:"data"`
}

// openRooms reads the shared view off the real endpoint, keyed by room name so
// a test can name the room it means without carrying ids around.
func openRooms(t *testing.T, router chi.Router, accountID int64) map[string]wireOpenRoom {
	t.Helper()

	req := testutil.NewRequest("GET", "/active/supervision-dashboard", nil)
	rr := testutil.ExecuteWithAuthPermissions(
		t, router, req, testutil.TeacherTestClaims(int(accountID)), dashboardPerms,
	)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

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
