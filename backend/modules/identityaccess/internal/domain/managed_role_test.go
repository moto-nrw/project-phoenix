package domain

import "testing"

func TestManagedRole_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		role    *ManagedRole
		wantErr bool
	}{
		{
			name: "valid role",
			role: &ManagedRole{
				Name: "admin",
			},
			wantErr: false,
		},
		{
			name: "valid role with description",
			role: &ManagedRole{
				Name:        "teacher",
				Description: "Teacher role with classroom access",
			},
			wantErr: false,
		},
		{
			name: "valid system role",
			role: &ManagedRole{
				Name:     "superadmin",
				IsSystem: true,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			role: &ManagedRole{
				Name: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ManagedRole.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagedRole_Validate_Normalization(t *testing.T) {
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
			role := &ManagedRole{Name: tt.inputName}

			err := role.Validate()
			if err != nil {
				t.Fatalf("ManagedRole.Validate() unexpected error = %v", err)
			}

			if role.Name != tt.expectedName {
				t.Errorf("ManagedRole.Validate() name = %q, want %q", role.Name, tt.expectedName)
			}
		})
	}
}

func TestManagedRole_Validate_BaseRole(t *testing.T) {
	t.Parallel()

	validUser := "user"
	validAdmin := "admin"
	validGuardian := "guardian"
	invalid := "superuser"
	empty := ""

	tests := []struct {
		name    string
		role    *ManagedRole
		wantErr bool
	}{
		{
			name:    "nil base_role on custom role is valid (legacy roles)",
			role:    &ManagedRole{Name: "custom", BaseRole: nil},
			wantErr: false,
		},
		{
			name:    "nil base_role on system role is valid",
			role:    &ManagedRole{Name: "admin", IsSystem: true, BaseRole: nil},
			wantErr: false,
		},
		{
			name:    "base_role admin is valid",
			role:    &ManagedRole{Name: "custom", BaseRole: &validAdmin},
			wantErr: false,
		},
		{
			name:    "base_role user is valid",
			role:    &ManagedRole{Name: "custom", BaseRole: &validUser},
			wantErr: false,
		},
		{
			name:    "base_role guardian is valid",
			role:    &ManagedRole{Name: "custom", BaseRole: &validGuardian},
			wantErr: false,
		},
		{
			name:    "invalid base_role is rejected",
			role:    &ManagedRole{Name: "custom", BaseRole: &invalid},
			wantErr: true,
		},
		{
			name:    "empty base_role on custom role is valid (normalized to nil)",
			role:    &ManagedRole{Name: "custom", BaseRole: &empty},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ManagedRole.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify empty base_role normalization on system role (still becomes nil, which is valid)
	t.Run("empty base_role on system role becomes nil after Validate", func(t *testing.T) {
		empty := ""
		role := &ManagedRole{Name: "admin", IsSystem: true, BaseRole: &empty}
		err := role.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if role.BaseRole != nil {
			t.Errorf("expected BaseRole to be nil after normalization, got %q", *role.BaseRole)
		}
	})
}
