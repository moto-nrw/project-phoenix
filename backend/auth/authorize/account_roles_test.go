package authorize

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Moved from models/auth/account_test.go (TestAccount_HasRole) when the role
// membership check left the model in issue #586 (Rule 12).
func TestAccountHasRole(t *testing.T) {
	account := &auth.Account{
		Email: "test@example.com",
		Roles: []*auth.Role{{Name: "admin"}},
	}

	if !AccountHasRole(account, "admin") {
		t.Error("AccountHasRole() should return true for 'admin'")
	}
	if AccountHasRole(account, "user") {
		t.Error("AccountHasRole() should return false for 'user'")
	}
	// Case-insensitive.
	if !AccountHasRole(account, "ADMIN") {
		t.Error("AccountHasRole() should be case-insensitive")
	}
	// Nil roles.
	account.Roles = nil
	if AccountHasRole(account, "admin") {
		t.Error("AccountHasRole() should return false when roles is nil")
	}
	// Nil account.
	if AccountHasRole(nil, "admin") {
		t.Error("AccountHasRole() should return false for a nil account")
	}
}

// Moved from models/auth/account_test.go (TestAccount_HasPermission).
func TestAccountHasPermission(t *testing.T) {
	account := &auth.Account{
		Email:       "test@example.com",
		Permissions: []*auth.Permission{{Name: "read"}},
	}

	if !AccountHasPermission(account, "read") {
		t.Error("AccountHasPermission() should return true for 'read'")
	}
	if AccountHasPermission(account, "write") {
		t.Error("AccountHasPermission() should return false for 'write'")
	}
	// Case-insensitive.
	if !AccountHasPermission(account, "READ") {
		t.Error("AccountHasPermission() should be case-insensitive")
	}
	// Nil permissions.
	account.Permissions = nil
	if AccountHasPermission(account, "read") {
		t.Error("AccountHasPermission() should return false when permissions is nil")
	}
	// Nil account.
	if AccountHasPermission(nil, "read") {
		t.Error("AccountHasPermission() should return false for a nil account")
	}
}
