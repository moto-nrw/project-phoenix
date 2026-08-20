// School-scope negative matrix (#2207): a school-portal token must be
// rejected on EVERY /api route — the acceptance criterion says "beliebige
// /api-Route", not "every route that happens to sit behind TenantMiddleware".
// This sweep walks the fully-assembled production router (same constructor as
// the route-table golden) and fires a school-scope JWT at each /api route.
//
// Why this exists: the /api/platform/announcements group shipped with
// Verifier + Authenticator but no scope middleware, so a school token read
// tenant data there until the guard was added. A per-package token-matrix
// test cannot catch that class of gap; only walking the assembled tree can.
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Deliberately NOT parallel: mutates process-global configuration.
func TestSchoolScopeRejectedOnAllAPIRoutes(t *testing.T) {
	apiInstance := newGoldenAPI(t)

	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err)
	schoolToken, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:  42,
		Sub: "lehrkraft@example.test",
		// A deliberately over-privileged claim set: even WITH tenant-admin
		// style roles and permissions in the token, the scope alone must
		// keep every /api door shut.
		Roles:       []string{"lehrkraft", "admin"},
		Permissions: []string{"class_day:read", "users:read", "config:update", "admin:*"},
		IsAdmin:     true,
		Scope:       tenant.ScopeSchool,
		TenantID:    testpkg.Tenant(t),
		OrgID:       1,
	})
	require.NoError(t, err)

	var apiRoutes []string
	walkErr := chi.Walk(apiInstance.Router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/api/") {
			apiRoutes = append(apiRoutes, method+" "+route)
		}
		return nil
	})
	require.NoError(t, walkErr)
	require.NotEmpty(t, apiRoutes)

	// Per route, two probes decide the expectation without any allowlist:
	//   1. no Authorization header → baseline. 401 means "JWT-gated".
	//   2. school token attached.
	// A JWT-gated route must answer 401 to the school token. A route that
	// is NOT JWT-gated (public enrollment endpoints, the display-token
	// dashboard, device-auth IoT routes) must answer the school token
	// exactly like the anonymous baseline — the token may never improve
	// the response. Together that is the acceptance criterion: a school
	// token unlocks nothing anywhere under /api.
	for _, mr := range apiRoutes {
		parts := strings.SplitN(mr, " ", 2)
		method, pattern := parts[0], parts[1]

		probePath := chiParamPattern.ReplaceAllString(pattern, "1")
		probePath = strings.ReplaceAll(probePath, "*", "x")

		baseline := httptest.NewRecorder()
		apiInstance.Router.ServeHTTP(baseline, httptest.NewRequest(method, probePath, nil))

		req := httptest.NewRequest(method, probePath, nil)
		req.Header.Set("Authorization", "Bearer "+schoolToken)
		rec := httptest.NewRecorder()
		apiInstance.Router.ServeHTTP(rec, req)

		if baseline.Code == http.StatusUnauthorized {
			require.Equal(t, http.StatusUnauthorized, rec.Code,
				"school-scope token must get 401 on the JWT-gated route %s %s (got %d: %s)",
				method, pattern, rec.Code, strings.TrimSpace(rec.Body.String()))
			continue
		}

		require.Equal(t, baseline.Code, rec.Code,
			"school-scope token must not change the response of the non-JWT-gated route %s %s (anonymous %d, with token %d: %s)",
			method, pattern, baseline.Code, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}
