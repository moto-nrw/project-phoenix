package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupRolePermissionRecords removes role-permission mappings directly
func cleanupRolePermissionRecords(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	if len(ids) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup role_permissions: %v", err)
	}
}

// rolePermissionTestData holds test fixtures
type rolePermissionTestData struct {
	Role       *auth.Role
	Permission *auth.Permission
}

// createRolePermissionTestData creates test fixtures
func createRolePermissionTestData(t *testing.T, db *bun.DB) *rolePermissionTestData {
	t.Helper()
	ctx := context.Background()

	repoFactory := repositories.NewFactory(db)
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create a test role using the repository
	role := &auth.Role{
		Name:        fmt.Sprintf("test_role_for_mapping_%s", uniqueSuffix),
		Description: "Test role for role-permission mapping tests",
	}
	err := repoFactory.Role.Create(ctx, role)
	require.NoError(t, err)

	// Create a test permission using the repository
	permission := &auth.Permission{
		Name:        fmt.Sprintf("test_permission_for_mapping_%s", uniqueSuffix),
		Description: "Test permission for role-permission mapping tests",
		Resource:    "test_resource",
		Action:      "read",
	}
	err = repoFactory.Permission.Create(ctx, permission)
	require.NoError(t, err)

	return &rolePermissionTestData{
		Role:       role,
		Permission: permission,
	}
}

// cleanupRolePermissionTestData removes test data
func cleanupRolePermissionTestData(t *testing.T, db *bun.DB, data *rolePermissionTestData) {
	t.Helper()
	ctx := context.Background()

	repoFactory := repositories.NewFactory(db)

	if data.Permission != nil {
		_ = repoFactory.Permission.Delete(ctx, data.Permission.ID)
	}
	if data.Role != nil {
		_ = repoFactory.Role.Delete(ctx, data.Role.ID)
	}
}

// ============================================================================
// List Tests
// ============================================================================

func TestRolePermissionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()
	data := createRolePermissionTestData(t, db)
	defer cleanupRolePermissionTestData(t, db, data)

	t.Run("lists all role-permissions with nil filters", func(t *testing.T) {
		// Create a test role-permission mapping
		rolePermission := &auth.RolePermission{
			RoleID:       data.Role.ID,
			PermissionID: data.Permission.ID,
		}
		err := repo.Create(ctx, rolePermission)
		require.NoError(t, err)
		defer cleanupRolePermissionRecords(t, db, rolePermission.ID)

		rolePermissions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, rolePermissions)
	})

	t.Run("lists with role_id filter", func(t *testing.T) {
		// Create a test role-permission mapping
		rolePermission := &auth.RolePermission{
			RoleID:       data.Role.ID,
			PermissionID: data.Permission.ID,
		}
		err := repo.Create(ctx, rolePermission)
		require.NoError(t, err)
		defer cleanupRolePermissionRecords(t, db, rolePermission.ID)

		filters := map[string]interface{}{
			"role_id": data.Role.ID,
		}
		rolePermissions, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rolePermissions)

		// All results should have the same role_id
		for _, rp := range rolePermissions {
			assert.Equal(t, data.Role.ID, rp.RoleID)
		}
	})

	t.Run("lists with nil filter value", func(t *testing.T) {
		filters := map[string]interface{}{
			"role_id": nil,
		}
		rolePermissions, err := repo.List(ctx, filters)
		require.NoError(t, err)
		// Should return results (nil filter ignored)
		assert.NotNil(t, rolePermissions)
	})
}
