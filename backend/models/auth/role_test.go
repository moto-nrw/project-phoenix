package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		role    *Role
		wantErr bool
	}{
		{
			name: "valid role",
			role: &Role{
				Name: "admin",
			},
			wantErr: false,
		},
		{
			name: "valid role with description",
			role: &Role{
				Name:        "teacher",
				Description: "Teacher role with classroom access",
			},
			wantErr: false,
		},
		{
			name: "valid system role",
			role: &Role{
				Name:     "superadmin",
				IsSystem: true,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			role: &Role{
				Name: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Role.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRole_Validate_Normalization(t *testing.T) {
	tests := []struct {
		name         string
		inputName    string
		expectedName string
	}{
		{
			name:         "uppercase to lowercase",
			inputName:    "ADMIN",
			expectedName: "admin",
		},
		{
			name:         "mixed case to lowercase",
			inputName:    "TeAcHeR",
			expectedName: "teacher",
		},
		{
			name:         "already lowercase",
			inputName:    "student",
			expectedName: "student",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := &Role{Name: tt.inputName}

			err := role.Validate()
			if err != nil {
				t.Fatalf("Role.Validate() unexpected error = %v", err)
			}

			if role.Name != tt.expectedName {
				t.Errorf("Role.Validate() name = %q, want %q", role.Name, tt.expectedName)
			}
		})
	}
}

func TestRole_HasPermission(t *testing.T) {
	readPerm := &Permission{
		Model: base.Model{ID: 1},
		Name:  "read",
	}
	writePerm := &Permission{
		Model: base.Model{ID: 2},
		Name:  "write",
	}

	tests := []struct {
		name       string
		role       *Role
		permission string
		expected   bool
	}{
		{
			name: "has permission",
			role: &Role{
				Name:        "admin",
				Permissions: []*Permission{readPerm, writePerm},
			},
			permission: "read",
			expected:   true,
		},
		{
			name: "has permission - case insensitive",
			role: &Role{
				Name:        "admin",
				Permissions: []*Permission{readPerm},
			},
			permission: "READ",
			expected:   true,
		},
		{
			name: "does not have permission",
			role: &Role{
				Name:        "viewer",
				Permissions: []*Permission{readPerm},
			},
			permission: "write",
			expected:   false,
		},
		{
			name: "nil permissions",
			role: &Role{
				Name:        "empty",
				Permissions: nil,
			},
			permission: "read",
			expected:   false,
		},
		{
			name: "empty permissions",
			role: &Role{
				Name:        "empty",
				Permissions: []*Permission{},
			},
			permission: "read",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.HasPermission(tt.permission)
			if got != tt.expected {
				t.Errorf("Role.HasPermission(%q) = %v, want %v", tt.permission, got, tt.expected)
			}
		})
	}
}

func TestRole_AddPermission(t *testing.T) {
	t.Run("add permission to nil slice", func(t *testing.T) {
		role := &Role{
			Name:        "test",
			Permissions: nil,
		}

		perm := &Permission{
			Model: base.Model{ID: 1},
			Name:  "read",
		}

		role.AddPermission(perm)

		if len(role.Permissions) != 1 {
			t.Errorf("Role.AddPermission() permissions count = %d, want 1", len(role.Permissions))
		}

		if role.Permissions[0] != perm {
			t.Error("Role.AddPermission() did not add the permission")
		}
	})

	t.Run("add permission to empty slice", func(t *testing.T) {
		role := &Role{
			Name:        "test",
			Permissions: []*Permission{},
		}

		perm := &Permission{
			Model: base.Model{ID: 1},
			Name:  "read",
		}

		role.AddPermission(perm)

		if len(role.Permissions) != 1 {
			t.Errorf("Role.AddPermission() permissions count = %d, want 1", len(role.Permissions))
		}
	})

	t.Run("add multiple permissions", func(t *testing.T) {
		role := &Role{
			Name: "test",
		}

		perm1 := &Permission{Model: base.Model{ID: 1}, Name: "read"}
		perm2 := &Permission{Model: base.Model{ID: 2}, Name: "write"}

		role.AddPermission(perm1)
		role.AddPermission(perm2)

		if len(role.Permissions) != 2 {
			t.Errorf("Role.AddPermission() permissions count = %d, want 2", len(role.Permissions))
		}
	})

	t.Run("duplicate permission not added", func(t *testing.T) {
		perm := &Permission{
			Model: base.Model{ID: 1},
			Name:  "read",
		}

		role := &Role{
			Name:        "test",
			Permissions: []*Permission{perm},
		}

		role.AddPermission(perm) // Add same permission again

		if len(role.Permissions) != 1 {
			t.Errorf("Role.AddPermission() should not add duplicate, count = %d, want 1", len(role.Permissions))
		}
	})
}

func TestRole_RemovePermission(t *testing.T) {
	t.Run("remove existing permission", func(t *testing.T) {
		perm1 := &Permission{Model: base.Model{ID: 1}, Name: "read"}
		perm2 := &Permission{Model: base.Model{ID: 2}, Name: "write"}

		role := &Role{
			Name:        "test",
			Permissions: []*Permission{perm1, perm2},
		}

		removed := role.RemovePermission(1)

		if !removed {
			t.Error("Role.RemovePermission() should return true when permission is removed")
		}

		if len(role.Permissions) != 1 {
			t.Errorf("Role.RemovePermission() permissions count = %d, want 1", len(role.Permissions))
		}

		if role.Permissions[0].ID != 2 {
			t.Error("Role.RemovePermission() removed wrong permission")
		}
	})

	t.Run("remove non-existing permission", func(t *testing.T) {
		perm := &Permission{Model: base.Model{ID: 1}, Name: "read"}

		role := &Role{
			Name:        "test",
			Permissions: []*Permission{perm},
		}

		removed := role.RemovePermission(999)

		if removed {
			t.Error("Role.RemovePermission() should return false when permission not found")
		}

		if len(role.Permissions) != 1 {
			t.Errorf("Role.RemovePermission() should not change count, got %d", len(role.Permissions))
		}
	})

	t.Run("remove from nil permissions", func(t *testing.T) {
		role := &Role{
			Name:        "test",
			Permissions: nil,
		}

		removed := role.RemovePermission(1)

		if removed {
			t.Error("Role.RemovePermission() should return false when permissions is nil")
		}
	})

	t.Run("remove from empty permissions", func(t *testing.T) {
		role := &Role{
			Name:        "test",
			Permissions: []*Permission{},
		}

		removed := role.RemovePermission(1)

		if removed {
			t.Error("Role.RemovePermission() should return false when permissions is empty")
		}
	})
}

func TestRole_TableName(t *testing.T) {
	role := &Role{}
	expected := "auth.roles"

	got := role.TableName()
	if got != expected {
		t.Errorf("Role.TableName() = %q, want %q", got, expected)
	}
}

func TestRole_EntityInterface(t *testing.T) {
	now := time.Now()
	role := &Role{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Name: "admin",
	}

	t.Run("GetID", func(t *testing.T) {
		got := role.GetID()
		if got != int64(123) {
			t.Errorf("Role.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := role.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Role.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := role.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Role.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}

func TestRole_IsSystemFlag(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		role := &Role{
			Name: "custom",
		}

		if role.IsSystem != false {
			t.Error("Role.IsSystem should default to false")
		}
	})

	t.Run("can be set to true", func(t *testing.T) {
		role := &Role{
			Name:     "superadmin",
			IsSystem: true,
		}

		if role.IsSystem != true {
			t.Error("Role.IsSystem should be true when set")
		}
	})
}
