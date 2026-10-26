package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRetireDirectPermissionsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, retireDirectPermissionsDown(ctx, db))
	account := testpkg.CreateTestAccount(t, db, "retire-direct")
	permission := testpkg.CreateTestPermission(t, db, "retire-direct", "retire-direct", "read")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), account.ID, permission.Name)
	_, err := db.ExecContext(ctx, `INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted) VALUES (?, ?, ?, TRUE)`, account.ID, permission.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	roleRights := func() []string {
		t.Helper()
		var names []string
		require.NoError(t, db.NewRaw(`SELECT DISTINCT p.resource || ':' || p.action
   FROM auth.account_roles ar JOIN auth.role_permissions rp ON rp.role_id = ar.role_id
   JOIN auth.permissions p ON p.id = rp.permission_id
   WHERE ar.account_id = ? AND ar.tenant_id = ? ORDER BY 1`, account.ID, testpkg.Tenant(t)).Scan(ctx, &names))
		return names
	}
	before := roleRights()
	require.Contains(t, before, permission.Resource+":"+permission.Action)
	require.NoError(t, retireDirectPermissionsUp(ctx, db))
	assert.Equal(t, before, roleRights(), "removing redundant direct grants preserves role permissions")
	var table *string
	require.NoError(t, db.NewRaw(`SELECT to_regclass('auth.account_permissions')::text`).Scan(ctx, &table))
	assert.Nil(t, table)
	var callers []string
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || p.proname FROM pg_proc p
  JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') AND p.prosrc LIKE '%auth.account_permissions%'`).Scan(ctx, &callers))
	assert.Empty(t, callers, "no installed function may access removed storage")
	require.NoError(t, retireDirectPermissionsDown(ctx, db))
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_permissions`).Scan(ctx, &rows))
	assert.Zero(t, rows, "Down restores storage, not deleted direct grants")
	assert.Equal(t, before, roleRights())
	require.NoError(t, retireDirectPermissionsUp(ctx, db), "the restored schema can migrate forward again")
	result := eraseSchoolAsCLI(t, db, testpkg.Tenant(t), beginErasure(t, db, testpkg.Tenant(t)))
	require.Equal(t, "db_done", result.Status, "school erasure must still pass its schema and FK guards: %s", result.ErrorCode)
}

func TestRetireDirectPermissionsDownRestoresTenantIsolationAndFences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, retireDirectPermissionsDown(ctx, db))
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "rollback-direct")
	permission := testpkg.CreateTestPermission(t, db, "rollback-direct", "rollback-direct", "read")
	_, err := db.ExecContext(ctx, `INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)`, account.ID, permission.ID, home, account.ID, permission.ID, other)
	require.NoError(t, err, "the same grant can exist independently at different schools")
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, home, func(ctx context.Context, tx bun.Tx) error {
		var schools []int64
		if err := tx.NewRaw(`SELECT tenant_id FROM auth.account_permissions`).Scan(ctx, &schools); err != nil {
			return err
		}
		assert.Equal(t, []int64{home}, schools)
		return nil
	}))
	err = testpkg.WithTenantTx(t, ctx, db, home, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted) VALUES (?, ?, ?, FALSE)`, account.ID, permission.ID, other)
		return err
	})
	require.Error(t, err, "a tenant cannot write another school's grant")
	_, err = db.ExecContext(ctx, `INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id) VALUES (?, ?, ?)`, account.ID, permission.ID, home)
	require.ErrorContains(t, err, "idx_account_permissions_tenant")
	_, err = db.ExecContext(ctx, `UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.account_permissions SET granted = FALSE WHERE account_id = ?`, account.ID)
	require.ErrorContains(t, err, "cannot reference an erased account", "rollback restores the live-account reference fence")
}

func TestRetireDirectPermissionsRefusesUnknownDependenciesAtomically(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, retireDirectPermissionsDown(ctx, db))
	account := testpkg.CreateTestAccount(t, db, "retirement-dependency")
	permission := testpkg.CreateTestPermission(t, db, "retirement-dependency", "retirement-dependency", "read")
	_, err := db.ExecContext(ctx, `INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id) VALUES (?, ?, ?);
  CREATE VIEW auth.direct_permission_dependency_probe AS SELECT id FROM auth.account_permissions`, account.ID, permission.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.ErrorContains(t, retireDirectPermissionsUp(ctx, db), "depend", "unknown dependencies must not be removed by CASCADE")
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_permissions WHERE account_id = ?`, account.ID).Scan(ctx, &rows))
	assert.Equal(t, 1, rows, "failed migration preserves stored grants")
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('auth.account_is_unused(bigint,text)'::regprocedure)`).Scan(ctx, &definition))
	assert.Contains(t, definition, "auth.account_permissions", "failed migration rolls back earlier function replacements")
	_, err = db.ExecContext(ctx, `DROP VIEW auth.direct_permission_dependency_probe`)
	require.NoError(t, err)
	require.NoError(t, retireDirectPermissionsUp(ctx, db), "the migration retries after the dependency is resolved")
}
