package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newSchoolProjectedAccountTenantRepository(t *testing.T, db *bun.DB) authModels.AccountTenantRepository {
	t.Helper()
	capability, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	factory := repositories.NewFactory(db)
	factory.BindOrganizationTenancy(capability)
	return factory.AccountTenant
}

func TestAccountTenantRepository_CreateAndQuery(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "acctenant")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

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
		// Two: the one created above, plus the one CreateTestAccount claims
		// for this test's own tenant (#2419).
		require.Len(t, items, 2)
		assert.Contains(t, activeTenantIDs(items), tenantID)
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

// activeTenantIDs projects mappings onto their tenant IDs, so assertions can
// name the tenants they care about instead of an index.
func activeTenantIDs(items []authModels.AccountTenant) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TenantID)
	}
	return ids
}

func TestAccountTenantRepository_CreateValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.Ctx(t)

	err := repo.Create(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account tenant cannot be nil")

	err = repo.Create(ctx, &authModels.AccountTenant{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id is required")
}

func TestAccountTenantRepository_EnsureActive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "acctenant-reactivate")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	deactivatedAt := time.Now().Add(-time.Hour)
	inactive := &authModels.AccountTenant{
		AccountID:     account.ID,
		TenantID:      tenantID,
		Status:        authModels.AccountTenantStatusInactive,
		DeactivatedAt: &deactivatedAt,
	}
	require.NoError(t, repo.Create(ctx, inactive))

	activatedAt := time.Now()
	err := repo.EnsureActive(ctx, &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &activatedAt,
	})
	require.NoError(t, err)

	var mapping authModels.AccountTenant
	err = db.NewSelect().
		Model(&mapping).
		ModelTableExpr(`auth.account_tenants AS "account_tenant"`).
		Where(`"account_tenant".account_id = ?`, account.ID).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, authModels.AccountTenantStatusActive, mapping.Status)
	assert.NotNil(t, mapping.ActivatedAt)
	assert.Nil(t, mapping.DeactivatedAt)
}

func TestAccountTenantRepository_Deactivate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "acctenant-deactivate")
	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	for _, tid := range []int64{tenantID, otherTenantID} {
		require.NoError(t, repo.Create(ctx, &authModels.AccountTenant{
			AccountID: account.ID,
			TenantID:  tid,
			Status:    authModels.AccountTenantStatusActive,
		}))
	}

	require.NoError(t, repo.Deactivate(ctx, account.ID, tenantID))

	t.Run("deactivated mapping is inactive with timestamp", func(t *testing.T) {
		var mapping authModels.AccountTenant
		err := db.NewSelect().
			Model(&mapping).
			ModelTableExpr(`auth.account_tenants AS "account_tenant"`).
			Where(`"account_tenant".account_id = ?`, account.ID).
			Where(`"account_tenant".tenant_id = ?`, tenantID).
			Scan(ctx)
		require.NoError(t, err)
		assert.Equal(t, authModels.AccountTenantStatusInactive, mapping.Status)
		assert.NotNil(t, mapping.DeactivatedAt)
	})

	t.Run("exists check no longer matches deactivated mapping", func(t *testing.T) {
		exists, err := repo.ExistsByAccountAndTenant(ctx, account.ID, tenantID)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("other tenant mapping stays active", func(t *testing.T) {
		exists, err := repo.ExistsByAccountAndTenant(ctx, account.ID, otherTenantID)
		require.NoError(t, err)
		assert.True(t, exists)

		active, err := repo.FindActiveByAccountID(ctx, account.ID)
		require.NoError(t, err)
		tenants := activeTenantIDs(active)
		assert.Contains(t, tenants, otherTenantID, "the other mapping stays active")
		assert.NotContains(t, tenants, tenantID, "the deactivated mapping is gone")
		// The third is this test's own tenant, which CreateTestAccount claims
		// for every fixture account (#2419).
		assert.Len(t, tenants, 2)
	})

	t.Run("EnsureActive reactivates the deactivated mapping", func(t *testing.T) {
		now := time.Now()
		require.NoError(t, repo.EnsureActive(ctx, &authModels.AccountTenant{
			AccountID:   account.ID,
			TenantID:    tenantID,
			ActivatedAt: &now,
		}))
		exists, err := repo.ExistsByAccountAndTenant(ctx, account.ID, tenantID)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestAccountTenantRepository_ListAccountsByTenantID(t *testing.T) {
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

	repo := authRepo.NewAccountTenantRepository(db)

	t.Run("returns accounts for tenant", func(t *testing.T) {
		accounts, err := repo.ListAccountsByTenantID(ctx, tenantID)
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
		accounts, err := repo.ListAccountsByTenantID(ctx, 999999)
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})
}

func TestAccountTenantRepository_ListAccountsByTenantID_IncludesPendingInvitations(t *testing.T) {
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

	repo := authRepo.NewAccountTenantRepository(db)
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

func TestAccountTenantRepository_ListAccountsByOrganizationID(t *testing.T) {
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

	repo := newSchoolProjectedAccountTenantRepository(t, db)

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

func TestAccountTenantRepository_ListAllAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
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

	repo := newSchoolProjectedAccountTenantRepository(t, db)
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
func containsAccount(accounts []authModels.OrgAccountInfo, email string) bool {
	for _, a := range accounts {
		if a.Email == email {
			return true
		}
	}
	return false
}

// TestAccountTenantRepository_ListAllAccounts_ExcludesDeletedSchool verifies that
// the global org-accounts listing hides accounts whose tenant school is in the
// Papierkorb (soft-deleted), and re-includes them after restore.
func TestAccountTenantRepository_ListAllAccounts_ExcludesDeletedSchool(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
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

	repo := newSchoolProjectedAccountTenantRepository(t, db)

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
