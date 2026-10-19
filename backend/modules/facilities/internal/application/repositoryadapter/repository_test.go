package repositoryadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type roomCapabilityStub struct {
	facilities.Capability
	created facilities.CreateRoom
	room    facilities.Room
	err     error
	locked  int64
}

func (s *roomCapabilityStub) ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error) {
	return []facilities.Room{s.room}, s.err
}

func (s *roomCapabilityStub) CreateRoom(_ context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	s.created = input
	return s.room, nil
}

func (s *roomCapabilityStub) FindRoomForUpdate(_ context.Context, id int64) (facilities.Room, error) {
	s.locked = id
	return s.room, s.err
}

func TestRepositoryDelegatesCreateToOwner(t *testing.T) {
	t.Parallel()
	roomID := time.Now().UnixNano()
	owner := &roomCapabilityStub{room: facilities.Room{ID: roomID, Name: "Igelraum"}}
	repository := New(owner)
	room := &facilities.Room{Name: "Igelraum"}

	require.NoError(t, repository.Create(context.Background(), room))
	require.Equal(t, "Igelraum", owner.created.Name)
	require.Equal(t, roomID, room.ID)
}

func TestRepositoryRequiresTheCapability(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "room repository adapter: Facilities capability is required", func() { New(nil) })
}

func TestRepositoryListReturnsNilOnOwnerFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	repository := New(&roomCapabilityStub{err: failure})

	rooms, err := repository.List(context.Background(), nil)

	require.Nil(t, rooms)
	require.ErrorIs(t, err, failure)
}

func TestRepositoryDelegatesUpdateLockToOwner(t *testing.T) {
	t.Parallel()
	roomID := time.Now().UnixNano()
	owner := &roomCapabilityStub{room: facilities.Room{ID: roomID, Name: "Igelraum"}}
	repository := New(owner)

	room, err := repository.FindByIDForUpdate(context.Background(), roomID)

	require.NoError(t, err)
	require.Equal(t, roomID, owner.locked)
	require.Equal(t, roomID, room.ID)
}

func TestRecordsRequireTheOwnerQuery(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "room records adapter: Facilities query is required", func() { NewRecords(nil) })
}

func TestRecordsDelegateLockToOwner(t *testing.T) {
	t.Parallel()
	roomID := time.Now().UnixNano()
	owner := &roomCapabilityStub{room: facilities.Room{ID: roomID, Name: "Igelraum"}}

	room, err := NewRecords(owner).FindByIDForUpdate(context.Background(), roomID)

	require.NoError(t, err)
	require.Equal(t, roomID, owner.locked)
	require.Equal(t, roomID, room.ID)
}
