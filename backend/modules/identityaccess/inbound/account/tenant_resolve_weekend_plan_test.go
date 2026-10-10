// operations.weekend_follows_friday on the public GET /auth/tenant/resolve
// endpoint (#3921). Day pickers and day views of every role read it from the
// school context to stop skipping Saturday and Sunday.
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

func resolveWeekendFollowsFriday(t *testing.T, resource *authAPI.Resource, slug string) bool {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			WeekendFollowsFriday bool `json:"weekend_follows_friday"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data.WeekendFollowsFriday
}

func TestResolveTenant_WeekendFollowsFriday_DefaultOff(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.False(t, resolveWeekendFollowsFriday(t, resource, slug))
}

func TestResolveTenant_WeekendFollowsFriday_SchoolOverride(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)
	require.NoError(t, tenantSettings.SetValue(scope.Context(), settings.KeyWeekendFollowsFriday, true, nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.True(t, resolveWeekendFollowsFriday(t, resource, slug))
}

// An unreadable value keeps the weekend closed instead of guessing.
func TestResolveTenant_WeekendFollowsFriday_UnreadableValueFailsClosed(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)
	testutil.StoreRawTenantSetting(t, db, scope.TenantID, settings.KeyWeekendFollowsFriday, json.RawMessage(`"ja"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.False(t, resolveWeekendFollowsFriday(t, resource, slug))
}
