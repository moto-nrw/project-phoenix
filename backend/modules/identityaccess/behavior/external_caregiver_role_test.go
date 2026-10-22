package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Every school has one school-owned role „Extern“ for external caregivers
// with an account (#4573). Identity & Access provisions it and finds it by
// name; these tests drive that capability as the roots compose it.

func TestEnsureExternalCaregiverRoleCreatesItOnceWithDefaultPermissions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	_, found, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	require.False(t, found, "a school starts without the role until it is provisioned")

	_, err = identity.EnsureExternalCaregiverRole(ctx, tenantID)
	require.Error(t, err, "the role and its permissions commit with the caller's transaction only")
	assert.Equal(t, 0, externRoleCount(t, db, tenantID))

	role, err := ensureExternRole(t, db, identity, ctx, tenantID, tenantID)
	require.NoError(t, err)
	assert.True(t, identityaccess.IsExternalCaregiverRoleName(role.Name), "got role name %q", role.Name)
	require.NotNil(t, role.TenantID, "the role belongs to the school")
	assert.Equal(t, tenantID, *role.TenantID)
	assert.False(t, role.IsSystem, "the school's leadership must be able to edit the role")
	require.NotNil(t, role.BaseRole)
	assert.Equal(t, identityaccess.BaseRoleUser, *role.BaseRole)
	assert.Equal(t, identityaccess.ExternalCaregiverRoleDescription, role.Description)
	assert.ElementsMatch(t, identityaccess.ExternalCaregiverDefaultPermissions, rolePermissionNames(t, identity, ctx, role.ID))

	again, err := ensureExternRole(t, db, identity, ctx, tenantID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, role.ID, again.ID, "a second call returns the existing role")
	assert.Equal(t, 1, externRoleCount(t, db, tenantID))
	assert.ElementsMatch(t, identityaccess.ExternalCaregiverDefaultPermissions, rolePermissionNames(t, identity, ctx, role.ID))

	resolved, found, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, role.ID, resolved.ID)
}

func TestEnsureExternalCaregiverRoleKeepsTheSchoolsOwnRoleOfThatName(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	// A school that already named a role „ extern “ keeps it as it is,
	// permissions included: Ensure never changes an existing role.
	var existingID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role)
		VALUES (?, ' extern ', 'eigene Rolle', FALSE, 'user') RETURNING id`, tenantID).Scan(ctx, &existingID))
	groupsRead, err := identity.GetPermissionByName(ctx, "groups:read")
	require.NoError(t, err)
	require.NoError(t, identity.ReplaceRolePermissions(ctx, existingID, []int64{groupsRead.ID}))

	role, err := ensureExternRole(t, db, identity, ctx, tenantID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, existingID, role.ID)
	assert.Equal(t, "eigene Rolle", role.Description)
	assert.Equal(t, 1, externRoleCount(t, db, tenantID))
	assert.Equal(t, []string{"groups:read"}, rolePermissionNames(t, identity, ctx, existingID))

	resolved, found, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	require.True(t, found, "the name match is case-insensitive and trimmed")
	assert.Equal(t, existingID, resolved.ID)
}

func TestFindExternalCaregiverRoleRequiresTheUserBaseRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	_, err := db.NewRaw(`INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role)
		VALUES (?, 'extern', '', FALSE, 'admin')`, tenantID).Exec(ctx)
	require.NoError(t, err)

	_, found, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	assert.False(t, found, "a same-named role of another tier is not the external caregiver role")

	role, err := ensureExternRole(t, db, identity, ctx, tenantID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, role.BaseRole)
	assert.Equal(t, "admin", *role.BaseRole, "Ensure leaves the school's own role untouched")
	assert.Equal(t, 1, externRoleCount(t, db, tenantID))
}

func TestExternalCaregiverRoleIsTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	testpkg.OwnTenantRows(t, db, other)
	otherCtx := testpkg.ContextForTenant(ctx, other)

	foreign, err := ensureExternRole(t, db, identity, otherCtx, other, other)
	require.NoError(t, err)

	_, found, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	assert.False(t, found, "the role of another school is invisible")

	_, err = ensureExternRole(t, db, identity, ctx, tenantID, other)
	require.Error(t, err, "a context scoped to one school cannot provision another")

	own, err := ensureExternRole(t, db, identity, ctx, tenantID, tenantID)
	require.NoError(t, err)
	assert.NotEqual(t, foreign.ID, own.ID, "every school gets its own role")
	assert.Equal(t, 1, externRoleCount(t, db, tenantID))
	assert.Equal(t, 1, externRoleCount(t, db, other))

	resolved, found, err := identity.FindExternalCaregiverRole(otherCtx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, foreign.ID, resolved.ID)

	_, _, err = identity.FindExternalCaregiverRole(context.Background())
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
}

// ensureExternRole runs Ensure the way its callers do: inside the caller's
// transaction, here the one of school txTenant.
func ensureExternRole(t *testing.T, db *bun.DB, identity identityaccess.Owner, ctx context.Context, txTenant, tenantID int64) (identityaccess.Role, error) {
	t.Helper()
	var role identityaccess.Role
	err := testpkg.WithinTenantContext(t, ctx, db, txTenant, func(txCtx context.Context) error {
		var err error
		role, err = identity.EnsureExternalCaregiverRole(txCtx, tenantID)
		return err
	})
	return role, err
}

func rolePermissionNames(t *testing.T, identity identityaccess.Owner, ctx context.Context, roleID int64) []string {
	t.Helper()
	permissions, err := identity.GetRolePermissions(ctx, roleID)
	require.NoError(t, err)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}
	return names
}

// externRoleCount counts the school's roles named „Extern“ with the
// predicate the migration uses.
func externRoleCount(t *testing.T, db *bun.DB, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr("auth.roles").
		Where("tenant_id = ?", tenantID).
		Where("LOWER(TRIM(name)) = 'extern'").
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}
