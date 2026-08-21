package authorize

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Moved from models/auth/account_test.go (TestAccount_HasRole) when the role
// membership check left the model in issue #586 (Rule 12).
func TestAccountHasRole(t *testing.T) {
	t.Parallel()

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
