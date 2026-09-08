package supervisiondashboard

// The aggregation rules of the shared open-room view (#3065), exercised
// without a database because they are pure: which rooms appear, how often a
// child appears, which offering travels with them, and what the supervision
// flag means.

import (
	"context"
	"testing"
	"time"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func at(minute int) time.Time {
	return time.Date(2026, time.September, 8, 9, minute, 0, 0, time.UTC)
}

func visit(studentID, activeGroupID int64, first, last string, entry time.Time) *activeService.VisitWithStudentDisplay {
	return &activeService.VisitWithStudentDisplay{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		FirstName:     first,
		LastName:      last,
		SchoolClass:   "1a",
		EntryTime:     entry,
	}
}

// inputs joins the three port results the way loadOpenRoomInputs does, so the
// assembly tests below state room/session/visit facts and nothing else.
func inputs(
	rooms []ReleasedRoom,
	sessions []RunningSession,
	visits []*activeService.VisitWithStudentDisplay,
) openRoomInputs {
	byID := make(map[int64]RunningSession, len(sessions))
	for _, session := range sessions {
		byID[session.ActiveGroupID] = session
	}
	return openRoomInputs{
		rooms:        rooms,
		sessions:     sessions,
		sessionByID:  byID,
		visitsByRoom: groupOpenRoomVisits(byID, visits),
	}
}

// visible is the display context of a caller who may see everything; the
// access and photo gates themselves are exercised separately.
var visible = visitDisplay{fullAccess: true, photosEnabled: true}

func TestOpenRoomsIncludeReleasedRoomsWithNothingRunning(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"}}
	result := assembleOpenRooms(inputs(rooms, nil, nil), visible, nil)

	require.Len(t, result, 2, "several released rooms are reachable at once")
	for _, room := range result {
		assert.Empty(t, room.ActiveGroupIDs, "an empty room has no sessions")
		assert.Zero(t, room.StudentCount)
		assert.NotNil(t, room.Students, "an empty room reports an empty list, not null")
		assert.False(t, room.IsUserSupervising)
	}
}

// TestOpenRoomMergesEveryRunningSessionOfThatRoom is the criterion that the
// old navigation broke: parallel sessions produced one tab each.
func TestOpenRoomMergesEveryRunningSessionOfThatRoom(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{
		{ActiveGroupID: 10, RoomID: 1, ActivityName: "Fußball", StartTime: at(0)},
		{ActiveGroupID: 11, RoomID: 1, ActivityName: "Tanzen", StartTime: at(30)},
	}
	visits := []*activeService.VisitWithStudentDisplay{
		visit(100, 10, "Ada", "Adler", at(5)),
		visit(101, 11, "Bea", "Berg", at(35)),
	}

	result := assembleOpenRooms(inputs(rooms, sessions, visits), visible, nil)

	require.Len(t, result, 1, "two sessions in one room stay one room")
	room := result[0]
	assert.Equal(t, []string{"10", "11"}, room.ActiveGroupIDs, "both sessions feed the view, oldest first")
	assert.Equal(t, 2, room.StudentCount)
	require.Len(t, room.Students, 2)
	assert.Equal(t, "Fußball", room.Students[0].ActivityName)
	assert.Equal(t, "Tanzen", room.Students[1].ActivityName)
}

// TestChildAppearsOnceEvenWithSeveralAttributableSessions covers the
// deduplication promise directly.
func TestChildAppearsOnceEvenWithSeveralAttributableSessions(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{
		{ActiveGroupID: 10, RoomID: 1, ActivityName: "Fußball", StartTime: at(0)},
		{ActiveGroupID: 11, RoomID: 1, ActivityName: "Tanzen", StartTime: at(30)},
	}
	// The same child recorded under both sessions of the room.
	visits := []*activeService.VisitWithStudentDisplay{
		visit(100, 11, "Ada", "Adler", at(35)),
		visit(100, 10, "Ada", "Adler", at(5)),
	}

	result := assembleOpenRooms(inputs(rooms, sessions, visits), visible, nil)

	require.Len(t, result, 1)
	require.Len(t, result[0].Students, 1, "the child is listed once, not once per session")
	assert.Equal(t, 1, result[0].StudentCount)
	assert.Equal(t, "Fußball", result[0].Students[0].ActivityName,
		"the arrival the room actually saw wins, not whichever session started last")
}

// TestSessionWithoutATemplateReportsNoOffering pins that an empty offering is
// only ever a fact, never invented: this slice can display an independent stay
// but does not create one (#3066 does).
func TestSessionWithoutATemplateReportsNoOffering(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{{ActiveGroupID: 10, RoomID: 1, ActivityName: "", StartTime: at(0)}}
	visits := []*activeService.VisitWithStudentDisplay{visit(100, 10, "Ada", "Adler", at(5))}

	result := assembleOpenRooms(inputs(rooms, sessions, visits), visible, nil)

	require.Len(t, result[0].Students, 1)
	assert.Empty(t, result[0].Students[0].ActivityName)
}

func TestSupervisionFlagReportsOnlyTheCallersOwnSupervision(t *testing.T) {
	t.Parallel()

	var caller, somebodyElse int64 = 7, 9
	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"}}
	sessions := []RunningSession{
		{ActiveGroupID: 10, RoomID: 1, StartTime: at(0), SupervisorStaffIDs: []int64{caller}},
		{ActiveGroupID: 11, RoomID: 2, StartTime: at(0), SupervisorStaffIDs: []int64{somebodyElse}},
	}

	result := assembleOpenRooms(inputs(rooms, sessions, nil), visible, &caller)

	byName := map[string]OpenRoom{}
	for _, room := range result {
		byName[room.Name] = room
	}
	assert.True(t, byName["Turnhalle"].IsUserSupervising, "the caller supervises this one")
	assert.False(t, byName["Werkraum"].IsUserSupervising,
		"somebody else supervising must not read as the caller's own supervision")
}

func TestSupervisionFlagIsFalseWithoutAStaffIdentity(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{
		{ActiveGroupID: 10, RoomID: 1, StartTime: at(0), SupervisorStaffIDs: []int64{7}},
	}

	result := assembleOpenRooms(inputs(rooms, sessions, nil), visible, nil)

	assert.False(t, result[0].IsUserSupervising,
		"a caller with no staff identity supervises nothing, and still sees the room")
	assert.Len(t, result, 1, "visibility does not depend on supervision")
}

// TestVisitsOfUnknownSessionsAreIgnored guards the join: a visit whose session
// is not among the released rooms' sessions must not leak into any room.
func TestVisitsOfUnknownSessionsAreIgnored(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)}}
	visits := []*activeService.VisitWithStudentDisplay{
		visit(100, 10, "Ada", "Adler", at(5)),
		visit(200, 99, "Ivo", "Irrlaeufer", at(5)), // session in an unreleased room
		nil, // defensive: a nil row never panics
	}

	result := assembleOpenRooms(inputs(rooms, sessions, visits), visible, nil)

	require.Len(t, result[0].Students, 1)
	assert.Equal(t, int64(100), result[0].Students[0].StudentID)
}

func TestOpenRoomsAndStudentsAreOrderedStably(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 2, Name: "Werkraum"}, {ID: 1, Name: "Turnhalle"}}
	sessions := []RunningSession{{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)}}
	visits := []*activeService.VisitWithStudentDisplay{
		visit(101, 10, "Zoe", "Zander", at(5)),
		visit(100, 10, "Ada", "Adler", at(5)),
	}

	result := assembleOpenRooms(inputs(rooms, sessions, visits), visible, nil)

	assert.Equal(t, []string{"Turnhalle", "Werkraum"}, []string{result[0].Name, result[1].Name})
	assert.Equal(t, "Ada Adler", result[0].Students[0].StudentName)
	assert.Equal(t, "Zoe Zander", result[0].Students[1].StudentName)
}

// countingOpenRoomSource records how often each port is called, so the test can
// state the real invariant: the shared view costs the same whether a school has
// one released room with one session or many with many.
type countingOpenRoomSource struct {
	rooms    []ReleasedRoom
	sessions []RunningSession
	visits   []*activeService.VisitWithStudentDisplay

	roomCalls, sessionCalls, visitCalls int
}

func (c *countingOpenRoomSource) ListReleasedRooms(context.Context) ([]ReleasedRoom, error) {
	c.roomCalls++
	return c.rooms, nil
}

func (c *countingOpenRoomSource) ListRunningSessionsInRooms(context.Context, []int64) ([]RunningSession, error) {
	c.sessionCalls++
	return c.sessions, nil
}

func (c *countingOpenRoomSource) GetActiveGroupVisitsWithDisplayForGroups(
	context.Context, []int64,
) ([]*activeService.VisitWithStudentDisplay, error) {
	c.visitCalls++
	return c.visits, nil
}

func loadWith(t *testing.T, source *countingOpenRoomSource) []OpenRoom {
	t.Helper()
	svc := &service{deps: Dependencies{
		OpenRoomDirectory: source,
		OpenRoomSessions:  source,
		OpenRoomVisits:    source,
	}}
	loaded, err := svc.loadOpenRoomInputs(t.Context())
	require.NoError(t, err)
	return assembleOpenRooms(loaded, visible, nil)
}

// TestOpenRoomLoadCostDoesNotGrowWithRoomsOrSessions is the query-budget
// promise in its own terms: reading per room, or per session, would make the
// cost follow the school's timetable. One call per port, always.
func TestOpenRoomLoadCostDoesNotGrowWithRoomsOrSessions(t *testing.T) {
	t.Parallel()

	small := &countingOpenRoomSource{
		rooms:    []ReleasedRoom{{ID: 1, Name: "Turnhalle"}},
		sessions: []RunningSession{{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)}},
		visits:   []*activeService.VisitWithStudentDisplay{visit(100, 10, "Ada", "Adler", at(5))},
	}
	large := &countingOpenRoomSource{
		rooms: []ReleasedRoom{
			{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"},
			{ID: 3, Name: "Schulhof"}, {ID: 4, Name: "Leseecke"},
		},
		sessions: []RunningSession{
			{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)},
			{ActiveGroupID: 11, RoomID: 1, StartTime: at(10)},
			{ActiveGroupID: 12, RoomID: 2, StartTime: at(0)},
			{ActiveGroupID: 13, RoomID: 3, StartTime: at(0)},
		},
		visits: []*activeService.VisitWithStudentDisplay{
			visit(100, 10, "Ada", "Adler", at(5)),
			visit(101, 11, "Bea", "Berg", at(15)),
			visit(102, 12, "Cem", "Celik", at(5)),
			visit(103, 13, "Dana", "Daum", at(5)),
		},
	}

	smallRooms := loadWith(t, small)
	largeRooms := loadWith(t, large)

	require.Len(t, smallRooms, 1)
	require.Len(t, largeRooms, 4)

	assert.Equal(t, 1, small.roomCalls)
	assert.Equal(t, 1, small.sessionCalls)
	assert.Equal(t, 1, small.visitCalls)
	assert.Equal(t, small.roomCalls, large.roomCalls,
		"room reads must not grow with the number of released rooms")
	assert.Equal(t, small.sessionCalls, large.sessionCalls,
		"session reads must not grow with the number of released rooms")
	assert.Equal(t, small.visitCalls, large.visitCalls,
		"visit reads must not grow with the number of running sessions")
}

// TestOpenRoomLoadSkipsSessionsAndVisitsWithoutReleasedRooms keeps the cheap
// path cheap: a school that released nothing pays one read, not three.
func TestOpenRoomLoadSkipsSessionsAndVisitsWithoutReleasedRooms(t *testing.T) {
	t.Parallel()

	source := &countingOpenRoomSource{}
	rooms := loadWith(t, source)

	assert.Empty(t, rooms)
	assert.Equal(t, 1, source.roomCalls)
	assert.Zero(t, source.sessionCalls)
	assert.Zero(t, source.visitCalls)
}

// TestOpenRoomLoadSkipsVisitsWhenNothingRuns covers the middle case: released
// rooms exist but are all empty, so there are no sessions to ask visits for.
func TestOpenRoomLoadSkipsVisitsWhenNothingRuns(t *testing.T) {
	t.Parallel()

	source := &countingOpenRoomSource{rooms: []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}}
	rooms := loadWith(t, source)

	require.Len(t, rooms, 1, "the empty room is still reachable")
	assert.Equal(t, 1, source.sessionCalls)
	assert.Zero(t, source.visitCalls)
}

// TestOpenRoomStudentsCarryTheSameDisplayAsASessionRoster is why the released
// room is not a second, poorer student shape: the page renders it with the
// roster it already has, so photo, illness and the recorded arrival/pickup
// must survive — under the same access gates.
func TestOpenRoomStudentsCarryTheSameDisplayAsASessionRoster(t *testing.T) {
	t.Parallel()

	sick := true
	photo := studentPhotoStoredURLPrefix + "portrait.jpg"
	arrival := at(0)
	row := visit(100, 10, "Ada", "Adler", at(5))
	row.OGSGroupName = "Bären"
	row.Sick = &sick
	row.PhotoPath = &photo

	in := inputs(
		[]ReleasedRoom{{ID: 1, Name: "Turnhalle"}},
		[]RunningSession{{ActiveGroupID: 10, RoomID: 1, ActivityName: "Fußball", StartTime: at(0)}},
		[]*activeService.VisitWithStudentDisplay{row},
	)
	display := visitDisplay{
		attendance:    map[int64]*activeService.AttendanceStatus{100: {CheckInTime: &arrival}},
		fullAccess:    true,
		photosEnabled: true,
	}

	student := assembleOpenRooms(in, display, nil)[0].Students[0]
	assert.Equal(t, "Ada Adler", student.StudentName)
	assert.Equal(t, "Bären", student.GroupName, "the child's OGS group travels with them")
	assert.Equal(t, "Fußball", student.ActivityName, "and so does the offering they are recorded under")
	assert.True(t, student.Sick)
	assert.Equal(t, "/api/students/100/photo/portrait.jpg", student.PhotoURL)
	assert.NotEmpty(t, student.ActualArrivalTime)
}

// TestOpenRoomStudentsObeyTheAccessGates: a released room is reachable for
// everyone, which is exactly why it must not become a way around the redaction
// that applies to the same child in a session roster.
func TestOpenRoomStudentsObeyTheAccessGates(t *testing.T) {
	t.Parallel()

	photo := studentPhotoStoredURLPrefix + "portrait.jpg"
	arrival := at(0)
	row := visit(100, 10, "Ada", "Adler", at(5))
	row.PhotoPath = &photo

	in := inputs(
		[]ReleasedRoom{{ID: 1, Name: "Turnhalle"}},
		[]RunningSession{{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)}},
		[]*activeService.VisitWithStudentDisplay{row},
	)
	restricted := visitDisplay{
		attendance:    map[int64]*activeService.AttendanceStatus{100: {CheckInTime: &arrival}},
		photosEnabled: true,
	}

	student := assembleOpenRooms(in, restricted, nil)[0].Students[0]
	assert.Empty(t, student.PhotoURL, "no photo without full access")
	assert.Nil(t, student.ActualArrivalTime, "no recorded arrival without full access")
	assert.Equal(t, "Ada Adler", student.StudentName, "the child is still visible in the room")
}

// TestOpenRoomChildrenTakePartInTheSharedPerStudentLoads is the reason the
// dedup result is kept as an input instead of being rendered immediately:
// tracking indicators and pickup/arrival times are loaded once for the union,
// so a released room's children are not silently left out of them.
func TestOpenRoomChildrenTakePartInTheSharedPerStudentLoads(t *testing.T) {
	t.Parallel()

	in := inputs(
		[]ReleasedRoom{{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"}},
		[]RunningSession{
			{ActiveGroupID: 10, RoomID: 1, StartTime: at(0)},
			{ActiveGroupID: 11, RoomID: 2, StartTime: at(0)},
		},
		[]*activeService.VisitWithStudentDisplay{
			visit(100, 10, "Ada", "Adler", at(5)),
			visit(101, 11, "Bea", "Berg", at(5)),
			// Also present in the selected session below — counted once.
			visit(102, 10, "Cem", "Celik", at(5)),
		},
	)
	selected := []*activeService.VisitWithStudentDisplay{
		visit(102, 20, "Cem", "Celik", at(1)),
		visit(103, 20, "Dana", "Daum", at(1)),
	}

	assert.Equal(t, []int64{100, 101, 102}, in.studentIDs())
	assert.Equal(t, []int64{102, 103, 100, 101}, unionStudentIDs(selected, in.studentIDs()),
		"the selected session keeps its order, released rooms add what it does not already have")
}

func TestUnionStudentIDsIsEmptyWithoutAnySource(t *testing.T) {
	t.Parallel()

	assert.Empty(t, unionStudentIDs(nil, nil))
	assert.Empty(t, openRoomInputs{}.studentIDs())
}
