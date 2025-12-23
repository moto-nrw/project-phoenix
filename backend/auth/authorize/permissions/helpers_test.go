package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllResourcePermissions(t *testing.T) {
	tests := []struct {
		name                  string
		resource              string
		expectedCount         int
		expectedContains      []string
		expectedNotContains   []string
	}{
		{
			name:          "users resource - standard actions only",
			resource:      ResourceUsers,
			expectedCount: 6, // create, read, update, delete, list, manage
			expectedContains: []string{
				UsersCreate, UsersRead, UsersUpdate, UsersDelete, UsersList, UsersManage,
			},
			expectedNotContains: []string{
				"users:enroll", "users:assign",
			},
		},
		{
			name:          "activities resource - includes special actions",
			resource:      ResourceActivities,
			expectedCount: 8, // 6 standard + enroll, assign
			expectedContains: []string{
				ActivitiesCreate, ActivitiesRead, ActivitiesUpdate,
				ActivitiesDelete, ActivitiesList, ActivitiesManage,
				ActivitiesEnroll, ActivitiesAssign,
			},
		},
		{
			name:          "groups resource - includes assign",
			resource:      ResourceGroups,
			expectedCount: 7, // 6 standard + assign
			expectedContains: []string{
				GroupsCreate, GroupsRead, GroupsUpdate,
				GroupsDelete, GroupsList, GroupsManage,
				GroupsAssign,
			},
		},
		{
			name:          "rooms resource - standard only",
			resource:      ResourceRooms,
			expectedCount: 6,
			expectedContains: []string{
				RoomsCreate, RoomsRead, RoomsUpdate, RoomsDelete, RoomsList, RoomsManage,
			},
		},
		{
			name:          "custom resource - standard actions",
			resource:      "custom",
			expectedCount: 6,
			expectedContains: []string{
				"custom:create", "custom:read", "custom:update",
				"custom:delete", "custom:list", "custom:manage",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAllResourcePermissions(tt.resource)

			assert.Len(t, result, tt.expectedCount)

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}

			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, result, notExpected)
			}
		})
	}
}

func TestGetAdminPermissions(t *testing.T) {
	perms := GetAdminPermissions()

	require.Len(t, perms, 1)
	assert.Equal(t, AdminWildcard, perms[0])
}

func TestIsAdminPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		expected   bool
	}{
		{"admin wildcard", AdminWildcard, true},
		{"full access", FullAccess, true},
		{"regular permission", UsersRead, false},
		{"empty string", "", false},
		{"partial match", "admin", false},
		{"wrong format", "admin:read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAdminPermission(tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsResourcePermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		resource   string
		expected   bool
	}{
		{"exact match", UsersRead, ResourceUsers, true},
		{"different resource", UsersRead, ResourceRooms, false},
		{"wildcard resource", "*:read", ResourceUsers, true},
		{"wildcard action", "users:*", ResourceUsers, true},
		{"invalid format - no colon", "users", ResourceUsers, false},
		{"invalid format - too many colons", "users:read:extra", ResourceUsers, false},
		{"empty permission", "", ResourceUsers, false},
		{"empty resource match", "users:read", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResourcePermission(tt.permission, tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsActionPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		action     string
		expected   bool
	}{
		{"exact match read", UsersRead, ActionRead, true},
		{"exact match create", UsersCreate, ActionCreate, true},
		{"different action", UsersRead, ActionUpdate, false},
		{"wildcard action", "users:*", ActionRead, true},
		{"invalid format - no colon", "read", ActionRead, false},
		{"invalid format - too many colons", "users:read:extra", ActionRead, false},
		{"empty permission", "", ActionRead, false},
		{"empty action match", "users:read", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsActionPermission(tt.permission, tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStandardRolePermissions(t *testing.T) {
	roles := GetStandardRolePermissions()

	// Test admin role
	t.Run("admin role has wildcard", func(t *testing.T) {
		admin, ok := roles["admin"]
		require.True(t, ok)
		assert.Contains(t, admin, AdminWildcard)
	})

	// Test user_manager role
	t.Run("user_manager has all user permissions", func(t *testing.T) {
		manager, ok := roles["user_manager"]
		require.True(t, ok)
		assert.Contains(t, manager, UsersCreate)
		assert.Contains(t, manager, UsersRead)
		assert.Contains(t, manager, UsersUpdate)
		assert.Contains(t, manager, UsersDelete)
		assert.Contains(t, manager, UsersList)
		assert.Contains(t, manager, UsersManage)
	})

	// Test activity_manager role
	t.Run("activity_manager has activity and special permissions", func(t *testing.T) {
		manager, ok := roles["activity_manager"]
		require.True(t, ok)
		assert.Contains(t, manager, ActivitiesCreate)
		assert.Contains(t, manager, ActivitiesManage)
		assert.Contains(t, manager, ActivitiesEnroll)
		assert.Contains(t, manager, ActivitiesAssign)
	})

	// Test viewer roles
	t.Run("user_viewer has only read and list", func(t *testing.T) {
		viewer, ok := roles["user_viewer"]
		require.True(t, ok)
		assert.Contains(t, viewer, UsersRead)
		assert.Contains(t, viewer, UsersList)
		assert.NotContains(t, viewer, UsersCreate)
		assert.NotContains(t, viewer, UsersDelete)
	})

	// Test all expected roles exist
	t.Run("all standard roles exist", func(t *testing.T) {
		expectedRoles := []string{
			"admin", "user_manager", "activity_manager", "room_manager",
			"group_manager", "feedback_manager", "config_manager", "iot_manager",
			"user_viewer", "activity_viewer", "room_viewer", "group_viewer", "feedback_viewer",
		}

		for _, role := range expectedRoles {
			_, ok := roles[role]
			assert.True(t, ok, "role %s should exist", role)
		}
	})
}

func TestHasPermissionForResource(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		resource    string
		expected    bool
	}{
		{
			name:        "has direct permission",
			permissions: []string{UsersRead, GroupsRead},
			resource:    ResourceUsers,
			expected:    true,
		},
		{
			name:        "has admin wildcard",
			permissions: []string{AdminWildcard},
			resource:    ResourceUsers,
			expected:    true,
		},
		{
			name:        "has full access",
			permissions: []string{FullAccess},
			resource:    ResourceUsers,
			expected:    true,
		},
		{
			name:        "no matching permission",
			permissions: []string{GroupsRead, RoomsRead},
			resource:    ResourceUsers,
			expected:    false,
		},
		{
			name:        "empty permissions",
			permissions: []string{},
			resource:    ResourceUsers,
			expected:    false,
		},
		{
			name:        "nil permissions",
			permissions: nil,
			resource:    ResourceUsers,
			expected:    false,
		},
		{
			name:        "wildcard resource permission",
			permissions: []string{"*:read"},
			resource:    ResourceUsers,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPermissionForResource(tt.permissions, tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterPermissionsByResource(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		resource    string
		expected    []string
	}{
		{
			name:        "filters to matching resource",
			permissions: []string{UsersRead, UsersUpdate, GroupsRead, RoomsRead},
			resource:    ResourceUsers,
			expected:    []string{UsersRead, UsersUpdate},
		},
		{
			name:        "includes admin wildcards",
			permissions: []string{UsersRead, AdminWildcard, GroupsRead},
			resource:    ResourceUsers,
			expected:    []string{UsersRead, AdminWildcard},
		},
		{
			name:        "includes full access",
			permissions: []string{FullAccess, GroupsRead},
			resource:    ResourceUsers,
			// BUG: FullAccess is added twice - see issue #422
			expected:    []string{FullAccess, FullAccess},
		},
		{
			name:        "no matching permissions",
			permissions: []string{GroupsRead, RoomsRead},
			resource:    ResourceUsers,
			expected:    nil,
		},
		{
			name:        "empty input",
			permissions: []string{},
			resource:    ResourceUsers,
			expected:    nil,
		},
		{
			name:        "wildcard resource",
			permissions: []string{"*:read", GroupsRead},
			resource:    ResourceUsers,
			expected:    []string{"*:read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterPermissionsByResource(tt.permissions, tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}
