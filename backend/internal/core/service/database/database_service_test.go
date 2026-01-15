package database_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/internal/core/port/permissions"
	databaseSvc "github.com/moto-nrw/project-phoenix/internal/core/service/database"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDatabaseService creates a database service with real database connection.
func setupDatabaseService(t *testing.T) (*repositories.Factory, databaseSvc.DatabaseService) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repoFactory := repositories.NewFactory(db)

	return repoFactory, databaseSvc.NewService(databaseSvc.Repositories{
		Student:       repoFactory.Student,
		Teacher:       repoFactory.Teacher,
		Room:          repoFactory.Room,
		ActivityGroup: repoFactory.ActivityGroup,
		Group:         repoFactory.Group,
		Role:          repoFactory.Role,
		Device:        repoFactory.Device,
		Permission:    repoFactory.Permission,
	})
}

// contextWithPermissions creates a context with JWT claims containing permissions
func contextWithPermissions(userID int, perms ...string) context.Context {
	claims := port.AppClaims{
		ID:          userID,
		Permissions: perms,
	}
	return context.WithValue(context.Background(), port.CtxClaims, claims)
}

// ============================================================================
// GetStats Tests
// ============================================================================

func TestDatabaseService_GetStats(t *testing.T) {
	_, service := setupDatabaseService(t)

	t.Run("returns stats for admin user", func(t *testing.T) {
		// ARRANGE - Admin has full access
		ctx := contextWithPermissions(1, permissions.AdminWildcard)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Admin should see all stats
		assert.True(t, result.Permissions.CanViewStudents)
		assert.True(t, result.Permissions.CanViewTeachers)
		assert.True(t, result.Permissions.CanViewRooms)
		assert.True(t, result.Permissions.CanViewActivities)
		assert.True(t, result.Permissions.CanViewGroups)
		assert.True(t, result.Permissions.CanViewRoles)
		assert.True(t, result.Permissions.CanViewDevices)
		assert.True(t, result.Permissions.CanViewPermissions)
	})

	t.Run("returns stats for user with full access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.FullAccess)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// FullAccess should see all stats
		assert.True(t, result.Permissions.CanViewStudents)
		assert.True(t, result.Permissions.CanViewTeachers)
	})

	t.Run("returns limited stats for user with users permission only", func(t *testing.T) {
		// ARRANGE - User can only view users
		ctx := contextWithPermissions(1, permissions.UsersRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see students and teachers
		assert.True(t, result.Permissions.CanViewStudents)
		assert.True(t, result.Permissions.CanViewTeachers)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewRooms)
		assert.False(t, result.Permissions.CanViewActivities)
		assert.False(t, result.Permissions.CanViewGroups)
		assert.False(t, result.Permissions.CanViewRoles)
		assert.False(t, result.Permissions.CanViewDevices)
		assert.False(t, result.Permissions.CanViewPermissions)
	})

	t.Run("returns limited stats for user with rooms permission only", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.RoomsRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see rooms
		assert.True(t, result.Permissions.CanViewRooms)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewStudents)
		assert.False(t, result.Permissions.CanViewTeachers)
	})

	t.Run("returns limited stats for user with activities permission only", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.ActivitiesRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see activities
		assert.True(t, result.Permissions.CanViewActivities)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewRooms)
	})

	t.Run("returns limited stats for user with groups permission only", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.GroupsRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see groups
		assert.True(t, result.Permissions.CanViewGroups)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewActivities)
	})

	t.Run("returns limited stats for user with auth manage permission", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.AuthManage)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see roles and permissions
		assert.True(t, result.Permissions.CanViewRoles)
		assert.True(t, result.Permissions.CanViewPermissions)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewStudents)
	})

	t.Run("returns limited stats for user with IOT permission only", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.IOTRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see devices
		assert.True(t, result.Permissions.CanViewDevices)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewRoles)
	})

	t.Run("returns no stats for user without permissions", func(t *testing.T) {
		// ARRANGE - No permissions
		ctx := contextWithPermissions(1)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should not see any stats
		assert.False(t, result.Permissions.CanViewStudents)
		assert.False(t, result.Permissions.CanViewTeachers)
		assert.False(t, result.Permissions.CanViewRooms)
		assert.False(t, result.Permissions.CanViewActivities)
		assert.False(t, result.Permissions.CanViewGroups)
		assert.False(t, result.Permissions.CanViewRoles)
		assert.False(t, result.Permissions.CanViewDevices)
		assert.False(t, result.Permissions.CanViewPermissions)

		// Counts should be zero
		assert.Equal(t, 0, result.Students)
		assert.Equal(t, 0, result.Teachers)
		assert.Equal(t, 0, result.Rooms)
		assert.Equal(t, 0, result.Activities)
		assert.Equal(t, 0, result.Groups)
		assert.Equal(t, 0, result.Roles)
		assert.Equal(t, 0, result.Devices)
		assert.Equal(t, 0, result.PermissionCount)
	})

	t.Run("returns stats with multiple specific permissions", func(t *testing.T) {
		// ARRANGE - User has users and rooms permissions
		ctx := contextWithPermissions(1, permissions.UsersRead, permissions.RoomsRead, permissions.GroupsRead)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should see users, rooms, and groups
		assert.True(t, result.Permissions.CanViewStudents)
		assert.True(t, result.Permissions.CanViewTeachers)
		assert.True(t, result.Permissions.CanViewRooms)
		assert.True(t, result.Permissions.CanViewGroups)

		// Should NOT see other stats
		assert.False(t, result.Permissions.CanViewActivities)
		assert.False(t, result.Permissions.CanViewRoles)
		assert.False(t, result.Permissions.CanViewDevices)
		assert.False(t, result.Permissions.CanViewPermissions)
	})

	t.Run("works with empty context", func(t *testing.T) {
		// ARRANGE - Empty context without claims
		ctx := context.Background()

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// All permissions should be false
		assert.False(t, result.Permissions.CanViewStudents)
	})
}

// ============================================================================
// Permission Checking Tests
// ============================================================================

func TestDatabaseService_PermissionChecks(t *testing.T) {
	_, service := setupDatabaseService(t)

	t.Run("users list permission grants student access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.UsersList)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.Permissions.CanViewStudents)
		assert.True(t, result.Permissions.CanViewTeachers)
	})

	t.Run("rooms list permission grants room access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.RoomsList)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.Permissions.CanViewRooms)
	})

	t.Run("activities list permission grants activity access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.ActivitiesList)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.Permissions.CanViewActivities)
	})

	t.Run("groups list permission grants group access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.GroupsList)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.Permissions.CanViewGroups)
	})

	t.Run("iot manage permission grants device access", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithPermissions(1, permissions.IOTManage)

		// ACT
		result, err := service.GetStats(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.Permissions.CanViewDevices)
	})
}
