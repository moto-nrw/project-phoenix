package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
	testpkg "github.com/moto-ogs/project-phoenix/test"

	"github.com/moto-ogs/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/facilities/internal/ports"
	"github.com/stretchr/testify/require"
)

type deletionStore struct {
	ports.Store
	steps *[]string
	// referenced makes the database refuse the delete, as a NO ACTION key
	// does for a room past blocks or sessions still name.
	referenced bool
	room       facilities.Room
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
	if s.room.Name == "" {
		return facilities.Room{Name: "Igelraum"}, true, domain.OperationStats{}, nil
	}
	return s.room, true, domain.OperationStats{}, nil
}

func (s deletionStore) Delete(context.Context, int64) (bool, domain.OperationStats, error) {
	*s.steps = append(*s.steps, "delete")
	return !s.referenced, domain.OperationStats{}, nil
}

func (s deletionStore) Archive(_ context.Context, id int64) (facilities.Room, domain.OperationStats, error) {
	*s.steps = append(*s.steps, "archive")
	return facilities.Room{ID: id}, domain.OperationStats{}, nil
}

type deletionTransaction struct{ ports.Transaction }

// roomUsage answers the deletion's usage questions and records each one.
type roomUsage struct {
	steps            *[]string
	active, required bool
	activeErr        error
	requiredErr      error
	references       facilities.RoomReferences
}

// newDeletionService wires the service with the usage double answering both
// the guard and the reference questions.
func newDeletionService(store ports.Store, lock func(context.Context) error, usage roomUsage, observe func(ports.Observation)) *Service {
	return New(store, deletionTransaction{}, lock, usage, usage.RoomReferences, observe)
}

func (u roomUsage) RoomReferences(context.Context, int64) (facilities.RoomReferences, error) {
	*u.steps = append(*u.steps, "references")
	return u.references, nil
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
	service := newDeletionService(deletionStore{steps: &steps},
		func(context.Context) error { steps = append(steps, "recurrence"); return nil },
		roomUsage{steps: &steps},
		func(ports.Observation) {},
	)

	_, err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

	require.NoError(t, err)
	require.Equal(t, []string{"recurrence", "room", "sessions", "offerings", "references", "delete"}, steps)
}

// A room past blocks or sessions name is archived instead of deleted, so
// the history keeps its room and nothing is nulled (#6845).
func TestDeleteArchivesARoomHistoryStillNames(t *testing.T) {
	t.Parallel()
	steps := []string{}
	service := newDeletionService(deletionStore{steps: &steps, referenced: true}, func(context.Context) error { return nil },
		roomUsage{steps: &steps, references: facilities.RoomReferences{PastBlocks: 3, HasSessionHistory: true}}, func(ports.Observation) {})

	removal, err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

	require.NoError(t, err)
	require.Equal(t, facilities.RoomRemovalArchive, removal.Outcome)
	require.Equal(t, []string{"room", "sessions", "offerings", "references", "delete", "archive"}, steps)
}

// Activities, upcoming blocks, education groups and devices still using a
// room block its removal: deleting it used to null their room silently.
func TestDeleteRefusesARoomThatIsStillPlanned(t *testing.T) {
	t.Parallel()
	for name, references := range map[string]facilities.RoomReferences{
		"activity":        {ActivityGroups: 1},
		"upcoming block":  {UpcomingBlocks: 2},
		"education group": {EducationGroups: 1},
		"device":          {Devices: 1, PastBlocks: 4},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			steps := []string{}
			service := newDeletionService(deletionStore{steps: &steps}, func(context.Context) error { return nil },
				roomUsage{steps: &steps, references: references}, func(ports.Observation) {})

			_, err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

			var planned *facilities.RoomStillPlannedError
			require.ErrorAs(t, err, &planned)
			require.ErrorIs(t, err, facilities.ErrRoomStillPlanned)
			require.Equal(t, references, planned.References)
			require.Equal(t, []string{"room", "sessions", "offerings", "references"}, steps, "the room is not deleted")
		})
	}
}

// System rooms are recognized by is_system, not by their name (#6845).
func TestDeleteProtectsSystemRoomsByFlag(t *testing.T) {
	t.Parallel()
	steps := []string{}
	service := newDeletionService(deletionStore{steps: &steps, room: facilities.Room{Name: "Pausenhof", IsSystem: true}},
		func(context.Context) error { return nil }, roomUsage{steps: &steps}, func(ports.Observation) {})

	_, err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

	require.ErrorIs(t, err, facilities.ErrSystemRoomProtected)
	require.Equal(t, []string{"room"}, steps)
}

func TestRoomRemovalImpactNamesTheOutcome(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		usage   roomUsage
		outcome facilities.RoomRemovalOutcome
	}{
		"unused":          {usage: roomUsage{}, outcome: facilities.RoomRemovalDelete},
		"history only":    {usage: roomUsage{references: facilities.RoomReferences{PastBlocks: 1}}, outcome: facilities.RoomRemovalArchive},
		"running session": {usage: roomUsage{active: true}, outcome: facilities.RoomRemovalBlocked},
		"care offering":   {usage: roomUsage{required: true}, outcome: facilities.RoomRemovalBlocked},
		"device":          {usage: roomUsage{references: facilities.RoomReferences{Devices: 1}}, outcome: facilities.RoomRemovalBlocked},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			steps := []string{}
			usage := tc.usage
			usage.steps = &steps
			service := newDeletionService(deletionStore{steps: &steps}, func(context.Context) error { return nil }, usage, func(ports.Observation) {})

			impact, err := service.RoomRemovalImpact(testpkg.TenantContext(42), 7)

			require.NoError(t, err)
			require.Equal(t, tc.outcome, impact.Outcome)
			require.NotContains(t, steps, "delete", "the preview changes nothing")
		})
	}
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
			service := newDeletionService(deletionStore{steps: &steps}, func(context.Context) error { return nil }, usage, func(ports.Observation) {})

			_, err := service.DeleteRoom(testpkg.TenantContext(42), time.Now().UnixNano())

			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.steps, steps, "the room is not deleted")
		})
	}
}

func TestFindByIDPreservesStoreFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	var observed ports.Observation
	service := newDeletionService(failingReadStore{err: failure},
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
