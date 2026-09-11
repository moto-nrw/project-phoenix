package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRole_Validate(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestRole_Validate_BaseRole(t *testing.T) {
	t.Parallel()

	validUser := "user"
	validAdmin := "admin"
	validGuardian := "guardian"
	invalid := "superuser"
	empty := ""

	tests := []struct {
		name    string
		role    *Role
		wantErr bool
	}{
		{
			name:    "nil base_role on custom role is valid (legacy roles)",
			role:    &Role{Name: "custom", BaseRole: nil},
			wantErr: false,
		},
		{
			name:    "nil base_role on system role is valid",
			role:    &Role{Name: "admin", IsSystem: true, BaseRole: nil},
			wantErr: false,
		},
		{
			name:    "base_role admin is valid",
			role:    &Role{Name: "custom", BaseRole: &validAdmin},
			wantErr: false,
		},
		{
			name:    "base_role user is valid",
			role:    &Role{Name: "custom", BaseRole: &validUser},
			wantErr: false,
		},
		{
			name:    "base_role guardian is valid",
			role:    &Role{Name: "custom", BaseRole: &validGuardian},
			wantErr: false,
		},
		{
			name:    "invalid base_role is rejected",
			role:    &Role{Name: "custom", BaseRole: &invalid},
			wantErr: true,
		},
		{
			name:    "empty base_role on custom role is valid (normalized to nil)",
			role:    &Role{Name: "custom", BaseRole: &empty},
			wantErr: false,
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

	// Verify empty base_role normalization on system role (still becomes nil, which is valid)
	t.Run("empty base_role on system role becomes nil after Validate", func(t *testing.T) {
		empty := ""
		role := &Role{Name: "admin", IsSystem: true, BaseRole: &empty}
		err := role.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if role.BaseRole != nil {
			t.Errorf("expected BaseRole to be nil after normalization, got %q", *role.BaseRole)
		}
	})
}

func TestRole_IsSystemFlag(t *testing.T) {
	t.Parallel()

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

func TestRoleAuthorizationGrantDataPreservesMalformedPermission(t *testing.T) {
	t.Parallel()
	role := &Role{Permissions: []*Permission{{Name: "users:read"}, nil}}
	present, _, _, _, _, granted := role.AuthorizationGrantData()
	if !present || len(granted) != 2 || granted[0] != "users:read" || granted[1] != "" {
		t.Fatalf("AuthorizationGrantData() = present %v, permissions %q", present, granted)
	}
}

func TestRole_GetTenantID(t *testing.T) {
	t.Parallel()

	t.Run("returns tenant ID when set", func(t *testing.T) {
		tid := int64(42)
		role := &Role{Name: "test", TenantID: &tid}
		if got := role.GetTenantID(); got != 42 {
			t.Errorf("GetTenantID() = %v, want 42", got)
		}
	})

	t.Run("returns 0 for system role (nil tenant)", func(t *testing.T) {
		role := &Role{Name: "system", TenantID: nil, IsSystem: true}
		if got := role.GetTenantID(); got != 0 {
			t.Errorf("GetTenantID() = %v, want 0", got)
		}
	})
}

func TestRole_SetTenantID(t *testing.T) {
	t.Parallel()

	role := &Role{Name: "test"}
	role.SetTenantID(99)
	if role.TenantID == nil || *role.TenantID != 99 {
		t.Errorf("SetTenantID(99) did not set TenantID correctly")
	}
}

func TestRole_GetID(t *testing.T) {
	t.Parallel()

	role := &Role{
		Model: base.Model{ID: 42},
		Name:  "test",
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := role.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", role.GetID())
	}
}

func TestRole_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	role := &Role{
		Model: base.Model{CreatedAt: now},
		Name:  "test",
	}

	if got := role.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestRole_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	role := &Role{
		Model: base.Model{UpdatedAt: now},
		Name:  "test",
	}

	if got := role.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
