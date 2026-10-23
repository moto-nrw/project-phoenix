// Settings failures on the public GET /auth/tenant/resolve endpoint.
package account_test

import (
	"context"
	"errors"
	"fmt"
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

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = failingTenantShellSettings{TenantReader: authRoute.SettingsService}

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

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

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "emergency_list_health_info_enabled")
}

type tenantShellReadFailure struct {
	settings.TenantReader
	key string
}

func (s tenantShellReadFailure) ResolveBoolForTenant(ctx context.Context, id int64, key string) (bool, error) {
	if key == s.key {
		return false, errors.New("private tenant policy failure")
	}
	return s.TenantReader.ResolveBoolForTenant(ctx, id, key)
}
func (s tenantShellReadFailure) ResolveStringForTenant(ctx context.Context, id int64, key string) (string, error) {
	if key == s.key {
		return "", errors.New("private tenant policy failure")
	}
	return s.TenantReader.ResolveStringForTenant(ctx, id, key)
}

type tenantShellIncompleteSnapshot struct {
	settings.TenantReader
	key string
}

func (s tenantShellIncompleteSnapshot) ResolveManyForTenant(ctx context.Context, id int64, keys []string) (*settings.Snapshot, error) {
	filtered := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != s.key {
			filtered = append(filtered, key)
		}
	}
	return s.TenantReader.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*settings.Snapshot, error)
	}).ResolveManyForTenant(ctx, id, filtered)
}

func TestResolveTenantRejectsUnreadablePrivacyAndPermissionSettings(t *testing.T) {
	t.Parallel()
	for _, key := range []string{settings.KeyAttendanceLogEnabled, settings.KeyOperationalOverviewScope, settings.KeyAttendanceEditScope, settings.KeyStudentPhotosEnabled, settings.KeyAnalyticsFreigabe, settings.KeyAnalyticsEnabled} {
		for _, batch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/batch=%t", key, batch), func(t *testing.T) {
				t.Parallel()
				db, authRoute := setupAuthDependenciesRoute(t)
				_, slug := newTenantResolveScope(t, db)
				resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
				if batch {
					resource.SettingsService = tenantShellIncompleteSnapshot{authRoute.SettingsService, key}
				} else {
					resource.SettingsService = tenantShellReadFailure{authRoute.SettingsService, key}
				}
				router := chi.NewRouter()
				router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+slug, nil))
				require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
				assert.Contains(t, response.Body.String(), `"code":"general.server"`)
				assert.NotContains(t, response.Body.String(), `"data"`)
				assert.NotContains(t, response.Body.String(), "private tenant policy failure")
			})
		}
	}
}
