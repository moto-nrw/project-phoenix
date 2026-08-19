package auth_test

import (
	"context"
	"testing"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountTenantRepository_CreateAndQuery(t *testing.T) {
	db := testpkg.SetupTestDB(t)

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

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.TenantContext(1)

	err := repo.Create(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account tenant cannot be nil")

	err = repo.Create(ctx, &authModels.AccountTenant{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id is required")
}

func TestAccountTenantRepository_EnsureActive(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "acctenant-reactivate")
	tenantID := account.ID + 2000
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		cleanupAccountRecords(t, db, account.ID)
	})

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
	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewAccountTenantRepository(db)
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "acctenant-deactivate")
	tenantID := account.ID + 3000
	otherTenantID := account.ID + 3001
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		cleanupAccountRecords(t, db, account.ID)
	})

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
		require.Len(t, active, 1)
		assert.Equal(t, otherTenantID, active[0].TenantID)
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
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := int64(90001)
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
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := int64(90002)
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
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := int64(90003)
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

	repo := authRepo.NewAccountTenantRepository(db)

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
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := int64(90004)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "list-all")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}()

	repo := authRepo.NewAccountTenantRepository(db)
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
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := int64(90011)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "list-all-deleted")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	repo := authRepo.NewAccountTenantRepository(db)

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
