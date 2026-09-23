// Query-budget guard for the public GET /auth/tenant/resolve endpoint
// (issue #2065): the shell-settings batch prefetches every key the handler
// needs — including enrollment.grade_level_max for the separate hard-fail
// resolve — so one request issues exactly ONE config.setting_values query and
// zero additional tenant transactions for settings.
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
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestResolveTenant_IssuesOneSettingValuesQuery(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	// The blanket attachment in api/base.go is what production requests run
	// through; mirror it here so the request cache is present.
	router := chi.NewRouter()
	router.Use(testutil.SettingsRequestCacheMiddleware)
	router.Mount("/auth", resource.Router())

	counter := testpkg.CaptureQueries(t, db)

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	queries := counter.Selects("config.setting_values")

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 4, resp.Data.GradeLevelMax,
		"grade cap must still resolve correctly through the batch + cache path")
	// Tenant resolve must load config.setting_values exactly once per request.
	testpkg.AssertQueryBudget(t, "api.auth.tenant_resolve.setting_values", queries)
}
