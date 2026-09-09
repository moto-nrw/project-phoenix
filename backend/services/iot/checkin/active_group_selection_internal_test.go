package checkin

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	activitiesService "github.com/moto-nrw/project-phoenix/services/activities"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rolloverActiveService struct {
	activeService.Service
	groups        []*active.Group
	endedGroupIDs []int64
}

func (s *rolloverActiveService) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*active.Group, error) {
	return s.groups, nil
}

func (s *rolloverActiveService) CreateActiveGroup(_ context.Context, group *active.Group) error {
	if len(s.endedGroupIDs) == 0 {
		return activeService.ErrRoomConflict
	}
	group.ID = 200
	return nil
}

func (s *rolloverActiveService) EndActiveGroupSession(_ context.Context, groupID int64) error {
	s.endedGroupIDs = append(s.endedGroupIDs, groupID)
	return nil
}

type rolloverActivityService struct {
	activitiesService.ActivityService
	group *activities.Group
}

type capacityRejectedActiveService struct {
	activeService.Service
	createdGroupID int64
	deletedGroupID int64
	roomOccupancy  int
	createCalls    int
}

func (s *capacityRejectedActiveService) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*active.Group, error) {
	return nil, nil
}

func (s *capacityRejectedActiveService) CreateActiveGroup(_ context.Context, group *active.Group) error {
	s.createCalls++
	group.ID = s.createdGroupID
	return nil
}

func (s *capacityRejectedActiveService) CountActiveVisitsByActiveGroupID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (s *capacityRejectedActiveService) CountActiveVisitsByRoomID(_ context.Context, _ int64) (int, error) {
	return s.roomOccupancy, nil
}

func (s *capacityRejectedActiveService) CreateVisit(_ context.Context, _ *studentpresence.Visit) error {
	return &activeService.RoomCapacityError{RoomID: 42, RoomName: constants.WCRoomName, CurrentOccupancy: 1, MaxCapacity: 1}
}

func (s *capacityRejectedActiveService) DeleteActiveGroup(_ context.Context, groupID int64) error {
	s.deletedGroupID = groupID
	return nil
}

type fixedRoomService struct {
	facilitiesService.Service
	room *facilities.Room
}

func (s *fixedRoomService) GetRoom(_ context.Context, _ int64) (*facilities.Room, error) {
	return s.room, nil
}

func (s *rolloverActivityService) ListGroups(_ context.Context, _ *activities.GroupListQuery) ([]*activities.Group, error) {
	return []*activities.Group{s.group}, nil
}

func TestUseExistingActiveGroupCannotSelectPreviousDaySession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	deviceID := int64(91)
	room := &facilities.Room{ID: 42, Name: "Schulhof"}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		DeviceID:  &deviceID,
		StartTime: now.AddDate(0, 0, -1),
	}
	current := &active.Group{
		Model:     base.Model{ID: 101},
		RoomID:    room.ID,
		StartTime: now.Add(-time.Hour),
	}

	groups := activeGroupsStartedToday([]*active.Group{stale, current}, now)
	selection := (&CheckinService{}).useExistingActiveGroup(context.Background(), groups, room, deviceID)

	require.NotNil(t, selection)
	assert.Equal(t, current.ID, selection.Group.ID)
	assert.False(t, selection.DeviceScoped, "yesterday's device match must not win")
}

func TestPreviousDaySpecialRoomGroupsAreEndedBeforeReplacement(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	endedAt := now.Add(-time.Hour)
	activeStub := &rolloverActiveService{}
	service := &CheckinService{active: activeStub}
	groups := []*active.Group{
		{Model: base.Model{ID: 100}, StartTime: now.AddDate(0, 0, -1)},
		{Model: base.Model{ID: 101}, StartTime: now.Add(-time.Hour)},
		{Model: base.Model{ID: 102}, StartTime: now.AddDate(0, 0, -2), EndTime: &endedAt},
		{Model: base.Model{ID: 103}, StartTime: now.AddDate(0, 0, 1)},
	}

	err := service.endPreviousDayActiveGroups(context.Background(), groups, now)

	require.NoError(t, err)
	assert.Equal(t, []int64{100}, activeStub.endedGroupIDs)
}

func TestFindOrCreateSpecialRoomGroupEndsStaleConflictBeforeCreate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    42,
		StartTime: now.AddDate(0, 0, -1),
	}
	activeStub := &rolloverActiveService{groups: []*active.Group{stale}}
	activity := &activities.Group{Model: activities.Model{ID: 300}}
	service := &CheckinService{
		active:     activeStub,
		activities: &rolloverActivityService{group: activity},
		logger:     slog.New(slog.DiscardHandler),
		now:        func() time.Time { return now },
	}
	room := &facilities.Room{ID: 42, Name: constants.WCRoomName}

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.NoError(t, err)
	require.NotNil(t, selection)
	assert.Equal(t, []int64{stale.ID}, activeStub.endedGroupIDs)
	assert.Equal(t, int64(200), selection.Group.ID)
	assert.Equal(t, activity.ID, *selection.Group.GroupID)
}

func TestFindOrCreateRegularRoomReusesPreviousDayGroup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	room := &facilities.Room{ID: 42, Name: "Klassenraum 1a"}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		StartTime: now.AddDate(0, 0, -1),
	}
	activeStub := &rolloverActiveService{groups: []*active.Group{stale}}
	service := &CheckinService{
		active: activeStub,
		logger: slog.New(slog.DiscardHandler),
		now:    func() time.Time { return now },
	}

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.NoError(t, err)
	require.NotNil(t, selection)
	assert.Empty(t, activeStub.endedGroupIDs, "regular rooms must not end sessions on scan")
	assert.Equal(t, stale.ID, selection.Group.ID)
}

func TestFindOrCreateSpecialRoomGroupEndsStaleGroupBeforeSelectingCurrent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	// Released, like every provisioned yard: the Schulhof auto-creates its own
	// kiosk session because it is an open room, not because of its name
	// (#3064), and that is what makes closing yesterday's session safe here.
	room := &facilities.Room{ID: 42, Name: constants.SchulhofRoomName, IsOpenRoom: true}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		StartTime: now.AddDate(0, 0, -1),
	}
	current := &active.Group{
		Model:     base.Model{ID: 101},
		RoomID:    room.ID,
		StartTime: now.Add(-time.Hour),
	}
	activeStub := &rolloverActiveService{groups: []*active.Group{stale, current}}
	service := &CheckinService{
		active: activeStub,
		logger: slog.New(slog.DiscardHandler),
		now:    func() time.Time { return now },
	}

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.NoError(t, err)
	require.NotNil(t, selection)
	assert.Equal(t, []int64{stale.ID}, activeStub.endedGroupIDs)
	assert.Equal(t, current.ID, selection.Group.ID)
}

func TestRejectedCapacityRemovesNewSpecialRoomGroup(t *testing.T) {
	t.Parallel()

	room := &facilities.Room{ID: 42, Name: constants.WCRoomName}
	activity := &activities.Group{Model: activities.Model{ID: 300}, Name: "WC", MaxParticipants: 999}
	activeStub := &capacityRejectedActiveService{createdGroupID: 200}
	service := &CheckinService{
		active:     activeStub,
		facilities: &fixedRoomService{room: room},
		activities: &rolloverActivityService{group: activity},
		logger:     slog.New(slog.DiscardHandler),
	}

	_, _, err := service.processCheckin(
		context.Background(),
		&users.Student{Model: base.Model{ID: 500}},
		&users.Person{FirstName: "Capacity", LastName: "Rejected"},
		room.ID,
		91,
	)

	require.ErrorIs(t, err, activeService.ErrRoomCapacityExceeded)
	assert.Equal(t, activeStub.createdGroupID, activeStub.deletedGroupID)
}

func TestFullRoomDoesNotCreateSpecialRoomGroup(t *testing.T) {
	t.Parallel()

	capacity := 1
	room := &facilities.Room{
		ID:       42,
		Name:     constants.WCRoomName,
		Capacity: &capacity,
	}
	activeStub := &capacityRejectedActiveService{createdGroupID: 200, roomOccupancy: 1}
	service := &CheckinService{
		active:     activeStub,
		facilities: &fixedRoomService{room: room},
		logger:     slog.New(slog.DiscardHandler),
	}

	_, _, err := service.processCheckin(
		context.Background(),
		&users.Student{Model: base.Model{ID: 500}},
		&users.Person{FirstName: "Capacity", LastName: "Full"},
		room.ID,
		91,
	)

	require.ErrorIs(t, err, activeService.ErrRoomCapacityExceeded)
	assert.Zero(t, activeStub.createCalls)
	assert.Zero(t, activeStub.deletedGroupID)
}

// TestDeactivatedSchulhofFollowsRegularRoomRules is the counterpart the
// name-based predicate could not express. Once an administrator removes the
// yard's release, the room loses its permanent kiosk journey and the ordinary
// room rules apply again (#3064): yesterday's session stays reusable instead of
// being closed on scan, exactly as for a Klassenraum. The release is enforced
// server-side, not by a hidden kiosk button.
func TestDeactivatedSchulhofFollowsRegularRoomRules(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	room := &facilities.Room{ID: 42, Name: constants.SchulhofRoomName, IsOpenRoom: false}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		StartTime: now.AddDate(0, 0, -1),
	}
	activeStub := &rolloverActiveService{groups: []*active.Group{stale}}
	service := &CheckinService{
		active: activeStub,
		logger: slog.New(slog.DiscardHandler),
		now:    func() time.Time { return now },
	}

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.NoError(t, err)
	require.NotNil(t, selection)
	assert.Empty(t, activeStub.endedGroupIDs,
		"a deactivated Schulhof must not end sessions on scan any more")
	assert.Equal(t, stale.ID, selection.Group.ID,
		"an existing session stays reusable under the ordinary room rules")
}

// TestDeactivatedSchulhofDoesNotAutoCreateASession pins the other half: with no
// session running, a scan into a deactivated yard must not silently provision
// one. It fails like any ordinary room without a session.
func TestDeactivatedSchulhofDoesNotAutoCreateASession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	room := &facilities.Room{ID: 42, Name: constants.SchulhofRoomName, IsOpenRoom: false}
	activeStub := &rolloverActiveService{}
	service := &CheckinService{
		active:     activeStub,
		activities: &rolloverActivityService{group: &activities.Group{Model: activities.Model{ID: 300}}},
		logger:     slog.New(slog.DiscardHandler),
		now:        func() time.Time { return now },
	}

	selection, err := service.findOrCreateActiveGroupForRoom(context.Background(), room, 91)

	require.Error(t, err, "a deactivated yard has no permanent session to fall back on")
	assert.Nil(t, selection)

	var checkinErr *CheckinError
	require.ErrorAs(t, err, &checkinErr)
	assert.Equal(t, checkinErrNoGroupsInRoom, checkinErr.Message,
		"it fails like any ordinary room without a session, not with a Schulhof-specific error")
	assert.Empty(t, activeStub.endedGroupIDs,
		"and it does not close anything on the way out")
}

// TestRoomProvisioningPredicatesFollowTheRelease covers the branch decision the
// name-based check could not express: which rooms carry the permanent kiosk
// journey. The released Schulhof and the toilets provision their own session;
// a deactivated yard and every ordinary room do not (#3064).
//
// Toilet rooms stay name-based on purpose — they are kiosk infrastructure for
// short stays, never a released destination.
func TestRoomProvisioningPredicatesFollowTheRelease(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		room           *facilities.Room
		releasedYard   bool
		selfProvisions bool
	}{
		{
			name:           "released Schulhof",
			room:           &facilities.Room{ID: 1, Name: constants.SchulhofRoomName, IsOpenRoom: true},
			releasedYard:   true,
			selfProvisions: true,
		},
		{
			name:           "deactivated Schulhof",
			room:           &facilities.Room{ID: 2, Name: constants.SchulhofRoomName},
			releasedYard:   false,
			selfProvisions: false,
		},
		{
			name:           "WC stays name-based",
			room:           &facilities.Room{ID: 3, Name: constants.WCRoomName},
			releasedYard:   false,
			selfProvisions: true,
		},
		{
			name:           "Toilette alias stays name-based",
			room:           &facilities.Room{ID: 4, Name: constants.WCRoomAliasName},
			releasedYard:   false,
			selfProvisions: true,
		},
		{
			name:           "a released ordinary room provisions nothing yet",
			room:           &facilities.Room{ID: 5, Name: "Turnhalle", IsOpenRoom: true},
			releasedYard:   false,
			selfProvisions: false,
		},
		{
			name:           "ordinary room",
			room:           &facilities.Room{ID: 6, Name: "Klassenraum 1a"},
			releasedYard:   false,
			selfProvisions: false,
		},
		{
			name:           "missing room is never special",
			room:           nil,
			releasedYard:   false,
			selfProvisions: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.releasedYard, isReleasedSchulhofRoom(tc.room))
			assert.Equal(t, tc.selfProvisions, isSelfProvisioningRoom(tc.room))
		})
	}
}
