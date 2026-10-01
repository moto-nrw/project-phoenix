// Early-checkout note settings on the public GET /auth/tenant/resolve endpoint
// (#3324). Staff carry no config:read, so the web checkout dialogs read the
// switch and its tolerance from the school context.
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

type earlyCheckoutNoteResolveResp struct {
	Data struct {
		EarlyCheckoutNoteEnabled          bool `json:"early_checkout_note_enabled"`
		EarlyCheckoutNoteToleranceMinutes int  `json:"early_checkout_note_tolerance_minutes"`
	} `json:"data"`
}

func resolveEarlyCheckoutNote(t *testing.T, resource *authAPI.Resource, slug string) earlyCheckoutNoteResolveResp {
	t.Helper()
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp earlyCheckoutNoteResolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

// Without an override the registry default applies: on, 15 minutes.
func TestResolveTenant_EarlyCheckoutNote_DefaultOnWithFifteenMinutes(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveEarlyCheckoutNote(t, resource, slug)
	assert.True(t, resp.Data.EarlyCheckoutNoteEnabled)
	assert.Equal(t, 15, resp.Data.EarlyCheckoutNoteToleranceMinutes)
}

func TestResolveTenant_EarlyCheckoutNote_SchoolOverrides(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyEarlyCheckoutNoteEnabled, false, nil, nil))
	require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyEarlyCheckoutNoteToleranceMinutes, 30, nil, nil))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveEarlyCheckoutNote(t, resource, slug)
	assert.False(t, resp.Data.EarlyCheckoutNoteEnabled)
	assert.Equal(t, 30, resp.Data.EarlyCheckoutNoteToleranceMinutes)
}

// An unreadable tolerance hides the note field instead of guessing a window.
func TestResolveTenant_EarlyCheckoutNote_UnreadableValueFailsClosed(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)
	testutil.StoreRawTenantSetting(t, db, scope.TenantID,
		settings.KeyEarlyCheckoutNoteToleranceMinutes, json.RawMessage(`"viel"`))

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	resp := resolveEarlyCheckoutNote(t, resource, slug)
	assert.False(t, resp.Data.EarlyCheckoutNoteEnabled)
	assert.Zero(t, resp.Data.EarlyCheckoutNoteToleranceMinutes)
}
