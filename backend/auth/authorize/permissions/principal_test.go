package permissions

import (
	"context"
	"errors"
	"testing"
)

func TestPrincipalScopeValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input PrincipalInput
		want  Scope
	}{
		{"legacy tenant", PrincipalInput{AccountID: 1, TenantID: 2}, ScopeTenant},
		{"explicit tenant", PrincipalInput{AccountID: 1, TenantID: 2, Scope: "tenant"}, ScopeTenant},
		{"organization", PrincipalInput{AccountID: 1, TenantID: 2, OrganizationID: 3, Scope: "org"}, ScopeOrganization},
		{"platform", PrincipalInput{AccountID: 1, Scope: "platform"}, ScopePlatform},
		{"parent", PrincipalInput{AccountID: 1, Scope: "parent"}, ScopeParent},
		{"school", PrincipalInput{AccountID: 1, TenantID: 2, Scope: "school"}, ScopeSchool},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := NewPrincipal(tt.input)
			if err != nil || principal.Scope() != tt.want {
				t.Fatalf("NewPrincipal() = scope %q, %v", principal.Scope(), err)
			}
		})
	}
}

func TestPrincipalRejectsInvalidIdentity(t *testing.T) {
	t.Parallel()
	inputs := []PrincipalInput{
		{},
		{AccountID: 1},
		{AccountID: 1, TenantID: 2, Scope: "parent"},
		{AccountID: 1, TenantID: 2, Scope: "org"},
		{AccountID: 1, TenantID: 2, Scope: "unknown"},
	}
	for _, input := range inputs {
		if _, err := NewPrincipal(input); !errors.Is(err, ErrInvalidPrincipal) {
			t.Errorf("NewPrincipal(%+v) error = %v", input, err)
		}
	}
}

func TestPrincipalContextAndPermissionsFailClosed(t *testing.T) {
	t.Parallel()
	if _, err := PrincipalFromContext(context.Background()); !errors.Is(err, ErrPrincipalRequired) {
		t.Fatalf("missing principal error = %v", err)
	}
	principal, err := NewPrincipal(PrincipalInput{AccountID: 1, TenantID: 2, Roles: []string{"ADMIN"}, Permissions: []string{"users:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if principal.HasAdminScope() || !principal.HasRole("admin") || !principal.HasPermission("users:read") || principal.HasPermission("users:update") {
		t.Fatal("principal role and permission decisions changed")
	}
	admin, err := NewPrincipal(PrincipalInput{AccountID: 2, TenantID: 2, Admin: true})
	if err != nil || !admin.HasAdminScope() {
		t.Fatalf("signed admin claim = %v, %v", admin.HasAdminScope(), err)
	}
	stored, err := PrincipalFromContext(WithPrincipal(context.Background(), principal))
	if err != nil || stored.AccountID() != 1 {
		t.Fatalf("stored principal = %+v, %v", stored, err)
	}
}
