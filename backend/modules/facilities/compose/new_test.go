package compose

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func (l *observationLog) operations() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	names := make([]string, 0, len(l.seen))
	for _, observation := range l.seen {
		names = append(names, observation.Operation)
	}
	return names
}

func buildModule(t *testing.T, db *bun.DB, observe func(Observation)) *facilities.Module {
	t.Helper()
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
}

func TestModuleReadsRoomsOnTheAmbientTenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)

	igel := testpkg.CreateTestRoom(t, db, "Igelraum")
	fuchs := testpkg.CreateTestRoom(t, db, "Fuchsbau")

	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		found, err := module.FindRoom(ctx, igel.ID)
		require.NoError(t, err)
		assert.Equal(t, igel.Name, found.Name)
		assert.Equal(t, igel.Building, found.Building)
		assert.Equal(t, igel.Capacity, found.Capacity)
		assert.Equal(t, tenantID, found.TenantID)

		listed, err := module.ListRoomsByID(ctx, []int64{igel.ID, fuchs.ID, igel.ID + fuchs.ID + 1})
		require.NoError(t, err)
		require.Len(t, listed, 2, "missing IDs are absent, not an error")
		assert.Equal(t, fuchs.Name, listed[0].Name, "sorted by name")
		assert.Equal(t, igel.Name, listed[1].Name)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"find_room", "list_rooms_by_id"}, log.operations())
}

func TestModuleReportsMissingRoomWithStableError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	existing := testpkg.CreateTestRoom(t, db, "Igelraum")

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := module.FindRoom(ctx, existing.ID+1_000_000)
		return err
	})
	require.ErrorIs(t, err, facilities.ErrRoomNotFound)
	require.Len(t, log.seen, 1)
	assert.ErrorIs(t, log.seen[0].Err, facilities.ErrRoomNotFound, "observation carries the public error")
}

func TestModuleHidesRoomsOfOtherTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})

	hostTenant := testpkg.Tenant(t)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	own := testpkg.CreateTestRoom(t, db, "Igelraum")
	foreign := testpkg.CreateTestRoomForTenant(t, db, otherTenant, "Fuchsbau")

	err := testpkg.WithTenantTx(t, context.Background(), db, hostTenant, func(ctx context.Context, _ bun.Tx) error {
		listed, err := module.ListRoomsByID(ctx, []int64{own.ID, foreign.ID})
		require.NoError(t, err)
		require.Len(t, listed, 1, "RLS keeps the other tenant's room invisible")
		assert.Equal(t, own.ID, listed[0].ID)

		_, err = module.FindRoom(ctx, foreign.ID)
		require.ErrorIs(t, err, facilities.ErrRoomNotFound)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleFallsBackToTheSharedConnectionWithoutTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	room := testpkg.CreateTestRoom(t, db, "Igelraum")

	found, err := module.FindRoom(testpkg.Ctx(t), room.ID)
	require.NoError(t, err)
	assert.Equal(t, room.Name, found.Name)
}

func TestModuleRejectsSharedConnectionWithoutTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	room := testpkg.CreateTestRoom(t, db, "Igelraum")

	_, err := module.FindRoom(context.Background(), room.ID)
	require.ErrorContains(t, err, "tenant is required")
}

func TestModuleUsesAmbientTransactionRLSWithoutTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	tenantID := testpkg.Tenant(t)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")

	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(_ context.Context, tx bun.Tx) error {
		found, err := module.FindRoom(tenant.WithTransactionForTest(context.Background(), tx), room.ID)
		require.NoError(t, err)
		assert.Equal(t, room.ID, found.ID)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleFiltersSharedConnectionByTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestRoomForTenant(t, db, otherTenant, "Fuchsbau")

	listed, err := module.ListRoomsByID(testpkg.Ctx(t), []int64{room.ID, foreign.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1, "the shared connection must still enforce the caller tenant")
	assert.Equal(t, room.ID, listed[0].ID)

	_, err = module.FindRoom(testpkg.Ctx(t), foreign.ID)
	assert.ErrorIs(t, err, facilities.ErrRoomNotFound)
}

func TestModuleKeepsLockedRoomsUntilTenantTransactionEnds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	tenantID := testpkg.Tenant(t)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")

	locked := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			_, err := module.LockRoomsByID(ctx, []int64{room.ID})
			if err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()

	select {
	case <-locked:
	case <-time.After(2 * time.Second):
		t.Fatal("room lock was not acquired")
	}

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.NewDelete().TableExpr("facilities.rooms").Where("id = ?", room.ID).Exec(ctx)
			return err
		})
	}()

	select {
	case err := <-deleteDone:
		require.NoError(t, err, "room deletion must wait for the restore lock")
		t.Fatal("room deletion did not wait for the restore lock")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-deleteDone)
}
