package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

func TestSecurityPrincipalMiddlewareResolvesPortalScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims jwt.AppClaims
		scope  string
	}{
		{name: "tenant", claims: jwt.AppClaims{ID: 1, TenantID: 11}, scope: ""},
		{name: "organization", claims: jwt.AppClaims{ID: 2, TenantID: 11, OrgID: 7, Scope: "org"}, scope: "org"},
		{name: "platform", claims: jwt.AppClaims{ID: 3, Scope: "platform"}, scope: "platform"},
		{name: "parent", claims: jwt.AppClaims{ID: 4, Scope: "parent"}, scope: "parent"},
		{name: "school", claims: jwt.AppClaims{ID: 5, TenantID: 11, Scope: "school"}, scope: "school"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			handler := SecurityPrincipalMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				principal, err := CurrentPrincipal(r.Context())
				if err != nil {
					t.Fatalf("CurrentPrincipal() error = %v", err)
				}
				if principal.AccountID() != int64(tt.claims.ID) || string(principal.Scope()) != tt.scope {
					t.Fatalf("CurrentPrincipal() = account %d scope %q", principal.AccountID(), principal.Scope())
				}
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, tt.claims))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusNoContent || !called {
				t.Fatalf("status = %d, called = %v; want 204, true", recorder.Code, called)
			}
		})
	}
}

func TestSecurityPrincipalMiddlewareRejectsMalformedClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims *jwt.AppClaims
	}{
		{name: "missing claims"},
		{name: "unknown scope", claims: &jwt.AppClaims{ID: 1, TenantID: 11, Scope: "unknown"}},
		{name: "parent carrying tenant", claims: &jwt.AppClaims{ID: 1, TenantID: 11, Scope: "parent"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := SecurityPrincipalMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("malformed claims reached handler")
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, *tt.claims))
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", recorder.Code)
			}
		})
	}
}

func TestPermissionMiddlewareFailsClosed(t *testing.T) {
	t.Parallel()

	terminal := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	missing := httptest.NewRecorder()
	RequiresPermission("users:read")(terminal).ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing principal status = %d, want 401", missing.Code)
	}

	for _, tt := range []struct {
		name        string
		permissions []string
		want        int
	}{
		{name: "denied", permissions: []string{"users:update"}, want: http.StatusForbidden},
		{name: "allowed", permissions: []string{"users:read"}, want: http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			claims := jwt.AppClaims{ID: 1, TenantID: 11, Permissions: tt.permissions}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
			recorder := httptest.NewRecorder()
			SecurityPrincipalMiddleware(RequiresPermission("users:read")(terminal)).ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}

func TestPermissionMiddlewareAnyAndAllSemantics(t *testing.T) {
	t.Parallel()
	terminal := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tt := range []struct {
		name string
		gate Middleware
		want int
	}{
		{name: "any allows one", gate: RequiresAnyPermission("users:read", "users:update"), want: http.StatusNoContent},
		{name: "any denies none", gate: RequiresAnyPermission("users:update", "users:delete"), want: http.StatusForbidden},
		{name: "all allows all", gate: RequiresAllPermissions("users:read"), want: http.StatusNoContent},
		{name: "all denies partial", gate: RequiresAllPermissions("users:read", "users:update"), want: http.StatusForbidden},
		{name: "empty any denies", gate: RequiresAnyPermission(), want: http.StatusForbidden},
		{name: "empty all allows", gate: RequiresAllPermissions(), want: http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			claims := jwt.AppClaims{ID: 1, TenantID: 11, Permissions: []string{"users:read"}}
			req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
			recorder := httptest.NewRecorder()
			SecurityPrincipalMiddleware(tt.gate(terminal)).ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}

func TestAuthorizationObserverReceivesStableReason(t *testing.T) {
	t.Parallel()
	var events []AuthorizationEvent
	handler := AuthorizationObserverMiddleware(func(event AuthorizationEvent) {
		events = append(events, event)
	})(SecurityPrincipalMiddleware(RequiresPermission("users:read")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1, TenantID: 2}))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if len(events) != 2 || events[0].Outcome != "resolved" || events[1].Reason != "permission_denied" {
		t.Fatalf("authorization events = %+v", events)
	}
}
