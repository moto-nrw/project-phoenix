package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModulePersistsRoomCreateAtPublicSeam(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	capacity := 24

	created, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name:     "Igelraum",
		Building: "Altbau",
		Capacity: &capacity,
	})

	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Igelraum", created.Name)
	assert.Equal(t, "Altbau", created.Building)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))
	_, err = module.FindRoom(otherCtx, created.ID)
	assert.ErrorIs(t, err, facilities.ErrRoomNotFound)
}

func TestModulePersistsRoomUpdateAtPublicSeam(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	created, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)
	capacity := 18

	updated, err := module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID:       created.ID,
		Name:     "Fuchsbau",
		Building: "Neubau",
		Capacity: &capacity,
	})

	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, created.TenantID, updated.TenantID)
	assert.Equal(t, "Fuchsbau", updated.Name)
	assert.Equal(t, "Neubau", updated.Building)
	assert.Equal(t, &capacity, updated.Capacity)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, found)
}

func TestModulePersistsRoomDeleteAtPublicSeam(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	created, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)

	err = module.DeleteRoom(ctx, created.ID)
	require.NoError(t, err)

	_, err = module.FindRoom(ctx, created.ID)
	assert.ErrorIs(t, err, facilities.ErrRoomNotFound)
}

func TestCreateRoomRejectsCaseInsensitiveDuplicatePerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)

	_, err = module.CreateRoom(ctx, facilities.CreateRoom{Name: "IGELRAUM"})
	require.ErrorIs(t, err, facilities.ErrDuplicateRoom)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))
	_, err = module.CreateRoom(otherCtx, facilities.CreateRoom{Name: "IGELRAUM"})
	require.NoError(t, err)
}

func TestSystemRoomCannotBeRenamedOrDeleted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	created, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: facilities.SchulhofRoomName, IsSystem: true,
	})
	require.NoError(t, err)

	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{ID: created.ID, Name: "Garten"})
	require.ErrorIs(t, err, facilities.ErrSystemRoomProtected)

	err = module.DeleteRoom(ctx, created.ID)
	require.ErrorIs(t, err, facilities.ErrSystemRoomProtected)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, facilities.SchulhofRoomName, found.Name)
}

func TestUpdateRoomRejectsCaseInsensitiveDuplicate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	first, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)
	second, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Fuchsbau"})
	require.NoError(t, err)

	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{ID: second.ID, Name: "IGELRAUM"})
	require.ErrorIs(t, err, facilities.ErrDuplicateRoom)

	unchanged, err := module.FindRoom(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, "Fuchsbau", unchanged.Name)
	assert.NotEqual(t, first.ID, unchanged.ID)
}

func TestRoomCommandsPreserveToiletAliasAndColorContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	legacyColor := "#112233"
	wc, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: facilities.WCRoomName, Color: &legacyColor})
	require.NoError(t, err)

	_, err = module.CreateRoom(ctx, facilities.CreateRoom{Name: facilities.WCRoomAliasName})
	require.ErrorIs(t, err, facilities.ErrDuplicateToiletRoom)

	changedColor := "#445566"
	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{ID: wc.ID, Name: wc.Name, Color: &changedColor})
	require.ErrorIs(t, err, facilities.ErrSystemRoomProtected)

	updated, err := module.UpdateRoom(ctx, facilities.UpdateRoom{ID: wc.ID, Name: wc.Name, Building: "Altbau"})
	require.NoError(t, err)
	assert.Equal(t, &legacyColor, updated.Color)
	assert.Equal(t, "Altbau", updated.Building)
}

func TestRoomCommandsRejectDuplicateColorPerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	color := "#123456"
	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum", Color: &color})
	require.NoError(t, err)

	duplicate := "#123456"
	_, err = module.CreateRoom(ctx, facilities.CreateRoom{Name: "Fuchsbau", Color: &duplicate})
	require.ErrorIs(t, err, facilities.ErrRoomColorAlreadyInUse)
}

func TestDeleteRoomRunsGuardInsideWriteTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	guardCalled := false
	module, err := New(Dependencies{
		DB: db,
		DeletionGuard: func(guardCtx context.Context, _ int64) error {
			guardCalled = true
			_, inTransaction := tenant.TransactionFromContext(guardCtx)
			assert.True(t, inTransaction)
			return facilities.ErrRoomInUse
		},
		Observe: func(Observation) {},
	})
	require.NoError(t, err)
	created, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)

	err = module.DeleteRoom(ctx, created.ID)
	require.ErrorIs(t, err, facilities.ErrRoomInUse)
	assert.True(t, guardCalled)

	_, err = module.FindRoom(ctx, created.ID)
	require.NoError(t, err, "a rejected deletion must roll back without removing the room")
}

func TestRoomQuerySupportsLegacyLookupAndCapacityContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	smallCapacity, largeCapacity := 8, 30
	groundFloor := 0
	category := "Gruppenraum"
	_, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Igelraum", Building: "Altbau", Floor: &groundFloor, Capacity: &smallCapacity, Category: &category,
	})
	require.NoError(t, err)
	large, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Fuchsbau", Building: "Neubau", Capacity: &largeCapacity, Category: &category,
	})
	require.NoError(t, err)

	found, err := module.FindRoomByName(ctx, "fUCHSBAU")
	require.NoError(t, err)
	assert.Equal(t, large.ID, found.ID)

	minimum := 20
	rooms, err := module.ListRooms(ctx, facilities.RoomFilter{MinimumCapacity: &minimum})
	require.NoError(t, err)
	require.Len(t, rooms, 1)
	assert.Equal(t, large.ID, rooms[0].ID)

	building := "altbau"
	rooms, err = module.ListRooms(ctx, facilities.RoomFilter{Building: &building, Floor: &groundFloor})
	require.NoError(t, err)
	require.Len(t, rooms, 1)
	assert.Equal(t, "Igelraum", rooms[0].Name)
}

func TestFindToiletRoomUsesExactCanonicalAliases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "wc"})
	require.NoError(t, err)

	_, err = module.FindToiletRoom(ctx, 0)
	require.ErrorIs(t, err, facilities.ErrRoomNotFound)

	canonical, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: facilities.WCRoomName, IsSystem: true})
	require.Error(t, err, "the case-insensitive duplicate-name contract rejects canonical WC while lowercase wc exists")

	_, err = module.FindToiletRoom(ctx, canonical.ID)
	require.ErrorIs(t, err, facilities.ErrRoomNotFound)
}
