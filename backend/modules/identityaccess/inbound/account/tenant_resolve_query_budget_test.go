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

	"github.com/moto-nrw/project-phoenix/api"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	tenantsettings "github.com/moto-nrw/project-phoenix/modules/settings"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestResolveTenant_IssuesOneSettingValuesQuery(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, authRoute, settings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	// The blanket attachment in api/base.go is what production requests run
	// through; mirror it here so the request cache is present.
	router := chi.NewRouter()
	router.Use(api.SettingsRequestCacheMiddleware)
	router.Mount("/auth", resource.Router(api.TenantSessionChain))

	overrides := []struct {
		key   string
		value any
	}{
		{tenantsettings.KeyTrackingIndicatorsEnabled, false},
		{tenantsettings.KeyTrackingIndicator1, "Mensa"},
		{tenantsettings.KeyTrackingIndicator2, "Hausaufgaben"},
		{tenantsettings.KeyTrackingIndicator3, ""},
		{tenantsettings.KeyEarlyCheckoutNoteEnabled, false},
		{tenantsettings.KeyStudentPhotosEnabled, false},
		{tenantsettings.KeyFeedbackEnabled, false},
		{tenantsettings.KeyDeviceOnlineWindowMinutes, 5},
	}
	counter := testpkg.CaptureQueries(t, db)
	seeded := 0
	for _, n := range []int{1, 3, 8} {
		for seeded < n {
			setting := overrides[seeded]
			require.NoError(t, settings.SetValue(scope.Context(), setting.key, setting.value, nil, nil))
			seeded++
		}
		req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
		rr := httptest.NewRecorder()
		counter.Reset()
		router.ServeHTTP(rr, req)
		queries := counter.Selects("config.setting_values")
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		var resp resolveResp
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, 4, resp.Data.GradeLevelMax,
			"grade cap must still resolve correctly through the batch + cache path")
		t.Logf("stored overrides N=%d: %d setting_values statements", n, len(queries))
		// Tenant resolve must load config.setting_values once, not once per row.
		querybudgets.AssertQueryBudget(t, "api.auth.tenant_resolve.setting_values", queries)
	}
}
