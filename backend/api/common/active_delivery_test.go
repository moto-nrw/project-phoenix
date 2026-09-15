package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func TestActiveDeliveryPermissionMappings(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		permission string
		guard      func() Middleware
	}{
		{"group read", permissions.GroupsRead, RequireActiveGroupRead},
		{"group create", permissions.GroupsCreate, RequireActiveGroupCreate},
		{"group update", permissions.GroupsUpdate, RequireActiveGroupUpdate},
		{"group delete", permissions.GroupsDelete, RequireActiveGroupDelete},
		{"group assign", permissions.GroupsAssign, RequireActiveGroupAssign},
		{"visit update", permissions.VisitsUpdate, RequireVisitUpdate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range []struct {
				granted       []string
				authenticated bool
				want          int
			}{
				{granted: []string{tc.permission}, authenticated: true, want: http.StatusNoContent},
				{granted: []string{"unrelated:read"}, authenticated: true, want: http.StatusForbidden},
				{want: http.StatusUnauthorized},
			} {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if scenario.authenticated {
					principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2, Permissions: scenario.granted})
					if err != nil {
						t.Fatal(err)
					}
					req = req.WithContext(permissions.WithPrincipal(req.Context(), principal))
				}
				recorder := httptest.NewRecorder()
				tc.guard()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(recorder, req)
				if recorder.Code != scenario.want {
					t.Fatalf("grant %v, authenticated %v: status %d, want %d", scenario.granted, scenario.authenticated, recorder.Code, scenario.want)
				}
			}
		})
	}
}

func TestVisitReadAllPreservesLegacyBroadGrants(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		roles       []string
		permissions []string
		admin       bool
		want        bool
	}{
		{name: "admin role", roles: []string{"admin"}, want: true},
		{name: "admin role remains canonical", roles: []string{"ADMIN"}},
		{name: "signed admin claim", admin: true, want: true},
		{name: "admin wildcard", permissions: []string{"admin:*"}, want: true},
		{name: "visits read", permissions: []string{permissions.VisitsRead}, want: true},
		{name: "visits manage", permissions: []string{permissions.VisitsManage}, want: true},
		{name: "resource wildcard was not a broad grant", permissions: []string{"visits:*"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
				AccountID: 1, TenantID: 2, Roles: tt.roles, Permissions: tt.permissions, Admin: tt.admin,
			})
			if err != nil {
				t.Fatal(err)
			}
			accountID, got, err := VisitReadAccess(permissions.WithPrincipal(context.Background(), principal))
			if err != nil || accountID != 1 {
				t.Fatalf("VisitReadAccess identity = %d, %v", accountID, err)
			}
			if got != tt.want {
				t.Fatalf("VisitReadAccess broad grant = %v, want %v", got, tt.want)
			}
		})
	}
}
