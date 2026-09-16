package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/ports"
)

// These decision tests pin the workflow's own choices (release gate, which
// system activity a room session runs under, provisioning under the lock)
// against fakes of the owner ports. The presence rules behind the Moves port
// are covered by the integration tests with a real database.

type fakeRooms struct {
	room     facilities.Room
	err      error
	lockRoom *facilities.Room
	lockErr  error
	order    *[]string
}

func (f *fakeRooms) note(step string) {
	if f.order != nil {
		*f.order = append(*f.order, step)
	}
}

func (f *fakeRooms) FindRoom(context.Context, int64) (facilities.Room, error) {
	f.note("FindRoom")
	return f.room, f.err
}

func (f *fakeRooms) FindRoomForUpdate(context.Context, int64) (facilities.Room, error) {
	f.note("FindRoomForUpdate")
	if f.lockErr != nil {
		return facilities.Room{}, f.lockErr
	}
	if f.lockRoom != nil {
		return *f.lockRoom, nil
	}
	return f.room, f.err
}

type fakeActivities struct {
	// listResults are returned in order, one per ListGroups call; the last
	// one repeats.
	listResults  [][]timetable.Group
	listCalls    int
	categories   []timetable.Category
	createdGroup *timetable.GroupInput
	createdCat   *timetable.CreateCategory
}

func (f *fakeActivities) ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error) {
	index := f.listCalls
	if index >= len(f.listResults) {
		index = len(f.listResults) - 1
	}
	f.listCalls++
	if index < 0 {
		return nil, nil
	}
	return f.listResults[index], nil
}

func (f *fakeActivities) CreateGroup(_ context.Context, input timetable.GroupInput) (timetable.Group, error) {
	f.createdGroup = &input
	return timetable.Group{ID: 900, Name: input.Name, IsSystem: input.IsSystem, PlannedRoomID: input.PlannedRoomID}, nil
}

func (f *fakeActivities) ListCategories(context.Context) ([]timetable.Category, error) {
	return f.categories, nil
}

func (f *fakeActivities) CreateCategory(_ context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	f.createdCat = &input
	return timetable.Category{ID: 77, Name: input.Name, IsSystem: input.IsSystem}, nil
}

type fakeRoomSessions struct {
	roomID, activityID int64
	calls              int
	order              *[]string
}

func (f *fakeRoomSessions) EnsureOpenRoomSession(_ context.Context, roomID, activityID int64) (int64, error) {
	if f.order != nil {
		*f.order = append(*f.order, "EnsureOpenRoomSession")
	}
	f.roomID, f.activityID = roomID, activityID
	f.calls++
	return 555, nil
}

type fakeMoves struct {
	sessionID  int64
	studentIDs []int64
	actor      openroommove.Actor
	calls      int
	outcome    ports.MoveOutcome
	err        error
	order      *[]string
}

func (f *fakeMoves) MoveIntoRoomSession(_ context.Context, sessionID int64, studentIDs []int64, actor openroommove.Actor) (ports.MoveOutcome, error) {
	if f.order != nil {
		*f.order = append(*f.order, "MoveIntoRoomSession")
	}
	f.sessionID, f.studentIDs, f.actor = sessionID, studentIDs, actor
	f.calls++
	return f.outcome, f.err
}

type harness struct {
	rooms      *fakeRooms
	activities *fakeActivities
	sessions   *fakeRoomSessions
	moves      *fakeMoves
	lockKeys   []string
	txCalls    int
	order      []string
	observed   []ports.Observation
	command    openroommove.Command
}

func newHarness(room facilities.Room) *harness {
	h := &harness{
		rooms:      &fakeRooms{room: room},
		activities: &fakeActivities{},
		sessions:   &fakeRoomSessions{},
		moves:      &fakeMoves{},
	}
	h.rooms.order = &h.order
	h.sessions.order = &h.order
	h.moves.order = &h.order
	h.command = NewCommand(Dependencies{
		Rooms:        h.rooms,
		Activities:   h.activities,
		RoomSessions: h.sessions,
		Moves:        h.moves,
		Runtime: ports.Runtime{
			TenantID: func(context.Context) int64 { return 42 },
			WithinTenant: func(ctx context.Context, fn func(context.Context) error) error {
				h.txCalls++
				return fn(ctx)
			},
			AcquireLock: func(_ context.Context, key string) error {
				h.lockKeys = append(h.lockKeys, key)
				return nil
			},
		},
		Observe: func(o ports.Observation) { h.observed = append(h.observed, o) },
	})
	return h
}

func releasedRoom(id int64, name string) facilities.Room {
	return facilities.Room{ID: id, Name: name, IsOpenRoom: true}
}

func sharedActivity(id int64) timetable.Group {
	return timetable.Group{ID: id, Name: timetable.OpenRoomActivityName, IsSystem: true}
}

func move(roomID int64, studentIDs ...int64) openroommove.Move {
	return openroommove.Move{RoomID: roomID, StudentIDs: studentIDs, Actor: openroommove.Actor{StaffID: 9}}
}

func TestMoveToOpenRoomRejectsAnEmptyRequestBeforeTouchingOwners(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))

	for _, request := range []openroommove.Move{move(7), move(0, 1), move(7, 0, -3)} {
		if _, err := h.command.MoveToOpenRoom(context.Background(), request); !errors.Is(err, openroommove.ErrInvalidMove) {
			t.Fatalf("MoveToOpenRoom(%+v) error = %v, want ErrInvalidMove", request, err)
		}
	}
	if h.txCalls != 0 || h.moves.calls != 0 {
		t.Fatalf("an invalid request opened %d transactions and moved %d times", h.txCalls, h.moves.calls)
	}
}

func TestMoveToOpenRoomReportsAMissingRoom(t *testing.T) {
	t.Parallel()
	h := newHarness(facilities.Room{})
	h.rooms.err = facilities.ErrRoomNotFound

	_, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1))

	if !errors.Is(err, openroommove.ErrRoomNotFound) {
		t.Fatalf("error = %v, want ErrRoomNotFound", err)
	}
}

func TestMoveToOpenRoomRequiresTheRelease(t *testing.T) {
	t.Parallel()
	// Seeing a room grants nothing: a room that is not (or no longer) released
	// rejects the move before any activity, session or visit is touched.
	h := newHarness(facilities.Room{ID: 7, Name: "Turnhalle", IsOpenRoom: false})

	_, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1))

	if !errors.Is(err, openroommove.ErrRoomNotReleased) {
		t.Fatalf("error = %v, want ErrRoomNotReleased", err)
	}
	if h.activities.listCalls != 0 || h.sessions.calls != 0 || h.moves.calls != 0 {
		t.Fatalf("an unreleased room reached activities=%d sessions=%d moves=%d",
			h.activities.listCalls, h.sessions.calls, h.moves.calls)
	}
	if len(h.order) != 1 || h.order[0] != "FindRoom" {
		t.Fatalf("order = %v, want only the unlocked room read", h.order)
	}
}

func TestMoveToOpenRoomKeepsTheSchulhofOnItsOwnSystemActivity(t *testing.T) {
	t.Parallel()
	// Phone moves and the Schulhof kiosk journey must share one room session,
	// so the canonical yard keeps its dedicated activity.
	room := facilities.Room{ID: 3, Name: facilities.SchulhofRoomName, IsSystem: true, IsOpenRoom: true}
	h := newHarness(room)
	otherRoom := room.ID + 1
	h.activities.listResults = [][]timetable.Group{{
		{ID: 10, Name: facilities.SchulhofActivityName, IsSystem: true, PlannedRoomID: &otherRoom},
		{ID: 11, Name: facilities.SchulhofActivityName, IsSystem: true, PlannedRoomID: &room.ID},
	}}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(3, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if h.sessions.roomID != 3 || h.sessions.activityID != 11 {
		t.Fatalf("room session for room %d activity %d, want room 3 activity 11", h.sessions.roomID, h.sessions.activityID)
	}
	if len(h.lockKeys) != 0 || h.activities.createdGroup != nil {
		t.Fatalf("an existing activity took the provisioning lock %v or created %+v", h.lockKeys, h.activities.createdGroup)
	}
}

func TestMoveToOpenRoomUsesTheSharedActivityForOtherRooms(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	plannedRoom := h.rooms.room.ID
	h.activities.listResults = [][]timetable.Group{{
		// A same-named activity pinned to a room, a non-system copy and an
		// archived one are not the shared system activity.
		{ID: 20, Name: timetable.OpenRoomActivityName, IsSystem: true, PlannedRoomID: &plannedRoom},
		{ID: 21, Name: timetable.OpenRoomActivityName, IsSystem: false},
		{ID: 22, Name: timetable.OpenRoomActivityName, IsSystem: true, ArchivedAt: &time.Time{}},
		sharedActivity(23),
	}}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if h.sessions.activityID != 23 {
		t.Fatalf("room session activity = %d, want the shared system activity 23", h.sessions.activityID)
	}
}

func TestMoveToOpenRoomProvisionsTheSharedActivityUnderTheLock(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{nil, nil}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if len(h.lockKeys) != 1 || h.lockKeys[0] != "open-room-activity:42" {
		t.Fatalf("provisioning locks = %v, want one tenant-scoped lock", h.lockKeys)
	}
	created := h.activities.createdGroup
	if created == nil || created.Name != timetable.OpenRoomActivityName || !created.IsSystem || !created.IsOpen ||
		created.PlannedRoomID != nil || created.CategoryID != 77 {
		t.Fatalf("created activity = %+v, want the shared, open system activity in the new category", created)
	}
	category := h.activities.createdCat
	if category == nil || category.Name != timetable.OpenRoomCategoryName || !category.IsSystem {
		t.Fatalf("created category = %+v, want the system open-room category", category)
	}
	if h.sessions.activityID != 900 {
		t.Fatalf("room session activity = %d, want the provisioned activity", h.sessions.activityID)
	}
}

func TestMoveToOpenRoomDoesNotProvisionTwiceAfterAConcurrentMove(t *testing.T) {
	t.Parallel()
	// A concurrent first move into another room provisioned the activity while
	// this one waited for the lock: the lookup under the lock must find it.
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{nil, {sharedActivity(31)}}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if h.activities.createdGroup != nil || h.activities.createdCat != nil {
		t.Fatalf("created activity %+v and category %+v after the concurrent provisioning", h.activities.createdGroup, h.activities.createdCat)
	}
	if h.sessions.activityID != 31 {
		t.Fatalf("room session activity = %d, want 31", h.sessions.activityID)
	}
}

func TestMoveToOpenRoomReusesAnExistingSystemCategory(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{nil, nil}
	h.activities.categories = []timetable.Category{
		{ID: 5, Name: timetable.OpenRoomCategoryName, IsSystem: false},
		{ID: 6, Name: timetable.OpenRoomCategoryName, IsSystem: true},
	}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if h.activities.createdCat != nil {
		t.Fatalf("created category %+v although a system category exists", h.activities.createdCat)
	}
	if h.activities.createdGroup == nil || h.activities.createdGroup.CategoryID != 6 {
		t.Fatalf("created activity = %+v, want category 6", h.activities.createdGroup)
	}
}

func TestMoveToOpenRoomMovesDistinctChildrenIntoTheRoomSession(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{{sharedActivity(23)}}
	h.moves.outcome = ports.MoveOutcome{
		Moved:     []int64{1},
		Unchanged: []int64{2},
		Skipped:   []openroommove.Skipped{{StudentID: 3, Reason: "not_present"}},
	}
	request := move(7, 1, 2, 1, 3)
	request.Actor = openroommove.Actor{StaffID: 9, SchoolWideAttendanceEligible: true}

	result, err := h.command.MoveToOpenRoom(context.Background(), request)
	if err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	if h.moves.sessionID != 555 {
		t.Fatalf("moved into session %d, want the room session 555", h.moves.sessionID)
	}
	if got := h.moves.studentIDs; len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("moved children = %v, want the distinct IDs [1 2 3]", got)
	}
	if h.moves.actor != request.Actor {
		t.Fatalf("actor = %+v, want %+v", h.moves.actor, request.Actor)
	}
	if result.RoomID != 7 || result.RoomSessionID != 555 || len(result.Moved) != 1 || len(result.Unchanged) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(h.observed) != 1 || h.observed[0].Moved != 1 || h.observed[0].Err != nil {
		t.Fatalf("observations = %+v", h.observed)
	}
}

func TestMoveToOpenRoomReturnsThePresenceFailureUnchanged(t *testing.T) {
	t.Parallel()
	// The inbound adapter maps presence failures (forbidden, capacity,
	// departed child) with the ordinary move's rules, so the workflow must
	// not re-wrap them.
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{{sharedActivity(23)}}
	presenceErr := errors.New("student move forbidden")
	h.moves.err = presenceErr

	result, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1))

	if !errors.Is(err, presenceErr) {
		t.Fatalf("error = %v, want the presence error", err)
	}
	if result.RoomSessionID != 0 {
		t.Fatalf("result = %+v, want the zero result on failure", result)
	}
	if len(h.observed) != 1 || !errors.Is(h.observed[0].Err, presenceErr) {
		t.Fatalf("observations = %+v, want the failure observed", h.observed)
	}
	if got, want := h.order, []string{"FindRoom", "EnsureOpenRoomSession", "MoveIntoRoomSession"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v (the room is locked only after a successful move)", got, want)
	}
}

func TestMoveToOpenRoomLocksTheRoomAfterStudentsAndSessions(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{{sharedActivity(23)}}
	h.moves.outcome = ports.MoveOutcome{Moved: []int64{1}}

	if _, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1)); err != nil {
		t.Fatalf("MoveToOpenRoom: %v", err)
	}

	want := []string{"FindRoom", "EnsureOpenRoomSession", "MoveIntoRoomSession", "FindRoomForUpdate"}
	if !equalStrings(h.order, want) {
		t.Fatalf("order = %v, want %v so the room row is locked after the presence write", h.order, want)
	}
}

func TestMoveToOpenRoomReChecksTheReleaseAfterTheMove(t *testing.T) {
	t.Parallel()
	h := newHarness(releasedRoom(7, "Turnhalle"))
	h.activities.listResults = [][]timetable.Group{{sharedActivity(23)}}
	unreleased := releasedRoom(7, "Turnhalle")
	unreleased.IsOpenRoom = false
	h.rooms.lockRoom = &unreleased

	_, err := h.command.MoveToOpenRoom(context.Background(), move(7, 1))

	if !errors.Is(err, openroommove.ErrRoomNotReleased) {
		t.Fatalf("error = %v, want ErrRoomNotReleased after the locked re-check", err)
	}
	if h.sessions.calls != 1 || h.moves.calls != 1 {
		t.Fatalf("sessions=%d moves=%d, want both to run before the locked re-check", h.sessions.calls, h.moves.calls)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
