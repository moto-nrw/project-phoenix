// Analyse-Freigabe on the public GET /auth/tenant/resolve endpoint (#3603).
// The OGS portal reads it from the school context on every page load, so a
// revocation takes effect at the next load at the latest; it records and
// sends pseudonymous IDs only while the flag is true.
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

type analyticsResolveResp struct {
	Data struct {
		AnalyticsFreigabe               bool `json:"analytics_freigabe"`
		AnalyticsRecordingSamplePercent int  `json:"analytics_recording_sample_percent"`
	} `json:"data"`
}

func resolveAnalytics(t *testing.T, resource *authAPI.Resource, slug string) analyticsResolveResp {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp analyticsResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

// A school that never agreed records nothing, and its sample stays hidden.
func TestResolveTenant_AnalyticsFreigabe_DefaultOff(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveAnalytics(t, resource, slug)
	assert.False(t, resp.Data.AnalyticsFreigabe)
	assert.Zero(t, resp.Data.AnalyticsRecordingSamplePercent,
		"without the Freigabe the client never sees a recording share")
}

func TestResolveTenant_AnalyticsFreigabe_OnWithSample(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyAnalyticsFreigabe, true, nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyAnalyticsRecordingSamplePercent, 40, nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveAnalytics(t, resource, slug)
	assert.True(t, resp.Data.AnalyticsFreigabe)
	assert.Equal(t, 40, resp.Data.AnalyticsRecordingSamplePercent)
}

// An unreadable Freigabe never turns recording on.
func TestResolveTenant_AnalyticsFreigabe_UnreadableValueFailsClosed(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)
	testutil.StoreRawTenantSetting(t, db, scope.TenantID,
		settings.KeyAnalyticsFreigabe, json.RawMessage(`"vielleicht"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveAnalytics(t, resource, slug)
	assert.False(t, resp.Data.AnalyticsFreigabe)
	assert.Zero(t, resp.Data.AnalyticsRecordingSamplePercent)
}
