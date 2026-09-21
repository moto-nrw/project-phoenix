package domain

import "testing"

func TestManagedPermission_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		permission *ManagedPermission
		wantErr    bool
	}{
		{
			name: "valid permission",
			permission: &ManagedPermission{
				Name:     "users_read",
				Resource: "users",
				Action:   "read",
			},
			wantErr: false,
		},
		{
			name: "valid permission with description",
			permission: &ManagedPermission{
				Name:        "users_write",
				Description: "Allows writing user data",
				Resource:    "users",
				Action:      "write",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			permission: &ManagedPermission{
				Name:     "",
				Resource: "users",
				Action:   "read",
			},
			wantErr: true,
		},
		{
			name: "empty resource",
			permission: &ManagedPermission{
				Name:     "users_read",
				Resource: "",
				Action:   "read",
			},
			wantErr: true,
		},
		{
			name: "empty action",
			permission: &ManagedPermission{
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
				t.Errorf("ManagedPermission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagedPermission_Validate_Normalization(t *testing.T) {
	t.Parallel()

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
			perm := &ManagedPermission{
				Name:     tt.inputName,
				Resource: tt.inputResource,
				Action:   tt.inputAction,
			}

			err := perm.Validate()
			if err != nil {
				t.Fatalf("ManagedPermission.Validate() unexpected error = %v", err)
			}

			if perm.Name != tt.expectedName {
				t.Errorf("ManagedPermission.Name = %q, want %q", perm.Name, tt.expectedName)
			}
			if perm.Resource != tt.expectedResource {
				t.Errorf("ManagedPermission.Resource = %q, want %q", perm.Resource, tt.expectedResource)
			}
			if perm.Action != tt.expectedAction {
				t.Errorf("ManagedPermission.Action = %q, want %q", perm.Action, tt.expectedAction)
			}
		})
	}
}
