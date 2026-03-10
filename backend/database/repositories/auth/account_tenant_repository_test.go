package auth_test

import (
	"testing"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountTenantRepository_CreateAndQuery(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "acctenant")
	tenantID := account.ID + 1000
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		cleanupAccountRecords(t, db, account.ID)
	})

	t.Run("creates active mapping", func(t *testing.T) {
		item := &authModels.AccountTenant{
			AccountID: account.ID,
			TenantID:  tenantID,
			Status:    authModels.AccountTenantStatusActive,
		}
		err := repo.Create(ctx, item)
		require.NoError(t, err)
		assert.NotZero(t, item.ID)
	})

	t.Run("finds active mappings by account id", func(t *testing.T) {
		items, err := repo.FindActiveByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, tenantID, items[0].TenantID)
	})

	t.Run("exists by account and tenant returns true", func(t *testing.T) {
		exists, err := repo.ExistsByAccountAndTenant(ctx, account.ID, tenantID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("exists by account and tenant returns false for missing mapping", func(t *testing.T) {
		exists, err := repo.ExistsByAccountAndTenant(ctx, account.ID, tenantID+999999)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestAccountTenantRepository_CreateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.TenantContext(1)

	err := repo.Create(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account tenant cannot be nil")

	err = repo.Create(ctx, &authModels.AccountTenant{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id is required")
}
