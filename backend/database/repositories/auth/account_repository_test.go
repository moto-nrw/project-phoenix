package auth_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestAccountRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("creates account with valid data", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("testcreate_%d@example.com", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.Account{
			Email:        uniqueEmail,
			PasswordHash: &passwordHash,
			Active:       true,
		}

		err := repo.Create(ctx, account)
		require.NoError(t, err)
		assert.NotZero(t, account.ID)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	})

	t.Run("creates account with username", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("testusername_%d@example.com", time.Now().UnixNano())
		uniqueUsername := fmt.Sprintf("user_%d", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.Account{
			Email:        uniqueEmail,
			Username:     &uniqueUsername,
			PasswordHash: &passwordHash,
			Active:       true,
		}

		err := repo.Create(ctx, account)
		require.NoError(t, err)
		assert.NotZero(t, account.ID)
		assert.NotNil(t, account.Username)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	})
}

func TestAccountRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("finds existing account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "findbyid")

		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
		assert.Contains(t, found.Email, "findbyid")
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestAccountRepository_FindByEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("finds account by email", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "findbyemail")

		found, err := repo.FindByEmail(ctx, account.Email)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		_, err := repo.FindByEmail(ctx, "nonexistent@example.com")
		require.Error(t, err)
	})
}

func TestAccountRepository_FindByUsername(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("finds account by username", func(t *testing.T) {
		// Create account with username
		uniqueEmail := fmt.Sprintf("username_%d@example.com", time.Now().UnixNano())
		uniqueUsername := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.Account{
			Email:        uniqueEmail,
			Username:     &uniqueUsername,
			PasswordHash: &passwordHash,
			Active:       true,
		}
		err := repo.Create(ctx, account)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		found, err := repo.FindByUsername(ctx, uniqueUsername)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
	})

	t.Run("returns error for non-existent username", func(t *testing.T) {
		_, err := repo.FindByUsername(ctx, "nonexistent_username")
		require.Error(t, err)
	})
}

func TestAccountRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("updates account email", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "update")

		newEmail := fmt.Sprintf("updated_%d@example.com", time.Now().UnixNano())
		account.Email = newEmail
		err := repo.Update(ctx, account)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, newEmail, found.Email)
	})

	t.Run("updates account active status", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deactivate")

		account.Active = false
		err := repo.Update(ctx, account)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		assert.False(t, found.Active)
	})
}

func TestAccountRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "delete")

		err := repo.Delete(ctx, account.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, account.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestAccountRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("lists all accounts", func(t *testing.T) {
		testpkg.CreateTestAccount(t, db, "list")

		accounts, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})
}

func TestAccountRepository_FindByRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("finds accounts by role name", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "findbyrole")
		role := testpkg.CreateTestRole(t, db, "FindByRoleTestRole")

		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		accounts, err := repo.FindByRole(ctx, role.Name)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)

		var found bool
		for _, a := range accounts {
			if a.ID == account.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Account should be found by role")
	})

	t.Run("finds accounts by role name case insensitive", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "rolecase")
		role := testpkg.CreateTestRole(t, db, "CaseSensitiveRole")

		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		upperRoleName := strings.ToUpper(role.Name)
		accounts, err := repo.FindByRole(ctx, upperRoleName)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("returns empty slice for non-existent role", func(t *testing.T) {
		accounts, err := repo.FindByRole(ctx, "NonExistentRoleName12345")
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})
}

// ============================================================================
// Update Operations
// ============================================================================

func TestAccountRepository_UpdateLastLogin(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("updates last login timestamp", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "lastlogin")

		// Get original last login
		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		originalLastLogin := found.LastLogin

		// Update last login
		err = repo.UpdateLastLogin(ctx, account.ID)
		require.NoError(t, err)

		// Verify update
		found, err = repo.FindByID(ctx, account.ID)
		require.NoError(t, err)

		if originalLastLogin == nil {
			assert.NotNil(t, found.LastLogin)
		} else {
			assert.True(t, found.LastLogin.After(*originalLastLogin))
		}
	})
}

func TestAccountRepository_UpdatePassword(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("updates password hash", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "password")

		newHash := "$argon2id$v=19$m=65536,t=3,p=4$newpasswordhash"
		err := repo.UpdatePassword(ctx, account.ID, newHash)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found.PasswordHash)
		assert.Equal(t, newHash, *found.PasswordHash)
	})
}

func TestAccountRepository_UpdateAvatar(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("updates global avatar path", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "avatar")

		newAvatar := "/uploads/avatars/global/avatar-test.jpg"
		err := repo.UpdateAvatar(ctx, account.ID, newAvatar)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, newAvatar, found.Avatar)
	})
}

// ============================================================================
// Complex Query Tests
// ============================================================================

func TestAccountRepository_FindAccountsWithRolesAndPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("finds accounts with their roles and permissions", func(t *testing.T) {
		// Create account with role
		account := testpkg.CreateTestAccount(t, db, "withperms")
		role := testpkg.CreateTestRole(t, db, "WithPermsRole")

		// Assign role to account
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		// Find accounts with roles and permissions
		accounts, err := repo.FindAccountsWithRolesAndPermissions(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})
}

// ============================================================================
// List with Filters Tests
// ============================================================================

func TestAccountRepository_ListWithFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("filters by email", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "emailfilter")

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

	t.Run("filters by active status", func(t *testing.T) {
		testpkg.CreateTestAccount(t, db, "activefilter")

		accounts, err := repo.List(ctx, map[string]interface{}{
			"active": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by email_like", func(t *testing.T) {
		testpkg.CreateTestAccount(t, db, "likefilter")

		accounts, err := repo.List(ctx, map[string]interface{}{
			"email_like": "likefilter",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by username", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("usernamefilter_%d@example.com", time.Now().UnixNano())
		uniqueUsername := fmt.Sprintf("filteruser_%d", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.Account{
			Email:        uniqueEmail,
			Username:     &uniqueUsername,
			PasswordHash: &passwordHash,
			Active:       true,
		}
		err := repo.Create(ctx, account)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		accounts, err := repo.List(ctx, map[string]interface{}{
			"username": uniqueUsername,
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

	t.Run("filters by username_like", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("usernamelike_%d@example.com", time.Now().UnixNano())
		uniqueUsername := fmt.Sprintf("likeuser_%d", time.Now().UnixNano())
		passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$testpasswordhash"
		account := &auth.Account{
			Email:        uniqueEmail,
			Username:     &uniqueUsername,
			PasswordHash: &passwordHash,
			Active:       true,
		}
		err := repo.Create(ctx, account)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		accounts, err := repo.List(ctx, map[string]interface{}{
			"username_like": "likeuser_",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})

	t.Run("filters by role", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "rolefilter")
		role := testpkg.CreateTestRole(t, db, "ListFilterRole")

		// Assign role to account
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		accounts, err := repo.List(ctx, map[string]interface{}{
			"role": role.Name,
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

	t.Run("filters by custom field", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "customfield")

		// Use a field that exists in the accounts table
		accounts, err := repo.List(ctx, map[string]interface{}{
			"id": account.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)
	})
}

// ============================================================================
// Batch Query Tests
// ============================================================================

func TestAccountRepository_FindEmailsByAccountIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("returns emails for valid IDs", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "emails1")
		account2 := testpkg.CreateTestAccount(t, db, "emails2")

		result, err := repo.FindEmailsByAccountIDs(ctx, []int64{account1.ID, account2.ID})
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, account1.Email, result[account1.ID])
		assert.Equal(t, account2.Email, result[account2.ID])
	})

	t.Run("returns partial results for mixed valid and invalid IDs", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "emailspartial")

		result, err := repo.FindEmailsByAccountIDs(ctx, []int64{account.ID, int64(999999)})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, account.Email, result[account.ID])
	})

	t.Run("returns empty map for empty slice", func(t *testing.T) {
		result, err := repo.FindEmailsByAccountIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestAccountRepository_CreateValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("rejects nil account", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountRepository_UpdateValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("rejects nil account", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountRepository_CalendarFeedToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "feedtoken")

	t.Run("unknown token resolves to nil without error", func(t *testing.T) {
		found, err := repo.FindByCalendarFeedToken(ctx, "does-not-exist")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("empty token resolves to nil", func(t *testing.T) {
		found, err := repo.FindByCalendarFeedToken(ctx, "")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("set then resolve round-trips", func(t *testing.T) {
		require.NoError(t, repo.SetCalendarFeedToken(ctx, account.ID, "feed-token-abc"))

		found, err := repo.FindByCalendarFeedToken(ctx, "feed-token-abc")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, account.ID, found.ID)

		// Rotating invalidates the old token.
		require.NoError(t, repo.SetCalendarFeedToken(ctx, account.ID, "feed-token-xyz"))
		old, err := repo.FindByCalendarFeedToken(ctx, "feed-token-abc")
		require.NoError(t, err)
		assert.Nil(t, old)
	})
}

func TestAccountRepository_EnsureCalendarFeedToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "ensurefeedtoken")

	// First caller claims the token.
	first, err := repo.EnsureCalendarFeedToken(ctx, account.ID, "ensure-token-1")
	require.NoError(t, err)
	assert.Equal(t, "ensure-token-1", first)

	// A second caller with a different token does NOT overwrite it — it gets the
	// already-persisted token back. This is what prevents a racing first-time
	// caller from being handed a URL that a later write overwrote.
	second, err := repo.EnsureCalendarFeedToken(ctx, account.ID, "ensure-token-2")
	require.NoError(t, err)
	assert.Equal(t, "ensure-token-1", second)

	// The persisted token still resolves; the losing token never does.
	found, err := repo.FindByCalendarFeedToken(ctx, "ensure-token-1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, account.ID, found.ID)

	loser, err := repo.FindByCalendarFeedToken(ctx, "ensure-token-2")
	require.NoError(t, err)
	assert.Nil(t, loser)
}
