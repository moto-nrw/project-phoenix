// Emergency-list enrichment of the public GET /auth/tenant/resolve endpoint
// (#2609). The Notfall page tells staff what the printed list contains, and
// every member of staff can open it without carrying config:read — so the
// health-column switch has to travel on the same shell metadata as the other
// feature flags, not via /api/settings/schema.
package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type failingTenantShellSettings struct {
	configSvc.SettingsService
}

func (failingTenantShellSettings) ResolveManyForTenant(
	context.Context,
	int64,
	[]string,
) (*configSvc.SettingsSnapshot, error) {
	return nil, errors.New("settings unavailable")
}

type emergencyResolveResp struct {
	Data struct {
		EmergencyHealthInfoEnabled bool `json:"emergency_list_health_info_enabled"`
	} `json:"data"`
}

// A school that never touched the setting gets the registry default: the
// health column is printed, which is what the schools asking for the feature
// expect without having to find a switch first.
func TestResolveTenant_EmergencyHealthInfo_DefaultTrue(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp emergencyResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Data.EmergencyHealthInfoEnabled,
		"registry default for emergency_list_health_info is true; resolver must surface that")
}

// A school that switched the column off must see that on the Notfall page,
// otherwise the page promises health data the PDF does not carry.
func TestResolveTenant_EmergencyHealthInfo_OverrideFalse(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t,
		authRoute.SettingsService.SetValue(ctx, configModel.KeyEmergencyListHealthInfo, false, nil, nil),
		"disable emergency_list_health_info for the isolated tenant")

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp emergencyResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.EmergencyHealthInfoEnabled,
		"tenant override of false must round-trip to the tenant shell")
}

// A settings backend that cannot answer at all is not a health-column
// decision: resolveTenant fails the whole contract rather than shipping a
// tenant shell assembled from fallbacks, so nothing is promised to the page.
func TestResolveTenant_EmergencyHealthInfo_SettingFailureFailsRequest(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = failingTenantShellSettings{SettingsService: authRoute.SettingsService}

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
}

// A single unreadable value is the case the resolver absorbs: the rest of the
// shell still loads, and the health column must not be announced off the back
// of a value nobody can interpret — the export omits it in the same situation.
func TestResolveTenant_EmergencyHealthInfo_UnreadableValueFailsClosed(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	// Stored straight through the repository: SetValue would reject the
	// non-boolean, and what we need to pin is the read side meeting a row
	// that no longer matches the registry type.
	stored := &configModel.SettingValue{
		SettingKey: configModel.KeyEmergencyListHealthInfo,
		Value:      json.RawMessage(`"vielleicht"`),
	}
	stored.SetTenantID(scope.TenantID)
	require.NoError(t,
		configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db)).Upsert(scope.Context(), stored),
		"store an unreadable emergency_list_health_info override")

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp emergencyResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.EmergencyHealthInfoEnabled,
		"an unreadable value must not promise health data that the export omits")
}
