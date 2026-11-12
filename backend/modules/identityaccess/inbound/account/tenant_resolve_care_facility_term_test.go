// enrollment.care_facility_term on the public GET /auth/tenant/resolve
// endpoint (#6042). The public enrollment form, its status page and the
// embedded parent form read the school's term for the care from the tenant
// context to fill it into their fixed texts.
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

func careFacilityTermResource(route *authAPI.Resource) *authAPI.Resource {
	resource := authAPI.NewResource(route.AuthService, route.Invitations, route.SchoolService, route.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, route.SettingsService)
	resource.SettingsService = route.SettingsService
	return resource
}

func resolveCareFacilityTerm(t *testing.T, resource *authAPI.Resource, slug string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+slug, nil))
	return rr
}

func careFacilityTermOf(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	var resp struct {
		Data struct {
			CareFacilityTerm *string `json:"care_facility_term"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.CareFacilityTerm, "the shell always carries the term")
	return *resp.Data.CareFacilityTerm
}

func TestResolveTenant_CareFacilityTerm_DefaultOGS(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	assert.Equal(t, "OGS", careFacilityTermOf(t, resolveCareFacilityTerm(t, careFacilityTermResource(authRoute), slug)))
}

// Each school reads its own term: a second school of the same database keeps
// the default while the first one chose "GTS".
func TestResolveTenant_CareFacilityTerm_SchoolOverride(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)
	_, otherSlug := newTenantResolveScope(t, db)
	require.NoError(t, tenantSettings.SetValue(scope.Context(), settings.KeyEnrollmentCareFacilityTerm, "GTS", nil, nil))

	resource := careFacilityTermResource(authRoute)
	assert.Equal(t, "GTS", careFacilityTermOf(t, resolveCareFacilityTerm(t, resource, slug)))
	assert.Equal(t, "OGS", careFacilityTermOf(t, resolveCareFacilityTerm(t, resource, otherSlug)))
}

// An unreadable term fails the shell instead of guessing a wording, on the
// single-key and on the batch read.
func TestResolveTenant_CareFacilityTerm_ReadFailureFailsRequest(t *testing.T) {
	t.Parallel()

	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			t.Parallel()
			db, authRoute := setupAuthDependenciesRoute(t)
			_, slug := newTenantResolveScope(t, db)
			resource := careFacilityTermResource(authRoute)
			if batch {
				resource.SettingsService = tenantShellIncompleteSnapshot{authRoute.SettingsService, settings.KeyEnrollmentCareFacilityTerm}
			} else {
				resource.SettingsService = tenantShellReadFailure{authRoute.SettingsService, settings.KeyEnrollmentCareFacilityTerm}
			}

			rr := resolveCareFacilityTerm(t, resource, slug)
			require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
			assert.NotContains(t, rr.Body.String(), `"data"`)
		})
	}
}
