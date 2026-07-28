package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
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

	t.Run("rejects a missing role", func(t *testing.T) {
		_, err := authSvc.ValidateAssignableSchoolRole(ctx, roleRepo, 0, homeTenantID)
		assert.ErrorIs(t, err, authSvc.ErrRoleNotAssignable)
	})
}
