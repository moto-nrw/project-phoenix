package facilities

import (
	"context"
	"errors"
	"testing"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type ownerStub struct {
	facilitiesModule.Capability
	created facilitiesModule.CreateRoom
	room    facilitiesModule.Room
	err     error
}

func (s *ownerStub) CreateRoom(_ context.Context, input facilitiesModule.CreateRoom) (facilitiesModule.Room, error) {
	s.created = input
	return s.room, nil
}

func (s *ownerStub) ListRooms(context.Context, facilitiesModule.RoomFilter) ([]facilitiesModule.Room, error) {
	return []facilitiesModule.Room{s.room}, s.err
}

func (s *ownerStub) FindRoom(context.Context, int64) (facilitiesModule.Room, error) {
	return s.room, s.err
}

func (s *ownerStub) FindRoomByName(context.Context, string) (facilitiesModule.Room, error) {
	return s.room, s.err
}

func (s *ownerStub) FindToiletRoom(context.Context, int64) (facilitiesModule.Room, error) {
	return s.room, s.err
}

func TestCompatibilityServiceDelegatesWritesToOwner(t *testing.T) {
	t.Parallel()

	roomID := time.Now().UnixNano()
	owner := &ownerStub{room: facilitiesModule.Room{ID: roomID, Name: "Igelraum"}}
	service := NewServiceWithConfig(ServiceConfig{
		Rooms: owner,
		Occupancy: func(_ context.Context, rooms []facilitiesModule.Room) ([]RoomWithOccupancy, error) {
			return []RoomWithOccupancy{{Room: &rooms[0]}}, nil
		},
		History: func(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error) {
			return nil, nil
		},
		ValidateDeletion: func(context.Context, int64) error { return nil },
	})
	room := &facilitiesModule.Room{Name: "Igelraum"}

	require.NoError(t, service.CreateRoom(context.Background(), room))
	require.Equal(t, "Igelraum", owner.created.Name)
	require.Equal(t, roomID, room.ID)

	listed, err := service.ListRooms(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 1)
}

func TestCompatibilityLookupsReturnNilOnOwnerFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	service := NewServiceWithConfig(ServiceConfig{
		Rooms:     &ownerStub{err: failure},
		Occupancy: func(context.Context, []facilitiesModule.Room) ([]RoomWithOccupancy, error) { return nil, nil },
		History: func(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error) {
			return nil, nil
		},
		ValidateDeletion: func(context.Context, int64) error { return nil },
	})

	room, err := service.GetRoom(context.Background(), time.Now().UnixNano())
	require.Nil(t, room)
	require.ErrorIs(t, err, failure)
	room, err = service.FindRoomByName(context.Background(), "Igelraum")
	require.Nil(t, room)
	require.ErrorIs(t, err, failure)
	room, err = service.FindToiletRoom(context.Background(), 0)
	require.Nil(t, room)
	require.ErrorIs(t, err, failure)
}

func TestCompatibilityListsReturnNilOnOwnerFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	service := NewServiceWithConfig(ServiceConfig{
		Rooms:     &ownerStub{err: failure},
		Occupancy: func(context.Context, []facilitiesModule.Room) ([]RoomWithOccupancy, error) { return nil, nil },
		History: func(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error) {
			return nil, nil
		},
		ValidateDeletion: func(context.Context, int64) error { return nil },
	})

	rooms, err := service.FindRoomsByCategory(context.Background(), "Gruppenraum")
	require.Nil(t, rooms)
	require.ErrorIs(t, err, failure)
	rooms, err = service.GetAvailableRooms(context.Background(), 10)
	require.Nil(t, rooms)
	require.ErrorIs(t, err, failure)
}

type roomServiceStub struct {
	Service
	toilet   *facilitiesModule.Room
	schulhof *facilitiesModule.Room
	created  *facilitiesModule.Room
}

func (s *roomServiceStub) FindToiletRoom(context.Context, int64) (*facilitiesModule.Room, error) {
	if s.toilet == nil {
		return nil, ErrRoomNotFound
	}
	return s.toilet, nil
}

func (s *roomServiceStub) FindRoomByName(context.Context, string) (*facilitiesModule.Room, error) {
	if s.schulhof == nil {
		return nil, ErrRoomNotFound
	}
	return s.schulhof, nil
}

func (s *roomServiceStub) CreateRoom(_ context.Context, room *facilitiesModule.Room) error {
	room.ID = 41
	s.created = room
	if facilitiesModule.IsWCRoomName(room.Name) {
		s.toilet = room
	} else {
		s.schulhof = room
	}
	return nil
}

type activityCatalogStub struct {
	activities []SystemActivity
	categories []SystemCategory
}

func (s *activityCatalogStub) ListActivities(context.Context, string) ([]SystemActivity, error) {
	return s.activities, nil
}

func (s *activityCatalogStub) CreateActivity(_ context.Context, activity SystemActivity) (SystemActivity, error) {
	activity.ID = 71
	s.activities = append(s.activities, activity)
	return activity, nil
}

func (s *activityCatalogStub) ListCategories(context.Context) ([]SystemCategory, error) {
	return s.categories, nil
}

func (s *activityCatalogStub) CreateCategory(_ context.Context, category SystemCategory) (SystemCategory, error) {
	category.ID = 61
	s.categories = append(s.categories, category)
	return category, nil
}

func TestWCServiceCreatesOwnerRoomAndActivity(t *testing.T) {
	t.Parallel()

	rooms, activities := &roomServiceStub{}, &activityCatalogStub{}
	service := NewWCService(rooms, activities, nil)

	created, err := service.EnsureInfrastructure(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(71), created.ID)
	require.Equal(t, facilitiesModule.WCRoomName, rooms.created.Name)
	require.True(t, rooms.created.IsSystem)
	require.Equal(t, int64(41), *created.PlannedRoomID)
}

type openGroupCatalogStub struct {
	groups      []OpenGroup
	supervisors []OpenGroupSupervisor
	visits      []OpenGroupVisit
}

func (s openGroupCatalogStub) ListByRoom(context.Context, int64) ([]OpenGroup, error) {
	return s.groups, nil
}

func (s openGroupCatalogStub) ListSupervisors(context.Context, []int64) ([]OpenGroupSupervisor, error) {
	return s.supervisors, nil
}

func (s openGroupCatalogStub) ListVisits(context.Context, int64) ([]OpenGroupVisit, error) {
	return s.visits, nil
}

func TestSchulhofStatusPrefersCallersSupervisedGroup(t *testing.T) {
	t.Parallel()

	roomID := int64(41)
	rooms := &roomServiceStub{schulhof: &facilitiesModule.Room{ID: roomID, Name: facilitiesModule.SchulhofRoomName, IsSystem: true}}
	activities := &activityCatalogStub{activities: []SystemActivity{{ID: 71, Name: facilitiesModule.SchulhofActivityName, PlannedRoomID: &roomID, IsSystem: true}}}
	now := time.Now()
	groups := openGroupCatalogStub{
		groups:      []OpenGroup{{ID: 81, StartTime: now.Add(-time.Hour), IsToday: true}, {ID: 82, StartTime: now, IsToday: true}},
		supervisors: []OpenGroupSupervisor{{ID: 91, GroupID: 81, StaffID: 5, FirstName: "Ada", LastName: "Lovelace"}},
		visits:      []OpenGroupVisit{{}, {ExitTime: pointer(now)}},
	}
	service := NewSchulhofService(rooms, activities, groups, nil)

	status, err := service.GetSchulhofStatus(context.Background(), 5)

	require.NoError(t, err)
	require.Equal(t, int64(81), *status.ActiveGroupID)
	require.True(t, status.IsUserSupervising)
	require.Equal(t, 1, status.StudentCount)
	require.Equal(t, "Ada Lovelace", status.Supervisors[0].Name)
}
