package compose

import (
	"context"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
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

func buildModule(t *testing.T, db *bun.DB, observe func(Observation)) *schoolstructure.Module {
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

func TestModuleReadsGroupsOnTheAmbientTenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)

	igel := testpkg.CreateTestEducationGroup(t, db, "Igel")
	fuchs := testpkg.CreateTestEducationGroup(t, db, "Fuchs")

	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		found, err := module.FindGroup(ctx, igel.ID)
		require.NoError(t, err)
		assert.Equal(t, igel.Name, found.Name)
		assert.Equal(t, tenantID, found.TenantID)

		listed, err := module.ListGroupsByID(ctx, []int64{igel.ID, fuchs.ID, igel.ID + fuchs.ID + 1})
		require.NoError(t, err)
		require.Len(t, listed, 2, "missing IDs are absent, not an error")
		assert.Equal(t, fuchs.Name, listed[0].Name, "sorted by name")
		assert.Equal(t, igel.Name, listed[1].Name)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"find_group", "list_groups_by_id"}, log.operations())
}

func TestModuleReportsMissingGroupWithStableError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	existing := testpkg.CreateTestEducationGroup(t, db, "Igel")

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := module.FindGroup(ctx, existing.ID+1_000_000)
		return err
	})
	require.ErrorIs(t, err, schoolstructure.ErrGroupNotFound)
	require.Len(t, log.seen, 1)
	assert.ErrorIs(t, log.seen[0].Err, schoolstructure.ErrGroupNotFound, "observation carries the public error")
}

func TestModuleHidesGroupsOfOtherTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})

	hostTenant := testpkg.Tenant(t)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	own := testpkg.CreateTestEducationGroup(t, db, "Igel")
	foreign := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "Fuchs")

	err := testpkg.WithTenantTx(t, context.Background(), db, hostTenant, func(ctx context.Context, _ bun.Tx) error {
		listed, err := module.ListGroupsByID(ctx, []int64{own.ID, foreign.ID})
		require.NoError(t, err)
		require.Len(t, listed, 1, "RLS keeps the other tenant's group invisible")
		assert.Equal(t, own.ID, listed[0].ID)

		_, err = module.FindGroup(ctx, foreign.ID)
		require.ErrorIs(t, err, schoolstructure.ErrGroupNotFound)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleFallsBackToTheSharedConnectionWithoutTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	group := testpkg.CreateTestEducationGroup(t, db, "Igel")

	found, err := module.FindGroup(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.Name, found.Name)
}
