package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoomCommandsRejectAnotherTenantsRoomID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	callerCtx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenantID))
	foreign, err := module.CreateRoom(otherCtx, facilities.CreateRoom{Name: "Fuchsbau"})
	require.NoError(t, err)

	_, err = module.UpdateRoom(callerCtx, facilities.UpdateRoom{ID: foreign.ID, Name: "Renamed"})
	require.ErrorIs(t, err, facilities.ErrRoomNotFound)
	require.ErrorIs(t, module.DeleteRoom(callerCtx, foreign.ID), facilities.ErrRoomNotFound)

	unchanged, err := module.FindRoom(otherCtx, foreign.ID)
	require.NoError(t, err)
	assert.Equal(t, "Fuchsbau", unchanged.Name)
}
