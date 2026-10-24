package securityruntime

import "testing"

func TestLeadsSchool(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		role        string
		permissions []string
		want        bool
	}{
		{name: "system admin role", role: "admin", want: true},
		{name: "admin role in any case and padding", role: " Admin ", want: true},
		{name: "school role with config:manage", role: "Leitung", permissions: []string{"config:manage"}, want: true},
		{name: "school role with config wildcard", role: "Leitung", permissions: []string{"config:*"}, want: true},
		{name: "caregiver role", role: "user", permissions: []string{"groups:read"}, want: false},
		{name: "no role", want: false},
	} {
		if got := LeadsSchool(tc.role, tc.permissions); got != tc.want {
			t.Errorf("%s: LeadsSchool(%q, %v) = %v, want %v", tc.name, tc.role, tc.permissions, got, tc.want)
		}
	}
}
