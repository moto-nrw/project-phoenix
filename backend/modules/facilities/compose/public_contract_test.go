package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicRoomContractReturnsStoredRoom(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	var catalog facilities.Catalog
	catalog, err := compose.NewRooms(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	found, err := catalog.FindRoom(testpkg.Ctx(t), room.ID)
	require.NoError(t, err)
	require.Equal(t, room.ID, found.ID)
	require.Equal(t, room.Name, found.Name)
	require.Equal(t, testpkg.Tenant(t), found.TenantID)
}
