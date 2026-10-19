package authorize

import "testing"

func TestHasPermission(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, required string
		granted        []string
		want           bool
	}{
		{name: "exact", required: "users:read", granted: []string{"users:read"}, want: true},
		{name: "different resource", required: "users:read", granted: []string{"posts:read"}},
		{name: "admin wildcard", required: "users:read", granted: []string{"admin:*"}, want: true},
		{name: "resource wildcard", required: "users:read", granted: []string{"users:*"}, want: true},
		{name: "action wildcard", required: "users:read", granted: []string{"*:read"}, want: true},
		{name: "full wildcard", required: "users:read", granted: []string{"*:*"}, want: true},
		{name: "empty requirement", granted: []string{"users:read"}, want: true},
		{name: "invalid requirement", required: "invalid", granted: []string{"users:read"}},
		{name: "invalid grant", required: "users:read", granted: []string{"user*"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasPermission(tt.required, tt.granted); got != tt.want {
				t.Fatalf("HasPermission(%q, %q) = %v, want %v", tt.required, tt.granted, got, tt.want)
			}
		})
	}
}

func TestHasAdminWildcard(t *testing.T) {
	t.Parallel()
	if !HasAdminWildcard([]string{"admin:*"}) || !HasAdminWildcard([]string{"*:*"}) {
		t.Fatal("admin wildcard must be recognized")
	}
	if HasAdminWildcard([]string{"admin:read", "users:*"}) {
		t.Fatal("narrow permissions must not create admin scope")
	}
}
