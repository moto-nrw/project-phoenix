package auth

import (
	"testing"
	"time"
)

func TestAccount_Validate(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		wantErr bool
	}{
		{
			name: "Valid account",
			account: Account{
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "Empty email",
			account: Account{
				Email: "",
			},
			wantErr: true,
		},
		{
			name: "Invalid email format",
			account: Account{
				Email: "invalid-email",
			},
			wantErr: true,
		},
		{
			name: "Email normalization",
			account: Account{
				Email: "TEST@Example.COM",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.account.Validate()

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("Account.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check email normalization
			if !tt.wantErr && tt.name == "Email normalization" && tt.account.Email != "test@example.com" {
				t.Errorf("Email was not normalized to lowercase, got %s", tt.account.Email)
			}
		})
	}
}

func TestAccount_HasRole(t *testing.T) {
	admin := &Role{Name: "admin"}
	user := &Role{Name: "user"}

	account := Account{
		Email: "test@example.com",
		Roles: []*Role{admin},
	}

	if !account.HasRole("admin") {
		t.Error("HasRole() should return true for 'admin'")
	}

	if account.HasRole("user") {
		t.Error("HasRole() should return false for 'user'")
	}

	// Test case insensitivity
	if !account.HasRole("ADMIN") {
		t.Error("HasRole() should be case-insensitive")
	}

	// Test with nil roles
	account.Roles = nil
	if account.HasRole("admin") {
		t.Error("HasRole() should return false when roles is nil")
	}
}

func TestAccount_HasPermission(t *testing.T) {
	readPerm := &Permission{Name: "read"}
	writePerm := &Permission{Name: "write"}

	account := Account{
		Email:       "test@example.com",
		Permissions: []*Permission{readPerm},
	}

	if !account.HasPermission("read") {
		t.Error("HasPermission() should return true for 'read'")
	}

	if account.HasPermission("write") {
		t.Error("HasPermission() should return false for 'write'")
	}

	// Test case insensitivity
	if !account.HasPermission("READ") {
		t.Error("HasPermission() should be case-insensitive")
	}

	// Test with nil permissions
	account.Permissions = nil
	if account.HasPermission("read") {
		t.Error("HasPermission() should return false when permissions is nil")
	}
}

func TestAccount_SetLastLogin(t *testing.T) {
	account := Account{
		Email: "test@example.com",
	}

	now := time.Now()
	account.SetLastLogin(now)

	if account.LastLogin == nil {
		t.Error("SetLastLogin() should set the LastLogin field")
	}

	if !account.LastLogin.Equal(now) {
		t.Errorf("LastLogin should equal %v, got %v", now, account.LastLogin)
	}
}

func TestAccount_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		active   bool
		expected bool
	}{
		{
			name:     "Active account",
			active:   true,
			expected: true,
		},
		{
			name:     "Inactive account",
			active:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := Account{
				Email:  "test@example.com",
				Active: tt.active,
			}

			if got := account.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
