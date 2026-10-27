// The public GET /auth/tenant/resolve endpoint answers anonymous callers. The
// school switches the app shell reads after sign-in (#4996) travel on
// /api/me/navigation instead and must never appear here.
package account_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

func TestResolveTenant_KeepsSignedInSwitchesPrivate(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	// Non-default values, so a leak could not hide behind a zero value.
	ctx := scope.Context()
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyGuardianParentInviteMode,
		settings.ParentInviteModeStaffApproval, nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyTimetableDayStartTime, "07:30", nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+slug, nil))
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	// Anker: the public shell metadata is there.
	assert.Contains(t, resp.Data, "timetable_enabled")
	for _, key := range []string{
		"school_switches",
		"parent_news_enabled",
		"parent_invite_mode",
		"timetable_day_start_time",
		"timetable_day_end_time",
	} {
		assert.NotContains(t, resp.Data, key, "%s belongs to the signed-in context", key)
	}
	assert.NotContains(t, rr.Body.String(), "staff_approval")
	assert.NotContains(t, rr.Body.String(), "07:30")
}
