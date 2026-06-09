package authorize

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Moved from models/auth/permission_test.go (TestPermission_IsAdminPermission)
// when the admin classification left the model in issue #586 (Rule 12).
func TestPermissionIsAdmin(t *testing.T) {
	tests := []struct {
		name       string
		permission *auth.Permission
		expected   bool
	}{
		{
			name:       "admin resource",
			permission: &auth.Permission{Name: "admin_manage", Resource: "admin", Action: "manage"},
			expected:   true,
		},
		{
			name:       "admin prefixed name",
			permission: &auth.Permission{Name: "admin:users", Resource: "users", Action: "manage"},
			expected:   true,
		},
		{
			name:       "non-admin permission",
			permission: &auth.Permission{Name: "users_read", Resource: "users", Action: "read"},
			expected:   false,
		},
		{
			name:       "contains admin but not prefix",
			permission: &auth.Permission{Name: "users_admin_view", Resource: "users", Action: "admin_view"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PermissionIsAdmin(tt.permission); got != tt.expected {
				t.Errorf("PermissionIsAdmin() = %v, want %v", got, tt.expected)
			}
		})
	}

	if PermissionIsAdmin(nil) {
		t.Error("PermissionIsAdmin(nil) should return false")
	}
}

// Moved from models/auth/role_test.go (TestRole_HasPermission).
func TestRoleHasPermission(t *testing.T) {
	readPerm := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}
	writePerm := &auth.Permission{Model: base.Model{ID: 2}, Name: "write"}

	tests := []struct {
		name       string
		role       *auth.Role
		permission string
		expected   bool
	}{
		{
			name:       "has permission",
			role:       &auth.Role{Name: "admin", Permissions: []*auth.Permission{readPerm, writePerm}},
			permission: "read",
			expected:   true,
		},
		{
			name:       "has permission - case insensitive",
			role:       &auth.Role{Name: "admin", Permissions: []*auth.Permission{readPerm}},
			permission: "READ",
			expected:   true,
		},
		{
			name:       "does not have permission",
			role:       &auth.Role{Name: "viewer", Permissions: []*auth.Permission{readPerm}},
			permission: "write",
			expected:   false,
		},
		{
			name:       "nil permissions",
			role:       &auth.Role{Name: "empty", Permissions: nil},
			permission: "read",
			expected:   false,
		},
		{
			name:       "empty permissions",
			role:       &auth.Role{Name: "empty", Permissions: []*auth.Permission{}},
			permission: "read",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoleHasPermission(tt.role, tt.permission); got != tt.expected {
				t.Errorf("RoleHasPermission(%q) = %v, want %v", tt.permission, got, tt.expected)
			}
		})
	}

	if RoleHasPermission(nil, "read") {
		t.Error("RoleHasPermission(nil, ...) should return false")
	}
}

// Moved from models/auth/role_test.go (TestRole_AddPermission).
func TestRoleAddPermission(t *testing.T) {
	t.Run("add permission to nil slice", func(t *testing.T) {
		role := &auth.Role{Name: "test", Permissions: nil}
		perm := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}

		RoleAddPermission(role, perm)

		if len(role.Permissions) != 1 {
			t.Errorf("RoleAddPermission() permissions count = %d, want 1", len(role.Permissions))
		}
		if role.Permissions[0] != perm {
			t.Error("RoleAddPermission() did not add the permission")
		}
	})

	t.Run("add permission to empty slice", func(t *testing.T) {
		role := &auth.Role{Name: "test", Permissions: []*auth.Permission{}}
		perm := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}

		RoleAddPermission(role, perm)

		if len(role.Permissions) != 1 {
			t.Errorf("RoleAddPermission() permissions count = %d, want 1", len(role.Permissions))
		}
	})

	t.Run("add multiple permissions", func(t *testing.T) {
		role := &auth.Role{Name: "test"}
		perm1 := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}
		perm2 := &auth.Permission{Model: base.Model{ID: 2}, Name: "write"}

		RoleAddPermission(role, perm1)
		RoleAddPermission(role, perm2)

		if len(role.Permissions) != 2 {
			t.Errorf("RoleAddPermission() permissions count = %d, want 2", len(role.Permissions))
		}
	})

	t.Run("duplicate permission not added", func(t *testing.T) {
		perm := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}
		role := &auth.Role{Name: "test", Permissions: []*auth.Permission{perm}}

		RoleAddPermission(role, perm)

		if len(role.Permissions) != 1 {
			t.Errorf("RoleAddPermission() should not add duplicate, count = %d, want 1", len(role.Permissions))
		}
	})
}

// Moved from models/auth/role_test.go (TestRole_RemovePermission).
func TestRoleRemovePermission(t *testing.T) {
	t.Run("remove existing permission", func(t *testing.T) {
		perm1 := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}
		perm2 := &auth.Permission{Model: base.Model{ID: 2}, Name: "write"}
		role := &auth.Role{Name: "test", Permissions: []*auth.Permission{perm1, perm2}}

		removed := RoleRemovePermission(role, 1)

		if !removed {
			t.Error("RoleRemovePermission() should return true when permission is removed")
		}
		if len(role.Permissions) != 1 {
			t.Errorf("RoleRemovePermission() permissions count = %d, want 1", len(role.Permissions))
		}
		if role.Permissions[0].ID != 2 {
			t.Error("RoleRemovePermission() removed wrong permission")
		}
	})

	t.Run("remove non-existing permission", func(t *testing.T) {
		perm := &auth.Permission{Model: base.Model{ID: 1}, Name: "read"}
		role := &auth.Role{Name: "test", Permissions: []*auth.Permission{perm}}

		removed := RoleRemovePermission(role, 999)

		if removed {
			t.Error("RoleRemovePermission() should return false when permission not found")
		}
		if len(role.Permissions) != 1 {
			t.Errorf("RoleRemovePermission() should not change count, got %d", len(role.Permissions))
		}
	})

	t.Run("remove from nil permissions", func(t *testing.T) {
		role := &auth.Role{Name: "test", Permissions: nil}
		if RoleRemovePermission(role, 1) {
			t.Error("RoleRemovePermission() should return false when permissions is nil")
		}
	})

	t.Run("remove from empty permissions", func(t *testing.T) {
		role := &auth.Role{Name: "test", Permissions: []*auth.Permission{}}
		if RoleRemovePermission(role, 1) {
			t.Error("RoleRemovePermission() should return false when permissions is empty")
		}
	})

	t.Run("nil role", func(t *testing.T) {
		if RoleRemovePermission(nil, 1) {
			t.Error("RoleRemovePermission(nil, ...) should return false")
		}
	})
}
