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

func TestAccountPermission_TableName(t *testing.T) {
	ap := &AccountPermission{}
	expected := "auth.account_permissions"

	got := ap.TableName()
	if got != expected {
		t.Errorf("AccountPermission.TableName() = %q, want %q", got, expected)
	}
}

func TestAccountPermission_IsGranted(t *testing.T) {
	tests := []struct {
		name     string
		granted  bool
		expected bool
	}{
		{
			name:     "granted permission",
			granted:  true,
			expected: true,
		},
		{
			name:     "denied permission",
			granted:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := &AccountPermission{
				AccountID:    1,
				PermissionID: 1,
				Granted:      tt.granted,
			}

			if got := ap.IsGranted(); got != tt.expected {
				t.Errorf("AccountPermission.IsGranted() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountPermission_Grant(t *testing.T) {
	ap := &AccountPermission{
		AccountID:    1,
		PermissionID: 1,
		Granted:      false,
	}

	ap.Grant()

	if !ap.Granted {
		t.Error("AccountPermission.Grant() should set Granted to true")
	}
}

func TestAccountPermission_Deny(t *testing.T) {
	ap := &AccountPermission{
		AccountID:    1,
		PermissionID: 1,
		Granted:      true,
	}

	ap.Deny()

	if ap.Granted {
		t.Error("AccountPermission.Deny() should set Granted to false")
	}
}
