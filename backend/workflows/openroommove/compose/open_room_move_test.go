package compose_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/compose"
)

// These tests run the workflow against the real Facilities and Timetable
// owners in a real database. The retained Student Presence binding is a
// recording stand-in: the presence rules behind it (source-side rights, no
// destination supervision, activity end, daily close) are covered by the
// modules/studentpresence/internal/application/presence tests with a real database.

type ensuredSession struct{ roomID, activityID int64 }

type recordedMove struct {
	sessionID  int64
	studentIDs []int64
	auth       studentpresence.StudentMoveAuthorization
}

type recordingPresence struct {
	mu       sync.Mutex
	sessions map[ensuredSession]int64
	ensured  []ensuredSession
	moves    []recordedMove
	inTx     []bool
	moveErr  error
}

func newRecordingPresence() *recordingPresence {
	return &recordingPresence{sessions: map[ensuredSession]int64{}}
}

func (p *recordingPresence) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*studentpresence.SessionDetail, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := ensuredSession{roomID: roomID, activityID: activityID}
	p.ensured = append(p.ensured, key)
	id, ok := p.sessions[key]
	if !ok {
		id = int64(1000 + len(p.sessions))
		p.sessions[key] = id
	}
	_, inTx := tenant.TransactionFromContext(ctx)
	p.inTx = append(p.inTx, inTx)
	group := &studentpresence.SessionDetail{RoomID: roomID, ActivityGroupID: &activityID}
	group.ID = id
	return group, nil
}

func (p *recordingPresence) MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, sessionID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.moveErr != nil {
		return nil, p.moveErr
	}
	p.moves = append(p.moves, recordedMove{sessionID: sessionID, studentIDs: studentIDs, auth: auth})
	_, inTx := tenant.TransactionFromContext(ctx)
	p.inTx = append(p.inTx, inTx)
	return &studentpresence.StudentMoveResult{Moved: studentIDs}, nil
}

type moveStack struct {
	rooms     *facilities.Module
	timetable timetable.Capability
	presence  *recordingPresence
	command   openroommove.Command
}

func newMoveStack(t *testing.T) *moveStack {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	rooms, err := facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:            db,
		DeletionLock:  func(context.Context) error { return nil },
		DeletionGuard: func(context.Context, int64) error { return nil },
		Observe:       func(facilitiesCompose.Observation) {},
	})
	require.NoError(t, err)
	s := &moveStack{rooms: rooms, timetable: timetabletest.New(t, db), presence: newRecordingPresence()}
	s.command, err = compose.New(compose.Dependencies{
		Rooms:      rooms,
		Activities: s.timetable,
		Presence:   s.presence,
		Observe:    func(compose.Observation) {},
	})
	require.NoError(t, err)
	return s
}

func (s *moveStack) systemActivities(t *testing.T, ctx context.Context, name string) []timetable.Group {
	t.Helper()
	isSystem := true
	groups, err := s.timetable.ListGroups(ctx, timetable.GroupFilter{Name: name, IsSystem: &isSystem})
	require.NoError(t, err)
	matching := make([]timetable.Group, 0, len(groups))
	for _, group := range groups {
		if group.Name == name {
			matching = append(matching, group)
		}
	}
	return matching
}

func staffMove(roomID int64, studentIDs ...int64) openroommove.Move {
	return openroommove.Move{RoomID: roomID, StudentIDs: studentIDs, Actor: openroommove.Actor{StaffID: 7}}
}

func TestMoveToOpenRoomProvisionsTheSharedActivityInTheTimetable(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	gym := testpkg.CreateTestOpenRoom(t, testpkg.SetupTestDB(t), "Turnhalle Bereitstellung")

	result, err := s.command.MoveToOpenRoom(ctx, staffMove(gym.ID, 11, 12))
	require.NoError(t, err)

	activities := s.systemActivities(t, ctx, timetable.OpenRoomActivityName)
	require.Len(t, activities, 1, "the shared system activity is provisioned on first use")
	activity := activities[0]
	assert.True(t, activity.IsSystem)
	assert.True(t, activity.IsOpen)
	assert.Nil(t, activity.PlannedRoomID, "the shared activity serves every released room")

	categories, err := s.timetable.ListCategories(ctx)
	require.NoError(t, err)
	var category *timetable.Category
	for i := range categories {
		if categories[i].ID == activity.CategoryID {
			category = &categories[i]
		}
	}
	require.NotNil(t, category)
	assert.Equal(t, timetable.OpenRoomCategoryName, category.Name)
	assert.True(t, category.IsSystem)

	require.Equal(t, []ensuredSession{{roomID: gym.ID, activityID: activity.ID}}, s.presence.ensured)
	require.Len(t, s.presence.moves, 1)
	assert.Equal(t, s.presence.sessions[s.presence.ensured[0]], s.presence.moves[0].sessionID)
	assert.Equal(t, []int64{11, 12}, s.presence.moves[0].studentIDs)
	assert.Equal(t, staffMove(gym.ID).Actor.StaffID, s.presence.moves[0].auth.StaffID)
	assert.False(t, s.presence.moves[0].auth.BypassResourceChecks)
	for _, inTx := range s.presence.inTx {
		assert.True(t, inTx, "the room session and the move run in the workflow's unit")
	}
	assert.Equal(t, gym.ID, result.RoomID)
	assert.Equal(t, []int64{11, 12}, result.Moved)
}

func TestMoveToOpenRoomReusesTheSharedActivityAcrossRooms(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	db := testpkg.SetupTestDB(t)
	gym := testpkg.CreateTestOpenRoom(t, db, "Turnhalle Wiederverwendung")
	craft := testpkg.CreateTestOpenRoom(t, db, "Werkraum Wiederverwendung")

	_, err := s.command.MoveToOpenRoom(ctx, staffMove(gym.ID, 1))
	require.NoError(t, err)
	_, err = s.command.MoveToOpenRoom(ctx, staffMove(craft.ID, 2))
	require.NoError(t, err)

	activities := s.systemActivities(t, ctx, timetable.OpenRoomActivityName)
	require.Len(t, activities, 1)
	require.Len(t, s.presence.ensured, 2)
	assert.Equal(t, activities[0].ID, s.presence.ensured[0].activityID)
	assert.Equal(t, activities[0].ID, s.presence.ensured[1].activityID)
	assert.NotEqual(t, s.presence.ensured[0].roomID, s.presence.ensured[1].roomID, "each released room keeps its own room session")
}

func TestConcurrentFirstMovesProvisionTheSharedActivityOnce(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	db := testpkg.SetupTestDB(t)
	rooms := []int64{
		testpkg.CreateTestOpenRoom(t, db, "Turnhalle Gleichzeitig").ID,
		testpkg.CreateTestOpenRoom(t, db, "Werkraum Gleichzeitig").ID,
		testpkg.CreateTestOpenRoom(t, db, "Aula Gleichzeitig").ID,
	}

	var wg sync.WaitGroup
	errs := make([]error, len(rooms))
	for i, roomID := range rooms {
		wg.Add(1)
		go func(i int, roomID int64) {
			defer wg.Done()
			_, errs[i] = s.command.MoveToOpenRoom(ctx, staffMove(roomID, int64(i+1)))
		}(i, roomID)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	assert.Len(t, s.systemActivities(t, ctx, timetable.OpenRoomActivityName), 1,
		"concurrent first moves into different rooms must not provision the activity twice")
}

func TestMoveToOpenRoomRejectsAnUnreleasedRoomWithoutWriting(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	ordinary := testpkg.CreateTestRoom(t, testpkg.SetupTestDB(t), "Gruppenraum Nicht Freigegeben")

	_, err := s.command.MoveToOpenRoom(ctx, staffMove(ordinary.ID, 1))

	require.ErrorIs(t, err, openroommove.ErrRoomNotReleased)
	assert.Empty(t, s.systemActivities(t, ctx, timetable.OpenRoomActivityName))
	assert.Empty(t, s.presence.ensured)
	assert.Empty(t, s.presence.moves)
}

func TestMoveToOpenRoomStopsAtTheTenantBoundary(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	db := testpkg.SetupTestDB(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	foreign := testpkg.CreateTestOpenRoomForTenant(t, db, otherTenantID, "Turnhalle Andere Schule")

	_, err := s.command.MoveToOpenRoom(ctx, staffMove(foreign.ID, 1))

	require.ErrorIs(t, err, openroommove.ErrRoomNotFound,
		"a released room of another school must not be reachable")
	assert.Empty(t, s.presence.ensured)
	assert.Empty(t, s.presence.moves)
}

func TestMoveToOpenRoomRollsBackProvisioningWhenTheMoveFails(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)
	gym := testpkg.CreateTestOpenRoom(t, testpkg.SetupTestDB(t), "Turnhalle Rücknahme")
	moveErr := errors.New("student move forbidden")
	s.presence.moveErr = moveErr

	_, err := s.command.MoveToOpenRoom(ctx, staffMove(gym.ID, 1))

	require.ErrorIs(t, err, moveErr)
	assert.Empty(t, s.systemActivities(t, ctx, timetable.OpenRoomActivityName),
		"a failed move must not leave a provisioned activity behind")
}

func TestMoveToOpenRoomKeepsTheSchulhofOnItsOwnActivity(t *testing.T) {
	t.Parallel()
	s := newMoveStack(t)
	ctx := testpkg.Ctx(t)

	capacity := facilities.SchulhofRoomCapacity
	schulhof, err := s.rooms.CreateRoom(ctx, facilities.CreateRoom{
		Name: facilities.SchulhofRoomName, Capacity: &capacity, IsSystem: true, IsOpenRoom: true,
	})
	require.NoError(t, err)
	category, err := s.timetable.CreateCategory(ctx, timetable.CreateCategory{
		Name: facilities.SchulhofCategoryName, Color: facilities.SchulhofColor, IsSystem: true,
	})
	require.NoError(t, err)
	yardActivity, err := s.timetable.CreateGroup(ctx, timetable.GroupInput{
		Name: facilities.SchulhofActivityName, MaxParticipants: facilities.SchulhofMaxParticipants,
		IsOpen: true, CategoryID: category.ID, PlannedRoomID: &schulhof.ID, IsSystem: true,
	})
	require.NoError(t, err)

	_, err = s.command.MoveToOpenRoom(ctx, staffMove(schulhof.ID, 1))
	require.NoError(t, err)

	require.Len(t, s.presence.ensured, 1)
	assert.Equal(t, yardActivity.ID, s.presence.ensured[0].activityID,
		"phone moves share the Schulhof kiosk journey's room session")
	assert.Empty(t, s.systemActivities(t, ctx, timetable.OpenRoomActivityName),
		"the Schulhof does not need the shared activity")
}
