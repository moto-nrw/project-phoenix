// Settings failures on the public GET /auth/tenant/resolve endpoint.
package account_test

import (
	"context"
	"errors"
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

type failingTenantShellSettings struct {
	settings.TenantReader
}

func (failingTenantShellSettings) ResolveManyForTenant(
	context.Context,
	int64,
	[]string,
) (*settings.Snapshot, error) {
	return nil, errors.New("settings unavailable")
}

// A settings backend that cannot answer at all fails the whole contract
// rather than shipping a tenant shell assembled from fallbacks.
func TestResolveTenant_SettingsBatchFailureFailsRequest(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = failingTenantShellSettings{TenantReader: authRoute.SettingsService}

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
}

// The Notfallliste always prints the health column since #3732, so the shell
// no longer carries a switch for it.
func TestResolveTenant_NoEmergencyHealthInfoFlag(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "emergency_list_health_info_enabled")
}
