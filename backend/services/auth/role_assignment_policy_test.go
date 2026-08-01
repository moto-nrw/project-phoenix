package auth_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The shared policy behind every path that attaches an account to a school
// (issue #1021): operator-led school access and /auth/link-to-tenant.
func TestValidateAssignableSchoolRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	roleRepo := repoFactory.Role
	ctx := context.Background()

	const homeTenantID int64 = 1
	const foreignTenantID int64 = 1021002
	testpkg.EnsureTestTenant(t, db, foreignTenantID)

	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")
	guardianRole := testpkg.GetOrCreateTestRole(t, db, "guardian")
	tenantRole := testpkg.CreateTestRoleForTenant(t, db, "zugriff-policy-rolle", homeTenantID)
	defer testpkg.CleanupRoleRecords(t, db, tenantRole.ID)

	t.Run("accepts a platform system role", func(t *testing.T) {
		role, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, adminRole.ID, homeTenantID)
		require.NoError(t, err)
		assert.Equal(t, adminRole.ID, role.ID)
	})

	t.Run("accepts a custom role of the same school", func(t *testing.T) {
		role, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, tenantRole.ID, homeTenantID)
		require.NoError(t, err)
		assert.Equal(t, tenantRole.ID, role.ID)
	})

	t.Run("rejects a custom role of another school", func(t *testing.T) {
		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, tenantRole.ID, foreignTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleForeignTenant)
	})

	t.Run("rejects the guardian role", func(t *testing.T) {
		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, guardianRole.ID, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleGuardianNotAssignable)
	})

	t.Run("rejects a custom role derived from guardian", func(t *testing.T) {
		guardianBase := authModels.BaseRoleGuardian
		derivedRole := testpkg.CreateTestRoleForTenant(t, db, "zugriff-policy-guardian-base", homeTenantID)
		derivedRole.BaseRole = &guardianBase
		_, updateErr := db.NewRaw(`UPDATE auth.roles SET base_role = ? WHERE id = ?`, guardianBase, derivedRole.ID).Exec(ctx)
		require.NoError(t, updateErr)
		defer testpkg.CleanupRoleRecords(t, db, derivedRole.ID)

		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, derivedRole.ID, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleGuardianNotAssignable)
	})

	t.Run("accepts a custom role named guardian without guardian base role", func(t *testing.T) {
		roleID := createTenantRole(t, db, "guardian", homeTenantID, nil)
		defer testpkg.CleanupRoleRecords(t, db, roleID)

		role, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, roleID, homeTenantID)
		require.NoError(t, err)
		assert.Equal(t, roleID, role.ID)
	})

	t.Run("rejects the retired teacher role", func(t *testing.T) {
		// The fixtures suffix role names to keep them unique, so the legacy
		// role has to be inserted under its exact historical name.
		teacherRoleID := createPlatformRole(t, db, "teacher", true)
		defer testpkg.CleanupRoleRecords(t, db, teacherRoleID)

		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, teacherRoleID, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleLegacyTeacherNotAssignable)
	})

	t.Run("accepts a custom role named teacher", func(t *testing.T) {
		roleID := createTenantRole(t, db, "teacher", homeTenantID, nil)
		defer testpkg.CleanupRoleRecords(t, db, roleID)

		role, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, roleID, homeTenantID)
		require.NoError(t, err)
		assert.Equal(t, roleID, role.ID)
	})

	t.Run("rejects a platform-wide role that is not a system role", func(t *testing.T) {
		strayRole := createPlatformRole(t, db, "zugriff-policy-stray-"+strconv.FormatInt(time.Now().UnixNano(), 10), false)
		defer testpkg.CleanupRoleRecords(t, db, strayRole)

		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, strayRole, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleNotAssignable)
	})

	t.Run("rejects a missing role", func(t *testing.T) {
		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, 0, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleNotAssignable)
	})

	t.Run("rejects an unknown role id", func(t *testing.T) {
		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, 99999999, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleNotAssignable)
	})
}

// createPlatformRole inserts a role without tenant_id under the exact name
// given. The shared fixtures suffix names to keep them unique, which does not
// work here: the policy decides by name (legacy "teacher") and by the
// is_system flag, so both have to be controlled precisely.
func createPlatformRole(t *testing.T, db *bun.DB, name string, isSystem bool) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(
		`INSERT INTO auth.roles (name, description, is_system, tenant_id, created_at, updated_at)
		 VALUES (?, 'policy test role', ?, NULL, NOW(), NOW()) RETURNING id`,
		name, isSystem,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

func createTenantRole(t *testing.T, db *bun.DB, name string, tenantID int64, baseRole *string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(
		`INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
		 VALUES (?, 'policy test role', FALSE, ?, ?, NOW(), NOW()) RETURNING id`,
		name, tenantID, baseRole,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}
