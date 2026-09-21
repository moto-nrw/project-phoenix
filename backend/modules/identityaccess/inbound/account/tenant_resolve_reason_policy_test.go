// Reason-policy metadata on the public GET /auth/tenant/resolve endpoint
// (#2267). Staff without config:read decide parent requests, so the switch
// that says whether an approval needs a written reason has to travel on the
// tenant shell like the other feature flags, not via /api/settings/schema.
package account_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

type reasonPolicyResolveResp struct {
	Data struct {
		ParentRequestReasonPolicy string `json:"parent_request_reason_policy"`
	} `json:"data"`
}

func resolveReasonPolicy(t *testing.T, slug string, resource *authAPI.Resource) string {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp reasonPolicyResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data.ParentRequestReasonPolicy
}

func TestTenantResolveEmitsReasonPolicy(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.Equal(t, settings.ReasonPolicyBoth, resolveReasonPolicy(t, slug, resource),
		"a school that never touched the setting gets the registry default")

	require.NoError(t,
		tenantSettings.SetValue(scope.Context(), settings.KeyParentRequestReasonPolicy,
			settings.ReasonPolicyNobody, nil, nil),
		"switch the reason requirement off for the isolated tenant")
	assert.Equal(t, settings.ReasonPolicyNobody, resolveReasonPolicy(t, slug, resource),
		"a tenant override must round-trip to the tenant shell")
}

// A stored value the registry no longer knows must not reach the client as a
// policy it would have to guess about: the strictest reading wins, so the UI
// asks for a reason instead of hiding a field the server may require.
func TestTenantResolveReasonPolicyUnknownValueFallsBackToBoth(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	// Stored straight through the repository: SetValue would reject a value
	// outside the registry options, and what we need to pin is the read side
	// meeting a row written by some future or older release.
	testutil.StoreRawTenantSetting(t, db, scope.TenantID,
		settings.KeyParentRequestReasonPolicy, json.RawMessage(`"irgendwas"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.Equal(t, settings.ReasonPolicyBoth, resolveReasonPolicy(t, slug, resource))
}
