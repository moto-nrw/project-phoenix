package active

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func TestRequireVisitViewUsesPrincipalAndPreservesDenialContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal *permissions.Principal
		param     string
		want      int
	}{
		{name: "missing principal", param: "1", want: http.StatusUnauthorized},
		{name: "malformed visit ID", principal: visitTestPrincipal(t), param: "bad", want: http.StatusForbidden},
		{name: "admin role bypass", principal: visitAdminRolePrincipal(t), param: "1", want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/active/visits/"+tt.param, nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("id", tt.param)
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, route)
			if tt.principal != nil {
				ctx = permissions.WithPrincipal(ctx, *tt.principal)
			}
			recorder := httptest.NewRecorder()
			(&Resource{}).requireVisitView(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(recorder, req.WithContext(ctx))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
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
			if got := visitReadAll(principal); got != tt.want {
				t.Fatalf("visitReadAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func visitTestPrincipal(t *testing.T) *permissions.Principal {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	return &principal
}

func visitAdminRolePrincipal(t *testing.T) *permissions.Principal {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2, Roles: []string{"admin"}})
	if err != nil {
		t.Fatal(err)
	}
	return &principal
}
