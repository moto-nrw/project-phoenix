package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// newProjectedOperatorAccountDirectory uses the serving owner projection.
func newProjectedOperatorAccountDirectory(t *testing.T, db *bun.DB) repositories.OperatorAccountDirectory {
	t.Helper()
	schools, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	return repositories.NewOperatorAccountDirectory(identity, persons, membership, schools)
}

func TestOperatorAccountDirectory_ListAccountsByTenantID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, "list-by-tenant")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Maria", "Schmidt")
	_, err := db.ExecContext(ctx,
		`UPDATE users.persons SET account_id = ? WHERE id = ?`, account.ID, person.ID)
	require.NoError(t, err)

	// Cleanup in reverse order (data deps)
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users.persons WHERE id = ?`, person.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}()

	repo := newProjectedOperatorAccountDirectory(t, db)

	t.Run("returns accounts for tenant", func(t *testing.T) {
		// The composed repository resolves persons through the tenant runtime.
		accounts, err := repo.ListAccountsByTenantID(testpkg.TenantContext(tenantID), tenantID)
		require.NoError(t, err)

		var found bool
		for _, a := range accounts {
			if a.Email == account.Email {
				found = true
				assert.Equal(t, account.ID, a.AccountID)
				assert.Equal(t, "Maria", a.FirstName)
				assert.Equal(t, "Schmidt", a.LastName)
				break
			}
		}
		assert.True(t, found, "expected to find test account in results")
	})

	t.Run("returns empty for nonexistent tenant", func(t *testing.T) {
		accounts, err := repo.ListAccountsByTenantID(testpkg.TenantContext(999999), 999999)
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})
}

func TestOperatorAccountDirectory_ListAccountsByTenantID_IncludesPendingInvitations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	role := testpkg.GetOrCreateTestRole(t, db, "admin")

	invitation := testpkg.CreateTestInvitationToken(t, db, "pending-invite", role.ID, 0, time.Now().Add(24*time.Hour))
	_, err := db.ExecContext(ctx,
		`UPDATE auth.invitation_tokens SET tenant_id = ? WHERE id = ?`, tenantID, invitation.ID)
	require.NoError(t, err)

	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.invitation_tokens WHERE id = ?`, invitation.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}()

	repo := newProjectedOperatorAccountDirectory(t, db)
	accounts, err := repo.ListAccountsByTenantID(ctx, tenantID)
	require.NoError(t, err)

	var found bool
	for _, a := range accounts {
		if a.Email == invitation.Email {
			found = true
			assert.Equal(t, "invited", a.Status)
			assert.Equal(t, int64(0), a.AccountID)
			break
		}
	}
	assert.True(t, found, "expected to find pending invitation in results")
}

func TestOperatorAccountDirectory_ListAccountsByOrganizationID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	orgID := tenantID // EnsureTestTenant creates org with same ID

	account := testpkg.CreateTestAccount(t, db, "list-by-org")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}()

	repo := newProjectedOperatorAccountDirectory(t, db)

	t.Run("returns accounts for organization", func(t *testing.T) {
		accounts, err := repo.ListAccountsByOrganizationID(ctx, orgID)
		require.NoError(t, err)

		var found bool
		for _, a := range accounts {
			if a.Email == account.Email {
				found = true
				assert.Equal(t, tenantID, a.SchoolID)
				break
			}
		}
		assert.True(t, found, "expected to find test account in org results")
	})

	t.Run("returns empty for nonexistent org", func(t *testing.T) {
		accounts, err := repo.ListAccountsByOrganizationID(ctx, 999999)
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})
}

func TestOperatorAccountDirectory_ListAllAccounts(t *testing.T) {
	t.Parallel()

	// A "list all" sweep reads every school twice (IDs first, names second).
	// On the package database a parallel sibling hard-deletes its school
	// between those reads and the sweep fails with "school missing for
	// account row"; a clone of its own has no concurrent deleters.
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "list-all")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}()

	repo := newProjectedOperatorAccountDirectory(t, db)
	accounts, err := repo.ListAllAccounts(ctx)
	require.NoError(t, err)

	var found bool
	for _, a := range accounts {
		if a.Email == account.Email {
			found = true
			assert.Equal(t, tenantID, a.SchoolID)
			break
		}
	}
	assert.True(t, found, "expected to find test account in all accounts")
}

// containsAccount reports whether the result list includes an entry for the given email.
func containsAccount(accounts []repositories.OrgAccountInfo, email string) bool {
	for _, a := range accounts {
		if a.Email == email {
			return true
		}
	}
	return false
}

// TestOperatorAccountDirectory_ListAllAccounts_ExcludesDeletedSchool verifies that
// the global org-accounts listing hides accounts whose tenant school is in the
// Papierkorb (soft-deleted), and re-includes them after restore.
func TestOperatorAccountDirectory_ListAllAccounts_ExcludesDeletedSchool(t *testing.T) {
	t.Parallel()

	// Same sweep as above, same race with parallel school deletes.
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "list-all-deleted")
	// The subject here is an account whose ONLY school is soft-deleted, so the
	// mapping CreateTestAccount adds for the test's own tenant has to go
	// (#2419) — otherwise the account stays visible through that second school.
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	repo := newProjectedOperatorAccountDirectory(t, db)

	// Baseline: account is visible while school is active.
	accounts, err := repo.ListAllAccounts(ctx)
	require.NoError(t, err)
	require.True(t, containsAccount(accounts, account.Email),
		"baseline: account should be visible while school is active")

	// Soft-delete the school directly to keep the test isolated from
	// SoftDeleteSchool's side effects (token revocation, invitation invalidation).
	_, err = db.ExecContext(ctx,
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	// After soft-delete: account must disappear from global listing.
	accounts, err = repo.ListAllAccounts(ctx)
	require.NoError(t, err)
	require.False(t, containsAccount(accounts, account.Email),
		"account whose school is soft-deleted must not appear in ListAllAccounts")

	// Also hidden when filtered by the parent organization.
	orgAccounts, err := repo.ListAccountsByOrganizationID(ctx, tenantID)
	require.NoError(t, err)
	require.False(t, containsAccount(orgAccounts, account.Email),
		"account whose school is soft-deleted must not appear in ListAccountsByOrganizationID")

	// Restore: account reappears in both listings.
	_, err = db.ExecContext(ctx,
		`UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, tenantID)
	require.NoError(t, err)

	accounts, err = repo.ListAllAccounts(ctx)
	require.NoError(t, err)
	assert.True(t, containsAccount(accounts, account.Email),
		"restore: account should be visible again in ListAllAccounts")

	orgAccounts, err = repo.ListAccountsByOrganizationID(ctx, tenantID)
	require.NoError(t, err)
	assert.True(t, containsAccount(orgAccounts, account.Email),
		"restore: account should be visible again in ListAccountsByOrganizationID")
}
