package sessiontoken

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// recordingScope records which binding the gate chose; BindTenant refuses a
// missing school like the tenant runtime does.
type recordingScope struct{ bound *string }

func (s recordingScope) BindTenant(ctx context.Context, tenantID, _ int64, _ string) (context.Context, error) {
	if tenantID <= 0 {
		return ctx, errors.New("missing tenant")
	}
	*s.bound = "tenant"
	return ctx, nil
}

func (s recordingScope) BindOrganization(ctx context.Context, _ int64, _ string) context.Context {
	*s.bound = "organization"
	return ctx
}

func (s recordingScope) BindScope(ctx context.Context, _ string) context.Context {
	*s.bound = "scope"
	return ctx
}

func gate(bound *string, accepted ...tokenprimitives.PortalClaims) func(http.Handler) http.Handler {
	return PortalGate(recordingScope{bound: bound}, accepted...)
}

// serveGate answers the status and the binding the gate chose.
func serveGate(accepted []tokenprimitives.PortalClaims, claims AppClaims) (int, string) {
	var bound string
	handler := gate(&bound, accepted...)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ctx := context.WithValue(context.Background(), CtxClaims, claims)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	return rec.Code, bound
}

var (
	tenantRoutes  = TenantRoutePortals
	parentRoutes  = []tokenprimitives.PortalClaims{tokenprimitives.ParentClaims}
	schoolRoutes  = []tokenprimitives.PortalClaims{tokenprimitives.SchoolClaims}
	catererRoutes = []tokenprimitives.PortalClaims{tokenprimitives.CatererClaims}
	traegerRoutes = []tokenprimitives.PortalClaims{tokenprimitives.OrgClaims}
)

// Each route group admits its own portals with the binding their tokens
// need, and refuses every other portal (ADR 0077, ADR 0084).
func TestPortalGateAdmitsOnlyTheRouteGroupsPortals(t *testing.T) {
	t.Parallel()
	tenantToken := AppClaims{ID: 42, TenantID: 100, OrgID: 10, Scope: ""}
	legacyTenant := AppClaims{ID: 42, TenantID: 100, Scope: "tenant"}
	platformAtSchool := AppClaims{ID: 42, TenantID: 100, Scope: "platform"}
	platform := AppClaims{ID: 42, OrgID: 10, Scope: "platform"}
	school := AppClaims{ID: 42, TenantID: 100, Scope: "school"}
	parent := AppClaims{ID: 42, Scope: "parent"}
	caterer := AppClaims{ID: 42, Scope: "caterer"}
	traeger := AppClaims{ID: 42, OrgID: 7, Scope: "org"}

	cases := []struct {
		name   string
		routes []tokenprimitives.PortalClaims
		claims AppClaims
		status int
		bound  string
	}{
		{"tenant token on tenant routes", tenantRoutes, tenantToken, http.StatusOK, "tenant"},
		{"legacy tenant claim on tenant routes", tenantRoutes, legacyTenant, http.StatusOK, "tenant"},
		{"platform token at a school on tenant routes", tenantRoutes, platformAtSchool, http.StatusOK, "tenant"},
		{"platform token without school keeps its organization", tenantRoutes, platform, http.StatusOK, "organization"},
		{"tenant token without school", tenantRoutes, AppClaims{ID: 42, Scope: ""}, http.StatusUnauthorized, ""},
		{"school token on tenant routes", tenantRoutes, school, http.StatusUnauthorized, ""},
		{"parent token on tenant routes", tenantRoutes, parent, http.StatusUnauthorized, ""},
		{"caterer token with school on tenant routes", tenantRoutes, AppClaims{ID: 42, TenantID: 100, Scope: "caterer"}, http.StatusUnauthorized, ""},
		{"Träger token on tenant routes", tenantRoutes, traeger, http.StatusUnauthorized, ""},

		{"school token on school routes", schoolRoutes, school, http.StatusOK, "tenant"},
		{"school token without school", schoolRoutes, AppClaims{ID: 42, Scope: "school"}, http.StatusUnauthorized, ""},
		{"tenant token on school routes", schoolRoutes, tenantToken, http.StatusUnauthorized, ""},

		{"parent token on parent routes", parentRoutes, parent, http.StatusOK, "scope"},
		{"tenant token on parent routes", parentRoutes, tenantToken, http.StatusUnauthorized, ""},
		{"platform token on parent routes", parentRoutes, platform, http.StatusUnauthorized, ""},

		{"caterer token on caterer routes", catererRoutes, caterer, http.StatusOK, "scope"},
		{"caterer without account", catererRoutes, AppClaims{Scope: "caterer"}, http.StatusUnauthorized, ""},
		{"legacy tenant claim on caterer routes", catererRoutes, legacyTenant, http.StatusUnauthorized, ""},
		{"school token on caterer routes", catererRoutes, school, http.StatusUnauthorized, ""},

		{"Träger token on Träger routes", traegerRoutes, traeger, http.StatusOK, "organization"},
		{"Träger token without Träger", traegerRoutes, AppClaims{ID: 42, Scope: "org"}, http.StatusUnauthorized, ""},
		{"Träger token naming a school", traegerRoutes, AppClaims{ID: 42, OrgID: 7, TenantID: 100, Scope: "org"}, http.StatusUnauthorized, ""},
		{"platform token on Träger routes", traegerRoutes, platform, http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		status, bound := serveGate(tc.routes, tc.claims)
		assert.Equal(t, tc.status, status, tc.name)
		assert.Equal(t, tc.bound, bound, tc.name)
	}
}

// A scope no portal issues is refused instead of being bound as a tenant
// token.
func TestTenantMiddlewareRefusesUnknownScope(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"unknown", "kiosk", "operator", "Tenant"} {
		status, _ := serveGate(tenantRoutes, AppClaims{ID: 42, TenantID: 100, OrgID: 10, Scope: scope})
		assert.Equal(t, http.StatusUnauthorized, status, scope)
	}
}

func TestPortalGatePreservesConnectionState(t *testing.T) {
	t.Parallel()
	var bound string
	var gotTLSVersion uint16
	handler := gate(&bound, tenantRoutes...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTLSVersion = r.TLS.Version
		w.WriteHeader(http.StatusOK)
	}))
	claims := AppClaims{ID: 42, Sub: "test@test.com", TenantID: 100, OrgID: 10, Scope: "tenant"}
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.WithValue(context.Background(), CtxClaims, claims))
	req.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uint16(tls.VersionTLS13), gotTLSVersion)
}
