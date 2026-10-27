// Settings the app reads outside the settings page travel on the public
// GET /auth/tenant/resolve endpoint (#4996): the navigation, the
// Elternzugänge page and the Betreuungsplan read parent news, the invite mode
// and the planner's day from the tenant shell instead of loading the whole
// /api/settings/schema.
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

type schemaFreeFlagsResolveResp struct {
	Data struct {
		ParentNewsEnabled     *bool   `json:"parent_news_enabled"`
		ParentInviteMode      *string `json:"parent_invite_mode"`
		TimetableDayStartTime *string `json:"timetable_day_start_time"`
		TimetableDayEndTime   *string `json:"timetable_day_end_time"`
	} `json:"data"`
}

func resolveSchemaFreeFlags(t *testing.T, slug string, resource *authAPI.Resource) schemaFreeFlagsResolveResp {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp schemaFreeFlagsResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.ParentNewsEnabled, "parent_news_enabled must always be on the wire")
	require.NotNil(t, resp.Data.ParentInviteMode, "parent_invite_mode must always be on the wire")
	require.NotNil(t, resp.Data.TimetableDayStartTime, "timetable_day_start_time must always be on the wire")
	require.NotNil(t, resp.Data.TimetableDayEndTime, "timetable_day_end_time must always be on the wire")
	return resp
}

func TestTenantResolveEmitsSchemaFreeFlags(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	defaults := resolveSchemaFreeFlags(t, slug, resource)
	assert.True(t, *defaults.Data.ParentNewsEnabled,
		"a school that never touched parent news gets the registry default (on)")
	assert.Equal(t, settings.ParentInviteModeDisabled, *defaults.Data.ParentInviteMode,
		"a school that never touched invitations gets the registry default")
	assert.Equal(t, "09:00", *defaults.Data.TimetableDayStartTime, "registry default day start")
	assert.Equal(t, "17:00", *defaults.Data.TimetableDayEndTime, "registry default day end")

	ctx := scope.Context()
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyParentNewsEnabled, false, nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyGuardianParentInviteMode,
		settings.ParentInviteModeStaffApproval, nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyTimetableDayStartTime, "07:30", nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyTimetableDayEndTime, "16:15", nil, nil))

	overridden := resolveSchemaFreeFlags(t, slug, resource)
	assert.False(t, *overridden.Data.ParentNewsEnabled, "a tenant override must round-trip")
	assert.Equal(t, settings.ParentInviteModeStaffApproval, *overridden.Data.ParentInviteMode,
		"a tenant override must round-trip")
	assert.Equal(t, "07:30", *overridden.Data.TimetableDayStartTime, "a tenant override must round-trip")
	assert.Equal(t, "16:15", *overridden.Data.TimetableDayEndTime, "a tenant override must round-trip")
}

// A stored value the registry no longer knows must not reach the client as a
// mode it would have to guess about: it reads as the registry default.
func TestTenantResolveParentInviteModeUnknownValueFallsBackToDisabled(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	// Stored straight through the repository: SetValue would reject a value
	// outside the registry options.
	storeRawTenantSetting(t, db, scope.TenantID,
		settings.KeyGuardianParentInviteMode, json.RawMessage(`"irgendwas"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	resp := resolveSchemaFreeFlags(t, slug, resource)
	assert.Equal(t, settings.ParentInviteModeDisabled, *resp.Data.ParentInviteMode)
}
