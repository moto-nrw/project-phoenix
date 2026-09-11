package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestModuleClassifiesToiletAliasUpdateConflicts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: facilities.WCRoomName})
	require.NoError(t, err)
	room, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)

	_, err = module.UpdateRoom(ctx, facilities.UpdateRoom{ID: room.ID, Name: facilities.WCRoomAliasName})
	require.ErrorIs(t, err, facilities.ErrDuplicateToiletRoom)
}
