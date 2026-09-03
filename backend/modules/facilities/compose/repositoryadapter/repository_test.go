package repositoryadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type roomCapabilityStub struct {
	facilities.Capability
	created facilities.CreateRoom
	room    facilities.Room
	err     error
}

func (s *roomCapabilityStub) ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error) {
	return []facilities.Room{s.room}, s.err
}

func (s *roomCapabilityStub) CreateRoom(_ context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	s.created = input
	return s.room, nil
}

func TestRepositoryDelegatesCreateToOwner(t *testing.T) {
	t.Parallel()
	roomID := time.Now().UnixNano()
	owner := &roomCapabilityStub{room: facilities.Room{ID: roomID, Name: "Igelraum"}}
	repository := New()
	repository.Bind(owner)
	room := &facilitiesModels.Room{Name: "Igelraum"}

	require.NoError(t, repository.Create(context.Background(), room))
	require.Equal(t, "Igelraum", owner.created.Name)
	require.Equal(t, roomID, room.ID)
}

func TestRepositoryFailsBeforeBinding(t *testing.T) {
	t.Parallel()
	_, err := New().FindByID(context.Background(), time.Now().UnixNano())
	require.EqualError(t, err, "room repository adapter: Facilities capability is not bound")
}

func TestRepositoryListReturnsNilOnOwnerFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	repository := New()
	repository.Bind(&roomCapabilityStub{err: failure})

	rooms, err := repository.List(context.Background(), nil)

	require.Nil(t, rooms)
	require.ErrorIs(t, err, failure)
}
