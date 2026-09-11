package authorize

import "testing"

type testAccount struct{ roles []string }

func (a *testAccount) AuthorizationRoleNames() []string {
	if a == nil {
		return nil
	}
	return a.roles
}

func TestAccountHasRole(t *testing.T) {
	t.Parallel()
	account := &testAccount{roles: []string{"User", "ADMIN"}}
	if !AccountHasRole(account, "admin") || AccountHasRole(account, "guardian") {
		t.Fatal("role membership must be case-insensitive and fail closed")
	}
	var missing *testAccount
	if AccountHasRole(missing, "admin") {
		t.Fatal("nil account must not have a role")
	}
}
