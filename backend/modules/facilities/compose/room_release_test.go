package compose

// Persistence behavior of the room release ("offener Raum", #3064) through the
// module's public seam against a real database: what is stored, what an
// omitted opinion leaves alone, and that a release never crosses tenants.

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func releasePtr(value bool) *bool { return &value }

func TestModulePersistsRoomReleaseOnCreate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	released, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Turnhalle", IsOpenRoom: true,
	})
	require.NoError(t, err)
	assert.True(t, released.IsOpenRoom)

	ordinary, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Werkraum"})
	require.NoError(t, err)
	assert.False(t, ordinary.IsOpenRoom,
		"a room created without an opinion must be unreleased")

	// Read back through a fresh query: the value has to be in the row, not
	// only in the struct the write returned.
	found, err := module.FindRoom(ctx, released.ID)
	require.NoError(t, err)
	assert.True(t, found.IsOpenRoom)

	foundOrdinary, err := module.FindRoom(ctx, ordinary.ID)
	require.NoError(t, err)
	assert.False(t, foundOrdinary.IsOpenRoom)
}

func TestModulePersistsRoomReleaseChangesBothWays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Turnhalle"})
	require.NoError(t, err)
	require.False(t, created.IsOpenRoom)

	released, err := module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: created.ID, Name: "Turnhalle", IsOpenRoom: releasePtr(true),
	})
	require.NoError(t, err)
	assert.True(t, released.IsOpenRoom)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, found.IsOpenRoom)

	// Removing a release must be just as possible as granting one — an
	// administrator who released a room by mistake needs a way back.
	revoked, err := module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: created.ID, Name: "Turnhalle", IsOpenRoom: releasePtr(false),
	})
	require.NoError(t, err)
	assert.False(t, revoked.IsOpenRoom)

	found, err = module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, found.IsOpenRoom)
}

// The regression this whole pointer design exists for: the room form posts a
// partial payload, so an edit that says nothing about the release must not
// revoke it. Without the conditional SET column, renaming a released room or
// picking a colour would silently close the yard.
func TestUnrelatedRoomEditPreservesAStandingRelease(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Turnhalle", IsOpenRoom: true,
	})
	require.NoError(t, err)
	require.True(t, created.IsOpenRoom)

	capacity := 42
	building := "Neubau"
	updated, err := module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: created.ID, Name: "Sporthalle", Building: building, Capacity: &capacity,
	})
	require.NoError(t, err)
	assert.True(t, updated.IsOpenRoom,
		"an edit without an opinion about the release must leave it standing")
	assert.Equal(t, "Sporthalle", updated.Name, "the requested change must still apply")
	assert.Equal(t, building, updated.Building)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, found.IsOpenRoom)
}

// The mirror case: an edit without an opinion must not accidentally release an
// ordinary room either.
func TestUnrelatedRoomEditPreservesAnUnreleasedRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Werkraum"})
	require.NoError(t, err)

	updated, err := module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: created.ID, Name: "Bastelraum",
	})
	require.NoError(t, err)
	assert.False(t, updated.IsOpenRoom)
}

func TestRoomReleaseIsIsolatedPerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	released, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Turnhalle", IsOpenRoom: true,
	})
	require.NoError(t, err)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))

	// Another school can neither see the released room nor revoke its release.
	_, err = module.FindRoom(otherCtx, released.ID)
	assert.ErrorIs(t, err, facilities.ErrRoomNotFound)

	_, err = module.UpdateRoom(otherCtx, facilities.UpdateRoom{
		ID: released.ID, Name: "Turnhalle", IsOpenRoom: releasePtr(false),
	})
	assert.Error(t, err, "a foreign tenant must not be able to change a release")

	stillReleased, err := module.FindRoom(ctx, released.ID)
	require.NoError(t, err)
	assert.True(t, stillReleased.IsOpenRoom,
		"the owning school's release must survive a foreign write attempt")

	// The same room name in another school is a different room and starts
	// unreleased.
	otherRoom, err := module.CreateRoom(otherCtx, facilities.CreateRoom{Name: "Turnhalle"})
	require.NoError(t, err)
	assert.False(t, otherRoom.IsOpenRoom)
}

func TestToiletRoomReleaseIsRefusedAtThePersistenceSeam(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: facilities.WCRoomName, IsSystem: true,
	})
	require.NoError(t, err)
	require.False(t, created.IsOpenRoom)

	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: created.ID, Name: facilities.WCRoomName, IsOpenRoom: releasePtr(true),
	})
	require.ErrorIs(t, err, facilities.ErrToiletRoomNotReleasable)

	found, err := module.FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, found.IsOpenRoom,
		"the refused release must not have been written")
}
