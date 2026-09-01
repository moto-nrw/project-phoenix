package authorize

import "testing"

type testRoleGrant struct {
	name        string
	base        *string
	system      bool
	tenantBound bool
	permissions []string
}

func (r *testRoleGrant) AuthorizationGrantData() (bool, string, *string, bool, bool, []string) {
	if r == nil {
		return false, "", nil, false, false, nil
	}
	return true, r.name, r.base, r.system, r.tenantBound, r.permissions
}

func TestCanGrantRole(t *testing.T) {
	t.Parallel()
	user := baseRoleUser
	tests := []struct {
		name  string
		role  *testRoleGrant
		perms []string
		want  bool
	}{
		{"manager grants anything", &testRoleGrant{name: "admin", system: true}, []string{usersManage}, true},
		{"subset role", &testRoleGrant{base: &user, permissions: []string{"users:read"}}, []string{"users:read"}, true},
		{"target escalation", &testRoleGrant{base: &user, permissions: []string{"users:update"}}, []string{"users:read"}, false},
		{"malformed target permission", &testRoleGrant{base: &user, permissions: []string{"users:read", ""}}, []string{"users:read"}, false},
		{"legacy tenant subset", &testRoleGrant{tenantBound: true, permissions: []string{"users:read"}}, []string{"users:read"}, true},
		{"unknown system role", &testRoleGrant{name: "guest", system: true}, nil, false},
		{"missing role", nil, []string{usersManage}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanGrantRole(tt.role, tt.perms); got != tt.want {
				t.Fatalf("CanGrantRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEffectiveBaseRole(t *testing.T) {
	t.Parallel()
	base := " USER "
	if got := EffectiveBaseRole(&testRoleGrant{base: &base}); got != baseRoleUser {
		t.Fatalf("explicit base role = %q", got)
	}
	if got := EffectiveBaseRole(&testRoleGrant{name: "ADMIN", system: true}); got != baseRoleAdmin {
		t.Fatalf("system base role = %q", got)
	}
}
