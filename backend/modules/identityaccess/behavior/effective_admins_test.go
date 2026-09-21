package behavior_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	mapping := &testpkg.AccountTenantFixture{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      "active",
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

	_, err = db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
		accountID, roleID, tenantID).Exec(ctx)
	require.NoError(t, err)
}

func TestNativeEffectiveAdminsIncludeGrantedWildcardsWithoutSchoolLeakage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	query, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	for _, resource := range []string{"admin", "*"} {
		t.Run(resource, func(t *testing.T) {
			var permissionID int64
			require.NoError(t, db.NewRaw(`INSERT INTO auth.permissions (name, resource, action)
				VALUES (?, ?, '*') ON CONFLICT (resource, action) DO UPDATE SET resource = EXCLUDED.resource RETURNING id`,
				uniqueTestName("effective-admin-grant"), resource).Scan(ctx, &permissionID))
			role := testpkg.CreateTestRole(t, db, "custom-admin-scope")
			roleAdmin := testpkg.CreateTestAccount(t, db, "role-admin-scope")
			directAdmin := testpkg.CreateTestAccount(t, db, "direct-admin-scope")
			denied := testpkg.CreateTestAccount(t, db, "denied-admin-scope")
			foreign := testpkg.CreateTestAccount(t, db, "foreign-admin-scope")
			testpkg.EnsureAccountTenant(t, db, foreign.ID, other)
			_, err := db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", roleAdmin.ID, role.ID, home).Exec(ctx)
			require.NoError(t, err)
			_, err = db.NewRaw("INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)", role.ID, permissionID).Exec(ctx)
			require.NoError(t, err)
			_, err = db.NewRaw(`INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted)
				VALUES (?, ?, ?, TRUE), (?, ?, ?, FALSE), (?, ?, ?, TRUE)`,
				directAdmin.ID, permissionID, home, denied.ID, permissionID, home, foreign.ID, permissionID, other).Exec(ctx)
			require.NoError(t, err)
			ids, err := query.ListEffectiveAdminAccountIDs(ctx)
			require.NoError(t, err)
			require.Contains(t, ids, roleAdmin.ID)
			require.Contains(t, ids, directAdmin.ID)
			require.NotContains(t, ids, denied.ID)
			require.NotContains(t, ids, foreign.ID)
			rollback := errors.New("rollback admin grant revocation")
			err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
				txCtx = tenant.WithTenantID(txCtx, home)
				_, writeErr := tx.NewRaw("DELETE FROM auth.role_permissions WHERE role_id = ?", role.ID).Exec(txCtx)
				require.NoError(t, writeErr)
				_, writeErr = tx.NewRaw("UPDATE auth.account_permissions SET granted = FALSE WHERE account_id = ? AND tenant_id = ?", directAdmin.ID, home).Exec(txCtx)
				require.NoError(t, writeErr)
				inside, readErr := query.ListEffectiveAdminAccountIDs(txCtx)
				require.NoError(t, readErr)
				require.NotContains(t, inside, roleAdmin.ID)
				require.NotContains(t, inside, directAdmin.ID)
				return rollback
			})
			require.ErrorIs(t, err, rollback)
			after, err := query.ListEffectiveAdminAccountIDs(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, ids, after)
			global, err := query.ListEffectiveAdminAccountIDs(context.Background())
			require.NoError(t, err)
			require.Contains(t, global, foreign.ID, "an unscoped administrative caller retains platform reads")
		})
	}
}

// TestNativeEffectiveAdminAccountIDs pins who counts as an
// effective admin. The result decides who receives tenant-wide data, so both
// directions matter: missing an admin is an annoyance, including a non-admin is
// a disclosure.
func TestNativeEffectiveAdminAccountIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	t.Run("includes the admin role and excludes a plain user", func(t *testing.T) {
		admin := testpkg.CreateTestAccount(t, db, "effective-admin@example.test")
		plain := testpkg.CreateTestAccount(t, db, "effective-plain@example.test")

		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), "admin")
		grantTenantRole(t, db, ctx, plain.ID, testpkg.Tenant(t), "user")

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.Contains(t, ids, admin.ID)
		assert.NotContains(t, ids, plain.ID, "a plain user is not an effective admin")
	})

	t.Run("excludes a deactivated admin account", func(t *testing.T) {
		admin := testpkg.CreateTestAccount(t, db, "effective-inactive@example.test")
		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), "admin")

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
		grantTenantRole(t, db, ctx, admin.ID, testpkg.Tenant(t), "admin")

		_, err := db.NewUpdate().
			TableExpr("auth.account_tenants").
			Set("status = ?", "inactive").
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
		grantTenantRole(t, db, ctx, foreignAdmin.ID, otherTenant, "admin")

		ids, err := repo.ListEffectiveAdminAccountIDs(ctx)
		require.NoError(t, err)

		assert.NotContains(t, ids, foreignAdmin.ID,
			"admin scope is per school, not global")
	})
}
