// Träger-scope matrix (ADR 0084 section 3, #4649): a Träger token (scope org)
// unlocks nothing outside /traeger, and no token of another portal unlocks a
// /traeger route. Like the school-scope matrix it walks the fully assembled
// production router, so a group that forgets its scope middleware fails here.
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func checkTraegerScopeMatrix(t *testing.T, apiInstance *goldenRuntime) {
	t.Parallel()
	tokenAuth, err := newSessionTokenAuth()
	require.NoError(t, err)
	mint := func(claims sessiontoken.AppClaims) string {
		token, err := tokenAuth.CreateJWT(claims)
		require.NoError(t, err)
		return token
	}
	// Deliberately over-privileged: every Träger right and the admin
	// wildcard. The scope alone must keep every foreign door shut.
	everyRight := append(identityaccess.TraegerRights([]string{
		identityaccess.TraegerRoleAdministration, identityaccess.TraegerRoleBilling, identityaccess.TraegerRolePersonnel,
	}), "admin:*")
	traegerToken := mint(sessiontoken.AppClaims{ID: 42, Sub: "buero@example.test", Roles: []string{identityaccess.TraegerRoleAdministration},
		Permissions: everyRight, IsAdmin: true, Scope: tenant.ScopeOrg, OrgID: 1})
	foreignTokens := map[string]string{
		"OGS portal": mint(sessiontoken.AppClaims{ID: 42, Sub: "ogs@example.test", Roles: []string{"admin"}, Permissions: everyRight,
			IsAdmin: true, TenantID: testpkg.Tenant(t), OrgID: 1}),
		"school portal": mint(sessiontoken.AppClaims{ID: 42, Sub: "lehrkraft@example.test", Roles: []string{"lehrkraft"}, Permissions: everyRight,
			Scope: tenant.ScopeSchool, TenantID: testpkg.Tenant(t), OrgID: 1}),
		"parents":  mint(sessiontoken.AppClaims{ID: 42, Sub: "eltern@example.test", Roles: []string{"guardian"}, Permissions: everyRight, Scope: tenant.ScopeParent}),
		"operator": mint(sessiontoken.AppClaims{ID: 42, Sub: "operator@example.test", Roles: []string{"operator"}, Permissions: everyRight, Scope: tenant.ScopePlatform}),
	}

	var routes []string
	require.NoError(t, chi.Walk(apiInstance.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}))
	traegerRoutes := 0
	for _, mr := range routes {
		method, pattern, _ := strings.Cut(mr, " ")
		probePath := strings.ReplaceAll(chiParamPattern.ReplaceAllString(pattern, "1"), "*", "x")
		baseline := httptest.NewRecorder()
		apiInstance.router.ServeHTTP(baseline, httptest.NewRequest(method, probePath, nil))
		probe := func(token string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, probePath, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			apiInstance.router.ServeHTTP(rec, req)
			return rec
		}
		if strings.HasPrefix(pattern, "/traeger/") {
			traegerRoutes++
			for portal, token := range foreignTokens {
				requireNoScopeGain(t, baseline, probe(token), portal+" token", method, pattern)
			}
			continue
		}
		requireNoScopeGain(t, baseline, probe(traegerToken), "Träger token", method, pattern)
	}
	require.Positive(t, traegerRoutes, "the Träger routes must be mounted")
}

// requireNoScopeGain: on a JWT-gated route (anonymous 401) the token is
// refused, with 401 from the scope guards or 403 from the operator guard that
// refuses every non-operator session; on any other route it changes nothing.
func requireNoScopeGain(t *testing.T, baseline, rec *httptest.ResponseRecorder, token, method, pattern string) {
	t.Helper()
	if baseline.Code == http.StatusUnauthorized {
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code, "%s must be refused on the JWT-gated route %s %s (got %d: %s)",
			token, method, pattern, rec.Code, strings.TrimSpace(rec.Body.String()))
		return
	}
	require.Equal(t, baseline.Code, rec.Code, "%s must not change the response of %s %s (anonymous %d, with token %d: %s)",
		token, method, pattern, baseline.Code, rec.Code, strings.TrimSpace(rec.Body.String()))
}
