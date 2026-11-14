// The tests pin operations.simple_staff_navigation on the public GET
// /auth/tenant/resolve endpoint (#6956). Every staff member's navigation reads
// it from the school context, also without config:read.
package account_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rootAPI "github.com/moto-ogs/project-phoenix/api"
	transactionFailures "github.com/moto-ogs/project-phoenix/modules/transactionruntime"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identityaccessCompose "github.com/moto-ogs/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-ogs/project-phoenix/modules/settings"
)

func resolveSimpleStaffNavigation(t *testing.T, resource *authAPI.Resource, slug string) bool {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			SimpleStaffNavigation bool `json:"simple_staff_navigation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data.SimpleStaffNavigation
}

func TestResolveTenant_SimpleStaffNavigation_DefaultOff(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	assert.False(t, resolveSimpleStaffNavigation(t, resource, slug))
}

func TestResolveTenant_SimpleStaffNavigation_SchoolOverride(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)
	require.NoError(t, tenantSettings.SetValue(scope.Context(), settings.KeySimpleStaffNavigation, true, nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	assert.True(t, resolveSimpleStaffNavigation(t, resource, slug))
}

// An unreadable value fails the shell instead of guessing navigation.
func TestResolveTenant_SimpleStaffNavigation_UnreadableValueFailsRequest(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)
	storeRawTenantSetting(t, db, scope.TenantID, settings.KeySimpleStaffNavigation, json.RawMessage(`"ja"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+slug, nil))
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), `"data"`)
}
