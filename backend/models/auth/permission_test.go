package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPermission_Validate(t *testing.T) {
	tests := []struct {
		name       string
		permission *Permission
		wantErr    bool
	}{
		{
			name: "valid permission",
			permission: &Permission{
				Name:     "users_read",
				Resource: "users",
				Action:   "read",
			},
			wantErr: false,
		},
		{
			name: "valid permission with description",
			permission: &Permission{
				Name:        "users_write",
				Description: "Allows writing user data",
				Resource:    "users",
				Action:      "write",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			permission: &Permission{
				Name:     "",
				Resource: "users",
				Action:   "read",
			},
			wantErr: true,
		},
		{
			name: "empty resource",
			permission: &Permission{
				Name:     "users_read",
				Resource: "",
				Action:   "read",
			},
			wantErr: true,
		},
		{
			name: "empty action",
			permission: &Permission{
				Name:     "users_read",
				Resource: "users",
				Action:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.permission.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Permission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPermission_Validate_Normalization(t *testing.T) {
	tests := []struct {
		name             string
		inputName        string
		inputResource    string
		inputAction      string
		expectedName     string
		expectedResource string
		expectedAction   string
	}{
		{
			name:             "lowercase conversion",
			inputName:        "USERS_READ",
			inputResource:    "USERS",
			inputAction:      "READ",
			expectedName:     "users_read",
			expectedResource: "users",
			expectedAction:   "read",
		},
		{
			name:             "spaces to underscores in name",
			inputName:        "users read",
			inputResource:    "users",
			inputAction:      "read",
			expectedName:     "users_read",
			expectedResource: "users",
			expectedAction:   "read",
		},
		{
			name:             "mixed case normalization",
			inputName:        "Admin Write",
			inputResource:    "Admin",
			inputAction:      "Write",
			expectedName:     "admin_write",
			expectedResource: "admin",
			expectedAction:   "write",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := &Permission{
				Name:     tt.inputName,
				Resource: tt.inputResource,
				Action:   tt.inputAction,
			}

			err := perm.Validate()
			if err != nil {
				t.Fatalf("Permission.Validate() unexpected error = %v", err)
			}

			if perm.Name != tt.expectedName {
				t.Errorf("Permission.Name = %q, want %q", perm.Name, tt.expectedName)
			}
			if perm.Resource != tt.expectedResource {
				t.Errorf("Permission.Resource = %q, want %q", perm.Resource, tt.expectedResource)
			}
			if perm.Action != tt.expectedAction {
				t.Errorf("Permission.Action = %q, want %q", perm.Action, tt.expectedAction)
			}
		})
	}
}

func TestPermission_GetFullName(t *testing.T) {
	tests := []struct {
		name       string
		permission *Permission
		expected   string
	}{
		{
			name: "standard permission",
			permission: &Permission{
				Resource: "users",
				Action:   "read",
			},
			expected: "users:read",
		},
		{
			name: "admin permission",
			permission: &Permission{
				Resource: "admin",
				Action:   "manage",
			},
			expected: "admin:manage",
		},
		{
			name: "empty fields",
			permission: &Permission{
				Resource: "",
				Action:   "",
			},
			expected: ":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.permission.GetFullName()
			if got != tt.expected {
				t.Errorf("Permission.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPermission_IsAdminPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission *Permission
		expected   bool
	}{
		{
			name: "admin resource",
			permission: &Permission{
				Name:     "admin_manage",
				Resource: "admin",
				Action:   "manage",
			},
			expected: true,
		},
		{
			name: "admin prefixed name",
			permission: &Permission{
				Name:     "admin:users",
				Resource: "users",
				Action:   "manage",
			},
			expected: true,
		},
		{
			name: "non-admin permission",
			permission: &Permission{
				Name:     "users_read",
				Resource: "users",
				Action:   "read",
			},
			expected: false,
		},
		{
			name: "contains admin but not prefix",
			permission: &Permission{
				Name:     "users_admin_view",
				Resource: "users",
				Action:   "admin_view",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.permission.IsAdminPermission()
			if got != tt.expected {
				t.Errorf("Permission.IsAdminPermission() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPermission_Clone(t *testing.T) {
	now := time.Now()
	original := &Permission{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:        "users_read",
		Description: "Read users",
		Resource:    "users",
		Action:      "read",
	}

	clone := original.Clone()

	// Verify all fields are copied
	if clone.ID != original.ID {
		t.Errorf("Clone.ID = %v, want %v", clone.ID, original.ID)
	}
	if clone.Name != original.Name {
		t.Errorf("Clone.Name = %q, want %q", clone.Name, original.Name)
	}
	if clone.Description != original.Description {
		t.Errorf("Clone.Description = %q, want %q", clone.Description, original.Description)
	}
	if clone.Resource != original.Resource {
		t.Errorf("Clone.Resource = %q, want %q", clone.Resource, original.Resource)
	}
	if clone.Action != original.Action {
		t.Errorf("Clone.Action = %q, want %q", clone.Action, original.Action)
	}

	// Verify it's a different instance
	if clone == original {
		t.Error("Clone should be a different instance")
	}

	// Verify modifying clone doesn't affect original
	clone.Name = "modified"
	if original.Name == "modified" {
		t.Error("Modifying clone should not affect original")
	}
}
