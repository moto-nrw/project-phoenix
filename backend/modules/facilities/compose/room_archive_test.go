package compose

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A room other rows still reference is archived, never deleted: the keys to
// facilities.rooms no longer empty those rows (#6845). The guard double
// reports no references, so this proves the database decides.
func TestDeleteArchivesAReferencedRoomAndKeepsItsReferences(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	room, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum", Color: testpkg.StrPtr("#123456")})
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroupForTenant(t, db, testpkg.Tenant(t), "Igel")
	_, err = db.NewRaw(`UPDATE education.groups SET room_id = ? WHERE id = ?`, room.ID, group.ID).Exec(context.Background())
	require.NoError(t, err)

	removal, err := module.DeleteRoom(ctx, room.ID)

	require.NoError(t, err)
	assert.Equal(t, facilities.RoomRemovalArchive, removal.Outcome)
	var roomID *int64
	require.NoError(t, db.NewRaw(`SELECT room_id FROM education.groups WHERE id = ?`, group.ID).Scan(context.Background(), &roomID))
	require.NotNil(t, roomID, "the group keeps its room")
	assert.Equal(t, room.ID, *roomID)

	archived, err := module.FindRoom(ctx, room.ID)
	require.NoError(t, err, "history still resolves the room")
	require.NotNil(t, archived.ArchivedAt)

	listed, err := module.ListRooms(ctx, facilities.RoomFilter{})
	require.NoError(t, err)
	for _, candidate := range listed {
		assert.NotEqual(t, room.ID, candidate.ID, "archived rooms leave every list")
	}
	_, err = module.FindRoomByName(ctx, "Igelraum")
	require.ErrorIs(t, err, facilities.ErrRoomNotFound)

	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{ID: room.ID, Name: "Igelraum alt"})
	require.ErrorIs(t, err, facilities.ErrRoomArchived)

	replacement, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum", Color: testpkg.StrPtr("#123456")})
	require.NoError(t, err, "name and colour of an archived room are free again")
	assert.NotEqual(t, room.ID, replacement.ID)
}

// A toilet room a school created or renamed itself is a system room: the
// flag, not the name, protects it (#6845).
func TestSelfMadeToiletRoomIsASystemRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	room, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Abstellraum"})
	require.NoError(t, err)
	require.False(t, room.IsSystem)

	toilet, err := module.UpdateRoom(ctx, facilities.UpdateRoom{ID: room.ID, Name: facilities.WCRoomName})
	require.NoError(t, err)
	assert.True(t, toilet.IsSystem)
	_, err = module.DeleteRoom(ctx, toilet.ID)
	require.ErrorIs(t, err, facilities.ErrSystemRoomProtected)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))
	created, err := module.CreateRoom(otherCtx, facilities.CreateRoom{Name: facilities.WCRoomAliasName})
	require.NoError(t, err)
	assert.True(t, created.IsSystem)
}
