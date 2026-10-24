package behavior_test

import (
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An account holds a role by its exact name at the school in context; a
// role it does not hold, or one of another account, never counts.
func TestAccountHoldsRoleByExactName(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	roles := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "holds-role")
	peer := testpkg.CreateTestAccount(t, db, "holds-role-peer")
	baseRole := "user"
	held, err := roles.CreateRole(ctx, "Springerin", "held", &baseRole)
	require.NoError(t, err)
	notHeld, err := roles.CreateRole(ctx, "Leitung", "not held", &baseRole)
	require.NoError(t, err)
	require.NoError(t, roles.AssignRoleToAccount(ctx, account.ID, held.ID))

	assert.True(t, roles.AccountHoldsRole(ctx, account.ID, held.Name))
	assert.False(t, roles.AccountHoldsRole(ctx, account.ID, strings.ToUpper(held.Name)), "the name matches exactly")
	assert.False(t, roles.AccountHoldsRole(ctx, account.ID, notHeld.Name))
	assert.False(t, roles.AccountHoldsRole(ctx, peer.ID, held.Name))
}
