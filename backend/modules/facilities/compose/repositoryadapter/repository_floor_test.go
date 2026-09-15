package repositoryadapter

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type floorFilterCapabilityStub struct {
	facilities.Capability
	filter facilities.RoomFilter
}

func (s *floorFilterCapabilityStub) ListRooms(_ context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
	s.filter = filter
	return nil, nil
}

func TestRepositoryPreservesFloorListFilter(t *testing.T) {
	t.Parallel()

	owner := &floorFilterCapabilityStub{}
	repository := New()
	repository.Bind(owner)

	_, err := repository.List(context.Background(), map[string]any{"floor": 0})

	require.NoError(t, err)
	require.NotNil(t, owner.filter.Floor)
	require.Zero(t, *owner.filter.Floor)
}
