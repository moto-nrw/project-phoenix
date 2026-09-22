// Attendance edit scope on the public GET /auth/tenant/resolve endpoint
// (#3066). Together with the overview scope it tells the staff client whether
// a caregiver may move children they do not supervise, so the Unterwegs list
// offers released rooms only when the server would accept the move.
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

type attendanceEditScopeResolveResp struct {
	Data struct {
		AttendanceEditScope string `json:"attendance_edit_scope"`
	} `json:"data"`
}

func resolveAttendanceEditScope(t *testing.T, slug string, resource *authAPI.Resource) string {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp attendanceEditScopeResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data.AttendanceEditScope
}

func TestTenantResolveEmitsAttendanceEditScope(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.Equal(t, settings.AttendanceEditScopeOwn, resolveAttendanceEditScope(t, slug, resource),
		"a school that never touched the setting gets the registry default")

	require.NoError(t,
		tenantSettings.SetValue(scope.Context(), settings.KeyAttendanceEditScope,
			settings.AttendanceEditScopeAllStaff, nil, nil),
		"let all staff edit attendance for the isolated tenant")
	assert.Equal(t, settings.AttendanceEditScopeAllStaff, resolveAttendanceEditScope(t, slug, resource),
		"a tenant override must round-trip to the tenant shell")
}

// A stored value the registry no longer knows must not widen the client: the
// restrictive reading wins, so the UI offers no move the server would refuse.
func TestTenantResolveAttendanceEditScopeUnknownValueFallsBackToOwn(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	// Stored straight through the repository: SetValue would reject a value
	// outside the registry options.
	testutil.StoreRawTenantSetting(t, db, scope.TenantID,
		settings.KeyAttendanceEditScope, json.RawMessage(`"irgendwas"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	assert.Equal(t, settings.AttendanceEditScopeOwn, resolveAttendanceEditScope(t, slug, resource))
}
