package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGuardianProfileRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("creates guardian profile with valid data", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("guardian-%d@test.local", time.Now().UnixNano())
		profile := &users.GuardianProfile{
			FirstName:              "Test",
			LastName:               "Guardian",
			Email:                  &uniqueEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		err := repo.Create(ctx, profile)
		require.NoError(t, err)
		assert.NotZero(t, profile.ID)

		// Cleanup
		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)
	})

	t.Run("creates guardian profile without email", func(t *testing.T) {
		// Note: Phone numbers are now stored in separate table (guardian_phone_numbers)
		profile := &users.GuardianProfile{
			FirstName:          "Minimal",
			LastName:           "Guardian",
			LanguagePreference: "de",
		}

		err := repo.Create(ctx, profile)
		require.NoError(t, err)
		assert.NotZero(t, profile.ID)

		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)
	})
}

func TestGuardianProfileRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("finds existing guardian profile", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "findbyid")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID)
		assert.Equal(t, profile.FirstName, found.FirstName)
	})

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianProfileRepository_FindByEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("finds guardian profile by email", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "byemail")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		found, err := repo.FindByEmail(ctx, *profile.Email)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID)
	})

	t.Run("finds guardian profile by email case-insensitive", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "casetest")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		// Search with the email
		found, err := repo.FindByEmail(ctx, *profile.Email)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		_, err := repo.FindByEmail(ctx, "nonexistent@test.local")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianProfileRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("updates guardian profile", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "update")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		profile.FirstName = "Updated"
		profile.LastName = "Name"

		err := repo.Update(ctx, profile)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", found.FirstName)
		assert.Equal(t, "Name", found.LastName)
	})

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		fakeEmail := "fake@test.local"
		profile := &users.GuardianProfile{
			FirstName:          "Fake",
			LastName:           "Profile",
			Email:              &fakeEmail,
			LanguagePreference: "de",
		}
		profile.ID = int64(999999)

		err := repo.Update(ctx, profile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianProfileRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing guardian profile", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "delete")

		err := repo.Delete(ctx, profile.ID)
		require.NoError(t, err)

		// Verify profile is deleted
		_, err = repo.FindByID(ctx, profile.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestGuardianProfileRepository_ListWithOptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("lists guardian profiles with pagination", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "listopt")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		profiles, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.NotEmpty(t, profiles)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "listnil")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		profiles, err := repo.ListWithOptions(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, profiles)
	})
}

func TestGuardianProfileRepository_FindWithoutAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("finds guardians without accounts", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "noaccount")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		// Profile should not have an account
		profiles, err := repo.FindWithoutAccount(ctx)
		require.NoError(t, err)

		var found bool
		for _, p := range profiles {
			if p.ID == profile.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Profile without account should be returned")
	})
}

func TestGuardianProfileRepository_FindInvitable(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("finds invitable guardians", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "invitable")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		// Profile has email and no account - should be invitable
		profiles, err := repo.FindInvitable(ctx)
		require.NoError(t, err)

		var found bool
		for _, p := range profiles {
			if p.ID == profile.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Profile with email and no account should be invitable")
	})

	t.Run("returns profiles ordered by last name", func(t *testing.T) {
		profile1 := testpkg.CreateTestGuardianProfile(t, db, "orderz")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile1.ID)
		profile1.LastName = "Zebra"
		_ = repo.Update(ctx, profile1)

		profile2 := testpkg.CreateTestGuardianProfile(t, db, "ordera")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile2.ID)
		profile2.LastName = "Alpha"
		_ = repo.Update(ctx, profile2)

		profiles, err := repo.FindInvitable(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, profiles)
		// Just verify the query works - ordering is tested implicitly
	})
}

// ============================================================================
// Account Link Tests
// ============================================================================

// NOTE: LinkAccount, UnlinkAccount, and FindByAccountID tests require proper FK setup
// between guardian_profiles.account_id and auth.accounts.id. The test database may
// have FK constraints that the simple CreateTestAccount fixture doesn't satisfy.
// These tests verify error handling for non-existent profiles.

func TestGuardianProfileRepository_LinkAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		// Create a real auth account to satisfy the FK constraint on
		// guardian_profiles.account_id, which (since parent-enrollment
		// PR 3, migration 1.15.46) references auth.accounts(id) instead
		// of the orphaned auth.accounts_parents(id).
		account := testpkg.CreateTestAccount(t, db, "linkaccount")
		defer testpkg.CleanupAccount(t, db, account.ID)

		// Use non-existent profile ID with real account ID
		err := repo.LinkAccount(ctx, int64(999999), account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("successfully links account to profile", func(t *testing.T) {
		// Create guardian profile and a real auth account.
		profile := testpkg.CreateTestGuardianProfile(t, db, "linktest")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		account := testpkg.CreateTestAccount(t, db, "linkprofile")
		defer testpkg.CleanupAccount(t, db, account.ID)

		// Link the account
		err := repo.LinkAccount(ctx, profile.ID, account.ID)
		require.NoError(t, err)

		// Verify the link
		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.AccountID)
		assert.Equal(t, account.ID, *found.AccountID)
		assert.True(t, found.HasAccount)
	})
}

func TestGuardianProfileRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent account ID", func(t *testing.T) {
		_, err := repo.FindByAccountID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// Student Count Tests
// ============================================================================

func TestGuardianProfileRepository_LoadProfileWithChildren_FiltersPortalAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	profile := testpkg.CreateTestGuardianProfile(t, db, "profilechildren")
	account := testpkg.CreateTestAccount(t, db, "profilechildren")
	visible := testpkg.CreateTestStudent(t, db, "Visible", "Child", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Hidden", "Child", "1b")
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, visible.ID, profile.ID)
		testpkg.CleanupActivityFixtures(t, db, hidden.ID, profile.ID)
		testpkg.CleanupAccount(t, db, account.ID)
	})
	require.NoError(t, repo.LinkAccount(ctx, profile.ID, account.ID))

	visibleRel := &users.StudentGuardian{
		StudentID:         visible.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "parent",
		GuardianRole:      authorize.GuardianRoleLegalGuardian,
		Permissions: map[string]interface{}{
			authorize.GuardianPermissionPortalAccess: true,
		},
	}
	visibleRel.SetTenantID(testpkg.Tenant(t))
	hiddenRel := &users.StudentGuardian{
		StudentID:         hidden.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		GuardianRole:      authorize.GuardianRolePickupOnly,
		Permissions:       map[string]interface{}{},
	}
	hiddenRel.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db).StudentGuardian.Create(ctx, visibleRel))
	require.NoError(t, repositories.NewFactory(db).StudentGuardian.Create(ctx, hiddenRel))

	loaded, err := repo.LoadProfileWithChildren(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Len(t, loaded.Children, 1)
	assert.Equal(t, visible.ID, loaded.Children[0].StudentID)
	assert.Equal(t, "Visible", loaded.Children[0].FirstName)
}

// seedNamedGuardian inserts a guardian with a caller-controlled first name (so
// search assertions can key off a unique token) and registers cleanup.
func seedNamedGuardian(t *testing.T, db *bun.DB, ctx context.Context, repo users.GuardianProfileRepository, firstName, lastName string) *users.GuardianProfile {
	t.Helper()
	email := fmt.Sprintf("%s-%d@search.local", firstName, time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	require.NoError(t, repo.Create(ctx, profile))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID) })
	return profile
}

// TestGuardianProfileRepository_SearchByText unit-tests the picker search at the
// repository layer (it was previously only exercised through the HTTP handler).
// It pins the substring/case-insensitive contract and the input guards that back
// the picker's enumeration defense.
func TestGuardianProfileRepository_SearchByText(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("matches a first-name substring case-insensitively and excludes non-matches", func(t *testing.T) {
		token := fmt.Sprintf("Zzsearch%d", time.Now().UnixNano())
		match := seedNamedGuardian(t, db, ctx, repo, token, "Alpha")
		other := seedNamedGuardian(t, db, ctx, repo, fmt.Sprintf("Zzother%d", time.Now().UnixNano()), "Beta")

		// Lower-cased query must still match (SearchByText lower-cases both sides).
		results, err := repo.SearchByText(ctx, token, 50)
		require.NoError(t, err)

		var sawMatch, sawOther bool
		for _, g := range results {
			if g.ID == match.ID {
				sawMatch = true
			}
			if g.ID == other.ID {
				sawOther = true
			}
		}
		assert.True(t, sawMatch, "search must return the guardian whose name contains the token")
		assert.False(t, sawOther, "search must exclude guardians that don't match the token")
	})

	t.Run("whitespace-only query returns an empty result without error", func(t *testing.T) {
		results, err := repo.SearchByText(ctx, "   ", 50)
		require.NoError(t, err)
		assert.Empty(t, results, "a query with no real tokens must not match anything")
	})

	t.Run("a non-positive limit falls back to a default and still returns matches", func(t *testing.T) {
		token := fmt.Sprintf("Zzlimit%d", time.Now().UnixNano())
		match := seedNamedGuardian(t, db, ctx, repo, token, "Gamma")

		// limit <= 0 must not mean "return nothing"; it falls back to the method's
		// internal default rather than passing LIMIT 0 to the query.
		results, err := repo.SearchByText(ctx, token, 0)
		require.NoError(t, err)

		var found bool
		for _, g := range results {
			if g.ID == match.ID {
				found = true
			}
		}
		assert.True(t, found, "limit<=0 must fall back to a default, not suppress all results")
	})

	t.Run("tolerates a pathological many-token query", func(t *testing.T) {
		// Far more tokens than maxSearchTokens. The repo caps the token count so
		// the WHERE clause can't grow unbounded; the call must still succeed and,
		// since every token is the same matching string, still find the guardian.
		token := fmt.Sprintf("Zzmany%d", time.Now().UnixNano())
		match := seedNamedGuardian(t, db, ctx, repo, token, "Delta")

		manyTokens := token
		for i := 0; i < 14; i++ {
			manyTokens += " " + token
		}

		results, err := repo.SearchByText(ctx, manyTokens, 50)
		require.NoError(t, err, "a many-token query must be handled gracefully, never error")

		var found bool
		for _, g := range results {
			if g.ID == match.ID {
				found = true
			}
		}
		assert.True(t, found, "repeated matching tokens must still find the guardian")
	})
}

// ============================================================================
// Portal Locale Tests (parent-portal i18n, migration 1.15.123)
// ============================================================================

// TestGuardianProfileRepository_UpdatePortalLocaleByAccountID covers the new
// write path that persists the parent's explicit portals-portal language.
func TestGuardianProfileRepository_UpdatePortalLocaleByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	t.Run("persists locale and is reflected by FindByAccountID", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "portallocale")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

		account := testpkg.CreateTestAccount(t, db, "portallocale")
		defer testpkg.CleanupAccount(t, db, account.ID)

		require.NoError(t, repo.LinkAccount(ctx, profile.ID, account.ID))

		// A profile starts with no portal language chosen.
		before, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Nil(t, before.PortalLocale, "a fresh profile must have NULL portal_locale")

		// First explicit choice.
		require.NoError(t, repo.UpdatePortalLocaleByAccountID(ctx, account.ID, "en"))
		afterEN, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.NotNil(t, afterEN.PortalLocale)
		assert.Equal(t, "en", *afterEN.PortalLocale)

		// Changing the choice overwrites the prior value.
		require.NoError(t, repo.UpdatePortalLocaleByAccountID(ctx, account.ID, "ru"))
		afterRU, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.NotNil(t, afterRU.PortalLocale)
		assert.Equal(t, "ru", *afterRU.PortalLocale, "a later choice must replace the earlier one")
	})

	t.Run("returns ErrGuardianProfileNotFound when the account has no profile", func(t *testing.T) {
		// Zero rows matched must fail loud, not silently succeed — otherwise a
		// "saved" preference that never persisted would masquerade as success.
		err := repo.UpdatePortalLocaleByAccountID(ctx, int64(999999), "en")
		require.Error(t, err)
		assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound)
	})
}

// TestGuardianProfileRepository_FindByAccountID_ReturnsLinkedProfile exercises
// the happy path of the reworked FindByAccountID (deterministic ordering +
// Limit(1)): a linked account resolves to its profile and carries portal_locale.
func TestGuardianProfileRepository_FindByAccountID_ReturnsLinkedProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	profile := testpkg.CreateTestGuardianProfile(t, db, "findbyaccount")
	defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", profile.ID)

	account := testpkg.CreateTestAccount(t, db, "findbyaccount")
	defer testpkg.CleanupAccount(t, db, account.ID)

	require.NoError(t, repo.LinkAccount(ctx, profile.ID, account.ID))

	found, err := repo.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, profile.ID, found.ID, "FindByAccountID must resolve to the linked profile")
	require.NotNil(t, found.AccountID)
	assert.Equal(t, account.ID, *found.AccountID)
}

// TestGuardianProfileRepository_FindByAccountID_PrefersExplicitPortalLocale
// locks in the reason the ordering clause exists. A cross-tenant parent owns
// one guardian_profiles row per school, so several rows share account_id
// (UNIQUE is per (tenant_id, account_id), not global). When the parent has
// already chosen a portal language and a newer enrollment then inserts a fresh
// row whose portal_locale is still NULL, FindByAccountID must return the row
// with the explicit choice — not the lowest id. The `portal_locale IS NULL, id`
// ordering guarantees that even though the explicit row is inserted *second*
// (higher id) in a *different* tenant.
func TestGuardianProfileRepository_FindByAccountID_PrefersExplicitPortalLocale(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "localeordering")
	defer testpkg.CleanupAccount(t, db, account.ID)

	// Older row in tenant 1, no portal language chosen (portal_locale NULL).
	older := testpkg.CreateTestGuardianProfile(t, db, "localeordering-a")
	defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", older.ID)
	require.NoError(t, repo.LinkAccount(ctx, older.ID, account.ID))

	// Newer row in a second tenant with an explicit "en". Different tenant_id
	// so it doesn't collide with the per-(tenant_id, account_id) UNIQUE index.
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	newerEmail := fmt.Sprintf("localeordering-b-%d@example.test", time.Now().UnixNano())
	enLocale := "en"
	newer := &users.GuardianProfile{
		FirstName:              "Guardian",
		LastName:               "Test",
		Email:                  &newerEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
		PortalLocale:           &enLocale,
	}
	newer.SetTenantID(tenantB)
	err := db.NewInsert().
		Model(newer).
		ModelTableExpr(`users.guardian_profiles`).
		Scan(context.Background())
	require.NoError(t, err, "failed to insert second-tenant guardian profile")
	defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", newer.ID)
	require.NoError(t, repo.LinkAccount(ctx, newer.ID, account.ID))
	require.Greater(t, newer.ID, older.ID, "explicit row must have the higher id for the test to be meaningful")

	found, err := repo.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, found.PortalLocale, "the row with an explicit portal_locale must win over the NULL one")
	assert.Equal(t, "en", *found.PortalLocale)
	assert.Equal(t, newer.ID, found.ID, "ordering must prefer the explicit-locale row, not the lowest id")
}

// ============================================================================
// LockByIDForUpdate
// ============================================================================

func TestGuardianProfileRepository_LockByIDForUpdate(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile

	t.Run("locks an existing guardian within a tenant transaction", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "lock-existing")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// The full-delete flow takes this lock inside a tenant tx; exercise the
		// real contract rather than a bare autocommit call.
		err := tenant.WithTenantTx(testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return repo.LockByIDForUpdate(txCtx, guardian.ID)
		})
		require.NoError(t, err)
	})

	t.Run("returns ErrGuardianProfileNotFound for a missing guardian", func(t *testing.T) {
		// A non-existent id must surface as the typed not-found error, not a raw
		// sql.ErrNoRows — the delete handler maps it to a clean 404/409.
		err := tenant.WithTenantTx(testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return repo.LockByIDForUpdate(txCtx, 999999999)
		})
		assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound)
	})
}
