package auth

import (
	"testing"
)

func TestAccountPermission_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ap      *AccountPermission
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid account permission",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: 1,
				Granted:      true,
			},
			wantErr: false,
		},
		{
			name: "zero account ID",
			ap: &AccountPermission{
				AccountID:    0,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "negative account ID",
			ap: &AccountPermission{
				AccountID:    -1,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "zero permission ID",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: 0,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
		{
			name: "negative permission ID",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: -1,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ap.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountPermission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("AccountPermission.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
