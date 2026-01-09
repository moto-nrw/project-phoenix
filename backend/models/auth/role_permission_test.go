package auth

import (
	"testing"
)

func TestRolePermission_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rp      *RolePermission
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid role permission",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: 1,
			},
			wantErr: false,
		},
		{
			name: "zero role ID",
			rp: &RolePermission{
				RoleID:       0,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
		{
			name: "negative role ID",
			rp: &RolePermission{
				RoleID:       -1,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
		{
			name: "zero permission ID",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: 0,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
		{
			name: "negative permission ID",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: -1,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rp.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RolePermission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("RolePermission.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
