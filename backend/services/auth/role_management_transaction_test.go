package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingDeleteByAccountTokenRepo struct {
	authModels.TokenRepository
	deleteErr error
}

func (r failingDeleteByAccountTokenRepo) DeleteByAccountID(context.Context, int64) error {
	return r.deleteErr
}

func TestRoleManagement_RollsBackWhenTokenRevocationFails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	t.Run("AssignRoleToAccount rolls back created mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "assign-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

		role := testpkg.CreateTestRole(t, db, "assign-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		expectedErr := errors.New("token cleanup failed")
		service.repos.Token = failingDeleteByAccountTokenRepo{
			TokenRepository: service.repos.Token,
			deleteErr:       expectedErr,
		}

		err := service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("RemoveRoleFromAccount rolls back deleted mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "remove-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

		role := testpkg.CreateTestRole(t, db, "remove-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID)))

		expectedErr := errors.New("token cleanup failed")
		service.repos.Token = failingDeleteByAccountTokenRepo{
			TokenRepository: service.repos.Token,
			deleteErr:       expectedErr,
		}

		err := service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		require.Len(t, roles, 1)
		assert.Equal(t, role.ID, roles[0].ID)
	})
}
