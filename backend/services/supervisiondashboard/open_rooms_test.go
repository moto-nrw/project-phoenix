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

func sessionIndex(sessions []RunningSession) map[int64]RunningSession {
	byID := make(map[int64]RunningSession, len(sessions))
	for _, session := range sessions {
		byID[session.ActiveGroupID] = session
	}
	return byID
}

func TestOpenRoomsIncludeReleasedRoomsWithNothingRunning(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"}}
	result := assembleOpenRooms(rooms, nil, nil, nil, nil)

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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), visits, nil)

	require.Len(t, result, 1, "two sessions in one room stay one room")
	room := result[0]
	assert.Equal(t, []int64{10, 11}, room.ActiveGroupIDs, "both sessions feed the view, oldest first")
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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), visits, nil)

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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), visits, nil)

	require.Len(t, result[0].Students, 1)
	assert.Empty(t, result[0].Students[0].ActivityName)
}

func TestSupervisionFlagReportsOnlyTheCallersOwnSupervision(t *testing.T) {
	t.Parallel()

	rooms := []ReleasedRoom{{ID: 1, Name: "Turnhalle"}, {ID: 2, Name: "Werkraum"}}
	sessions := []RunningSession{
		{ActiveGroupID: 10, RoomID: 1, StartTime: at(0), SupervisorStaffIDs: []int64{7}},
		{ActiveGroupID: 11, RoomID: 2, StartTime: at(0), SupervisorStaffIDs: []int64{9}},
	}
	caller := int64(7)

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), nil, &caller)

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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), nil, nil)

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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), visits, nil)

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

	result := assembleOpenRooms(rooms, sessions, sessionIndex(sessions), visits, nil)

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

func loadWith(t *testing.T, source *countingOpenRoomSource) *Projection {
	t.Helper()
	svc := &service{deps: Dependencies{
		OpenRoomDirectory: source,
		OpenRoomSessions:  source,
		OpenRoomVisits:    source,
	}}
	projection := emptyProjection()
	require.NoError(t, svc.loadOpenRooms(t.Context(), projection, nil))
	return projection
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

	smallProjection := loadWith(t, small)
	largeProjection := loadWith(t, large)

	require.Len(t, smallProjection.OpenRooms, 1)
	require.Len(t, largeProjection.OpenRooms, 4)

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
	projection := loadWith(t, source)

	assert.Empty(t, projection.OpenRooms)
	assert.Equal(t, 1, source.roomCalls)
	assert.Zero(t, source.sessionCalls)
	assert.Zero(t, source.visitCalls)
}

// TestOpenRoomLoadSkipsVisitsWhenNothingRuns covers the middle case: released
// rooms exist but are all empty, so there are no sessions to ask visits for.
func TestOpenRoomLoadSkipsVisitsWhenNothingRuns(t *testing.T) {
	t.Parallel()

	source := &countingOpenRoomSource{rooms: []ReleasedRoom{{ID: 1, Name: "Turnhalle"}}}
	projection := loadWith(t, source)

	require.Len(t, projection.OpenRooms, 1, "the empty room is still reachable")
	assert.Equal(t, 1, source.sessionCalls)
	assert.Zero(t, source.visitCalls)
}
