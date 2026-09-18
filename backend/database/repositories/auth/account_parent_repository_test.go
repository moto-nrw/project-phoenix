package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// cleanupParentAccount removes a parent account from the database.
func cleanupParentAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()
	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table("auth.accounts_parents").
		Where("id = ?", accountID).
		Exec(context.Background())
}

// ============================================================================
// AccountParentRepository CRUD Tests
// ============================================================================

func TestAccountParentRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountParent
	ctx := testpkg.Ctx(t)

	t.Run("creates parent account with valid data", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("parent_create_%d@example.com", time.Now().UnixNano())
		username := fmt.Sprintf("parent_%d", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.AccountParent{
			Email:        uniqueEmail,
			Username:     &username,
			PasswordHash: &passwordHash,
			Active:       true,
		}

		err := repo.Create(ctx, account)
		require.NoError(t, err)
		assert.NotZero(t, account.ID)

		defer cleanupParentAccount(t, db, account.ID)
	})

	t.Run("creates parent account without username", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("parent_no_user_%d@example.com", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.AccountParent{
			Email:        uniqueEmail,
			PasswordHash: &passwordHash,
			Active:       true,
		}

		err := repo.Create(ctx, account)
		require.NoError(t, err)
		assert.NotZero(t, account.ID)

		defer cleanupParentAccount(t, db, account.ID)
	})

	t.Run("rejects nil account", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountParentRepository_FindByEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountParent
	ctx := testpkg.Ctx(t)

	t.Run("finds parent account by email", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "find_by_email")
		defer cleanupParentAccount(t, db, account.ID)

		found, err := repo.FindByEmail(ctx, account.Email)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
		assert.Equal(t, account.Email, found.Email)
	})

	t.Run("finds parent account by email case insensitive", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("CASE_TEST_%d@EXAMPLE.COM", time.Now().UnixNano())
		username := fmt.Sprintf("caseuser_%d", time.Now().UnixNano())
		account := &auth.AccountParent{
			Email:    uniqueEmail,
			Username: &username,
			Active:   true,
		}
		err := repo.Create(ctx, account)
		require.NoError(t, err)
		defer cleanupParentAccount(t, db, account.ID)

		// Search with the original email (case-insensitive search)
		found, err := repo.FindByEmail(ctx, uniqueEmail)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		_, err := repo.FindByEmail(ctx, "nonexistent_parent@example.com")
		require.Error(t, err)
	})
}

func TestAccountParentRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountParent
	ctx := testpkg.Ctx(t)

	t.Run("updates parent account email", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "update_email")
		defer cleanupParentAccount(t, db, account.ID)

		newEmail := fmt.Sprintf("updated_parent_%d@example.com", time.Now().UnixNano())
		account.Email = newEmail
		err := repo.Update(ctx, account)
		require.NoError(t, err)

		found, err := repo.FindByEmail(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
	})

	t.Run("updates parent account active status", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "update_active")
		defer cleanupParentAccount(t, db, account.ID)

		account.Active = false
		err := repo.Update(ctx, account)
		require.NoError(t, err)

		found, err := repo.FindByEmail(ctx, account.Email)
		require.NoError(t, err)
		assert.False(t, found.Active)
	})

	t.Run("rejects nil account", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountParentRepository_FieldUpdatesRespectOptionalTenantScope(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountParent
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestParentAccount(t, db, "cross-tenant-field-update")

	account.Username = testpkg.StrPtr("cross-tenant-update")
	err := repo.Update(testpkg.TenantContext(otherTenantID), account)
	require.Error(t, err, "another school's context must not update the row")

	account.Username = testpkg.StrPtr("background-update")
	require.NoError(t, repo.Update(context.Background(), account))

	var found auth.AccountParent
	require.NoError(t, db.NewSelect().Model(&found).
		ModelTableExpr(`auth.accounts_parents AS "account_parent"`).
		Where(`"account_parent".id = ?`, account.ID).
		Scan(context.Background()))
	require.NotNil(t, found.Username)
	assert.Equal(t, "background-update", *found.Username)
}

// ============================================================================
// AccountParentRepository List Tests
// ============================================================================

func TestAccountParentRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountParent
	ctx := testpkg.Ctx(t)

	t.Run("lists all parent accounts", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_all")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)

		var found bool
		for _, a := range accounts {
			if a.ID == account.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created account should be in list")
	})

	t.Run("filters by email", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_email_filter")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"email": account.Email,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)

		var found bool
		for _, a := range accounts {
			if a.ID == account.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by username", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_user_filter")
		defer cleanupParentAccount(t, db, account.ID)
		require.NotNil(t, account.Username)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"username": *account.Username,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by email_like", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_like_filter")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"email_like": "list_like_filter",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by username_like", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_userlike")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"username_like": "parent",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by active status", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_active")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"active": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by custom field", func(t *testing.T) {
		account := testpkg.CreateTestParentAccount(t, db, "list_custom")
		defer cleanupParentAccount(t, db, account.ID)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"id": account.ID,
		})
		require.NoError(t, err)
		assert.Len(t, accounts, 1)
		assert.Equal(t, account.ID, accounts[0].ID)
	})
}
