// operations.weekend_follows_friday on the public GET /auth/tenant/resolve
// endpoint (#3921). Day pickers and day views of every role read it from the
// school context to stop skipping Saturday and Sunday.
package account_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

func resolveWeekendFollowsFriday(t *testing.T, resource *authAPI.Resource, slug string) bool {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

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

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	assert.False(t, resolveWeekendFollowsFriday(t, resource, slug))
}

func TestResolveTenant_WeekendFollowsFriday_SchoolOverride(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)
	require.NoError(t, tenantSettings.SetValue(scope.Context(), settings.KeyWeekendFollowsFriday, true, nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	assert.True(t, resolveWeekendFollowsFriday(t, resource, slug))
}

// An unreadable value keeps the weekend closed instead of guessing.
func TestResolveTenant_WeekendFollowsFriday_UnreadableValueFailsRequest(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)
	storeRawTenantSetting(t, db, scope.TenantID, settings.KeyWeekendFollowsFriday, json.RawMessage(`"ja"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+slug, nil))
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), `"data"`)
}
