package repositoryadapter

import (
	"context"
	"testing"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type roomCapabilityStub struct {
	facilities.Capability
	created facilities.CreateRoom
	room    facilities.Room
}

func (s *roomCapabilityStub) CreateRoom(_ context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	s.created = input
	return s.room, nil
}

func TestRepositoryDelegatesCreateToOwner(t *testing.T) {
	t.Parallel()
	owner := &roomCapabilityStub{room: facilities.Room{ID: 7, Name: "Igelraum"}}
	repository := New()
	repository.Bind(owner)
	room := &facilitiesModels.Room{Name: "Igelraum"}

	require.NoError(t, repository.Create(context.Background(), room))
	require.Equal(t, "Igelraum", owner.created.Name)
	require.Equal(t, int64(7), room.ID)
}

func TestRepositoryFailsBeforeBinding(t *testing.T) {
	t.Parallel()
	_, err := New().FindByID(context.Background(), int64(1))
	require.EqualError(t, err, "room repository adapter: Facilities capability is not bound")
}
