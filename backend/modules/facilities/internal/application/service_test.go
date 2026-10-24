package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"

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

func (s failingReadStore) FindByID(context.Context, int64, string) (facilities.Room, bool, domain.OperationStats, error) {
	return facilities.Room{}, false, domain.OperationStats{Queries: 1}, s.err
}

func (s deletionStore) FindByID(context.Context, int64, string) (facilities.Room, bool, domain.OperationStats, error) {
	*s.steps = append(*s.steps, "room")
	return facilities.Room{Name: "Igelraum"}, true, domain.OperationStats{}, nil
}

func (s deletionStore) Delete(context.Context, int64) (domain.OperationStats, error) {
	*s.steps = append(*s.steps, "delete")
	return domain.OperationStats{}, nil
}

type deletionTransaction struct{ ports.Transaction }

// roomUsage answers the deletion's usage questions and records each one.
type roomUsage struct {
	steps            *[]string
	active, required bool
	activeErr        error
	requiredErr      error
}

func (u roomUsage) HasActiveSessions(context.Context, int64) (bool, error) {
	*u.steps = append(*u.steps, "sessions")
	return u.active, u.activeErr
}

func (u roomUsage) RequiredByCareOffering(context.Context, int64) (bool, error) {
	*u.steps = append(*u.steps, "offerings")
	return u.required, u.requiredErr
}

func (deletionTransaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

func TestDeleteTakesRecurrenceLockBeforeRoomLock(t *testing.T) {
	t.Parallel()
	steps := []string{}
	service := New(
		deletionStore{steps: &steps}, deletionTransaction{},
		func(context.Context) error { steps = append(steps, "recurrence"); return nil },
		roomUsage{steps: &steps},
		func(ports.Observation) {},
	)

	err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

	require.NoError(t, err)
	require.Equal(t, []string{"recurrence", "room", "sessions", "offerings", "delete"}, steps)
}

func TestDeleteKeepsARoomSomeoneStillNeeds(t *testing.T) {
	t.Parallel()
	failure := errors.New("usage unavailable")
	cases := []struct {
		name  string
		usage roomUsage
		err   error
		steps []string
	}{
		{name: "running session", usage: roomUsage{active: true}, err: facilities.ErrRoomInUse, steps: []string{"room", "sessions"}},
		{name: "care offering", usage: roomUsage{required: true}, err: facilities.ErrRoomRequiredByOffering, steps: []string{"room", "sessions", "offerings"}},
		{name: "session read fails", usage: roomUsage{activeErr: failure}, err: failure, steps: []string{"room", "sessions"}},
		{name: "offering read fails", usage: roomUsage{requiredErr: failure}, err: failure, steps: []string{"room", "sessions", "offerings"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := []string{}
			usage := tc.usage
			usage.steps = &steps
			service := New(deletionStore{steps: &steps}, deletionTransaction{}, func(context.Context) error { return nil }, usage, func(ports.Observation) {})

			err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.steps, steps, "the room is not deleted")
		})
	}
}

func TestFindByIDPreservesStoreFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	var observed ports.Observation
	service := New(
		failingReadStore{err: failure}, deletionTransaction{},
		func(context.Context) error { return nil }, roomUsage{steps: &[]string{}},
		func(value ports.Observation) { observed = value },
	)

	room, err := service.FindRoom(testpkg.TenantContext(42), time.Now().UnixNano())

	require.Empty(t, room)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, observed.Err, failure)
	require.EqualValues(t, 1, observed.Stats.Queries)
	require.Zero(t, observed.Stats.Rows)
}
