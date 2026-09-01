package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// grantTenantRole gives an account an active mapping to a tenant plus one
// seeded base role, which is the minimum an account needs to count as a real
// member of a school.
func grantTenantRole(t *testing.T, db *bun.DB, ctx context.Context, accountID, tenantID int64, roleName string) {
	t.Helper()

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	// Upsert: since #2419 CreateTestAccount already maps a fixture account to
	// the tenant of the test that created it, so this row may already exist —
	// what this call still decides is its status.
	_, err := db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status, activated_at = EXCLUDED.activated_at").
		Exec(ctx)
	require.NoError(t, err)

	var roleID int64
	err = db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		Limit(1).
		Scan(ctx, &roleID)
	require.NoError(t, err, "seeded role %q must exist", roleName)

	assignment := &authModels.AccountRole{AccountID: accountID, RoleID: roleID}
	assignment.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(t, err)
}

// TestAccountRepository_ListEffectiveAdminAccountIDs pins who counts as an
// effective admin. The result decides who receives tenant-wide data, so both
// directions matter: missing an admin is an annoyance, including a non-admin is
// a disclosure.
func TestAccountRepository_ListEffectiveAdminAccountIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Account
	ctx := testpkg.Ctx(t)

	t.Run("includes the admin role and excludes a plain user", func(t *testing.T) {
		admin := testpkg.CreateTestAccount(t, db, "effective-admin@example.test")
		plain := testpkg.CreateTestAccount(t, db, "effective-plain@example.test")

		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), authModels.BaseRoleAdmin)
		grantTenantRole(t, db, ctx, plain.ID, testpkg.Tenant(t), authModels.BaseRoleUser)

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.Contains(t, ids, admin.ID)
		assert.NotContains(t, ids, plain.ID, "a plain user is not an effective admin")
	})

	t.Run("excludes a deactivated admin account", func(t *testing.T) {
		admin := testpkg.CreateTestAccount(t, db, "effective-inactive@example.test")
		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), authModels.BaseRoleAdmin)

		_, err := db.NewUpdate().
			TableExpr("auth.accounts").
			Set("active = false").
			Where("id = ?", admin.ID).
			Exec(ctx)
		require.NoError(t, err)

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.NotContains(t, ids, admin.ID, "a deactivated account receives nothing")
	})

	t.Run("excludes an admin whose tenant mapping is not active", func(t *testing.T) {
		admin := testpkg.CreateTestAccount(t, db, "effective-pending@example.test")
		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), authModels.BaseRoleAdmin)

		_, err := db.NewUpdate().
			TableExpr("auth.account_tenants").
			Set("status = ?", authModels.AccountTenantStatusInactive).
			Where("account_id = ? AND tenant_id = ?", admin.ID, testpkg.Tenant(t)).
			Exec(ctx)
		require.NoError(t, err)

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.NotContains(t, ids, admin.ID, "membership must be active, not merely present")
	})

	t.Run("does not leak another tenant's admins", func(t *testing.T) {
		otherTenant := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignAdmin := testpkg.CreateTestAccount(t, db, "effective-foreign@example.test")
		grantTenantRole(t, db, ctx, foreignAdmin.ID, otherTenant, authModels.BaseRoleAdmin)

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.NotContains(t, ids, foreignAdmin.ID,
			"admin scope is per school, not global")
	})
}
