package config

import "testing"

func TestHasPermissionPreservesWildcardSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		required    string
		permissions []string
		want        bool
	}{
		{name: "exact", required: "config:update", permissions: []string{"config:update"}, want: true},
		{name: "resource wildcard", required: "config:update", permissions: []string{"config:*"}, want: true},
		{name: "admin wildcard", required: "config:update", permissions: []string{"admin:*"}, want: true},
		{name: "global wildcard", required: "config:update", permissions: []string{"*:*"}, want: true},
		{name: "unrelated", required: "config:update", permissions: []string{"config:read"}, want: false},
		{name: "malformed required", required: "config", permissions: []string{"*:*"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := hasPermission(test.required, test.permissions); got != test.want {
				t.Fatalf("hasPermission(%q, %v) = %t, want %t", test.required, test.permissions, got, test.want)
			}
		})
	}
}
