// Reason-policy metadata on the public GET /auth/tenant/resolve endpoint
// (#2267). Staff without config:read decide parent requests, so the switch
// that says whether an approval needs a written reason has to travel on the
// tenant shell like the other feature flags, not via /api/settings/schema.
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

	db, svc := testutil.SetupAPITest(t)
	scope, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = svc.Settings

	assert.Equal(t, configModel.ReasonPolicyBoth, resolveReasonPolicy(t, slug, resource),
		"a school that never touched the setting gets the registry default")

	require.NoError(t,
		svc.Settings.SetValue(scope.Context(), configModel.KeyParentRequestReasonPolicy,
			configModel.ReasonPolicyNobody, nil, nil),
		"switch the reason requirement off for the isolated tenant")
	assert.Equal(t, configModel.ReasonPolicyNobody, resolveReasonPolicy(t, slug, resource),
		"a tenant override must round-trip to the tenant shell")
}

// A stored value the registry no longer knows must not reach the client as a
// policy it would have to guess about: the strictest reading wins, so the UI
// asks for a reason instead of hiding a field the server may require.
func TestTenantResolveReasonPolicyUnknownValueFallsBackToBoth(t *testing.T) {
	t.Parallel()

	db, svc := testutil.SetupAPITest(t)
	scope, slug := newTenantResolveScope(t, db)

	// Stored straight through the repository: SetValue would reject a value
	// outside the registry options, and what we need to pin is the read side
	// meeting a row written by some future or older release.
	stored := &configModel.SettingValue{
		SettingKey: configModel.KeyParentRequestReasonPolicy,
		Value:      json.RawMessage(`"irgendwas"`),
	}
	stored.SetTenantID(scope.TenantID)
	require.NoError(t,
		configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db)).Upsert(scope.Context(), stored),
		"store an unknown parent_request_reason_policy override")

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = svc.Settings

	assert.Equal(t, configModel.ReasonPolicyBoth, resolveReasonPolicy(t, slug, resource))
}
