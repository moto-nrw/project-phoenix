// Emergency-list enrichment of the public GET /auth/tenant/resolve endpoint
// (#2609). The Notfall page tells staff what the printed list contains, and
// every member of staff can open it without carrying config:read — so the
// health-column switch has to travel on the same shell metadata as the other
// feature flags, not via /api/settings/schema.
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

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

	db, svc := testutil.SetupAPITest(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = svc.Settings

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

	db, svc := testutil.SetupAPITest(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t,
		svc.Settings.SetValue(ctx, configModel.KeyEmergencyListHealthInfo, false, nil, nil),
		"disable emergency_list_health_info for the isolated tenant")
	t.Cleanup(func() {
		require.NoError(t, svc.Settings.ResetValue(ctx, configModel.KeyEmergencyListHealthInfo, nil, nil))
	})

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = svc.Settings

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
