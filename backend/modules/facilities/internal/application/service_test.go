package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
	"github.com/stretchr/testify/require"
)

type deletionStore struct {
	ports.Store
	steps *[]string
}

type failingReadStore struct {
	ports.Store
	err error
}

func (s failingReadStore) FindByID(context.Context, int64, string) (domain.Room, bool, domain.OperationStats, error) {
	return domain.Room{}, false, domain.OperationStats{Queries: 1}, s.err
}

func (s deletionStore) FindByID(context.Context, int64, string) (domain.Room, bool, domain.OperationStats, error) {
	*s.steps = append(*s.steps, "room")
	return domain.Room{Name: "Igelraum"}, true, domain.OperationStats{}, nil
}

func (s deletionStore) Delete(context.Context, int64) (domain.OperationStats, error) {
	*s.steps = append(*s.steps, "delete")
	return domain.OperationStats{}, nil
}

type deletionTransaction struct{ ports.Transaction }

func (deletionTransaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

func TestDeleteTakesRecurrenceLockBeforeRoomLock(t *testing.T) {
	t.Parallel()
	steps := []string{}
	service := New(
		deletionStore{steps: &steps}, deletionTransaction{},
		func(context.Context) error { steps = append(steps, "recurrence"); return nil },
		func(context.Context, int64) error { steps = append(steps, "guard"); return nil },
		func(ports.Observation) {},
	)

	err := service.Delete(context.Background(), time.Now().UnixNano())

	require.NoError(t, err)
	require.Equal(t, []string{"recurrence", "room", "guard", "delete"}, steps)
}

func TestFindByIDPreservesStoreFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	var observed ports.Observation
	service := New(
		failingReadStore{err: failure}, deletionTransaction{},
		func(context.Context) error { return nil }, func(context.Context, int64) error { return nil },
		func(value ports.Observation) { observed = value },
	)

	room, err := service.FindByID(context.Background(), time.Now().UnixNano())

	require.Empty(t, room)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, observed.Err, failure)
	require.EqualValues(t, 1, observed.Stats.Queries)
	require.Zero(t, observed.Stats.Rows)
}
