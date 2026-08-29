package authorize

import (
	"context"
	"errors"
	"testing"
)

func TestResolveStudentReadScope(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		perms []string
		staff *testStaffContext
		want  bool
	}{
		{name: "admin without user context", perms: []string{"admin:*"}, want: true},
		{name: "verified staff", perms: []string{"users:read"}, staff: &testStaffContext{present: true}, want: true},
		{name: "non-staff", perms: []string{"users:read"}, staff: &testStaffContext{}},
		{name: "missing user context", perms: []string{"users:read"}},
		{name: "staff lookup error", perms: []string{"users:read"}, staff: &testStaffContext{err: errors.New("database unavailable")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var staff StudentAccessUserContext
			if tt.staff != nil {
				staff = tt.staff
			}
			if got := ResolveStudentReadScope(context.Background(), tt.perms, staff); got != tt.want {
				t.Fatalf("ResolveStudentReadScope() = %v, want %v", got, tt.want)
			}
		})
	}
}
