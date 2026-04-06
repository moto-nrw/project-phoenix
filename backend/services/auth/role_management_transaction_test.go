package auth

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleManagement_PersistsRoleChangesWithoutTokenRevocation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	t.Run("AssignRoleToAccount persists created mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "assign-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

		role := testpkg.CreateTestRole(t, db, "assign-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		err := service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		require.Len(t, roles, 1)
		assert.Equal(t, role.ID, roles[0].ID)
	})

	t.Run("RemoveRoleFromAccount persists deleted mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "remove-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

		role := testpkg.CreateTestRole(t, db, "remove-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID)))

		err := service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}
