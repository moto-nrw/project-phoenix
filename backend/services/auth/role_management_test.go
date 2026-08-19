package auth_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccountEmailsByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	svc := setupAuthService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns emails for valid account IDs", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "svc-emails1")
		account2 := testpkg.CreateTestAccount(t, db, "svc-emails2")
		defer testpkg.CleanupAuthFixtures(t, db, account1.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account2.ID)

		result, err := svc.GetAccountEmailsByIDs(ctx, []int64{account1.ID, account2.ID})
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, account1.Email, result[account1.ID])
		assert.Equal(t, account2.Email, result[account2.ID])
	})

	t.Run("returns empty map for empty slice", func(t *testing.T) {
		result, err := svc.GetAccountEmailsByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("returns partial results for mixed valid and invalid IDs", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "svc-emailspartial")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		result, err := svc.GetAccountEmailsByIDs(ctx, []int64{account.ID, int64(999999)})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, account.Email, result[account.ID])
	})
}
