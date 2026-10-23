package account_test

import (
	"context"
	"encoding/json"
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
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type resolveCareOfferingsResponse struct {
	Data struct {
		CareOfferingsEnabled bool `json:"care_offerings_enabled"`
	} `json:"data"`
}

func TestResolveTenant_CareOfferingsEnabled(t *testing.T) {
	t.Parallel()

	db, authRoute, tenantSettings := setupAuthDependenciesRouteWithSettings(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	subdomain := fmt.Sprintf("t%d", tenantID)

	cleanupExec := func(query string, args ...any) {
		if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
			t.Logf("cleanup care-offerings tenant fixture: %v", err)
		}
	}
	cleanup := func() {
		cleanupExec(`DELETE FROM config.setting_audit WHERE tenant_id = ? AND setting_key = ?`, tenantID, settings.KeyEnrollmentCareOfferingsEnabled)
		cleanupExec(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`, tenantID, settings.KeyEnrollmentCareOfferingsEnabled)
		cleanupExec(`DELETE FROM platform.schools WHERE id = ?`, tenantID)
		cleanupExec(`DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	}
	t.Cleanup(cleanup)

	schoolService := authRoute.SchoolService
	request := func(t *testing.T, reader settings.TenantReader, status int) resolveCareOfferingsResponse {
		t.Helper()
		resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, schoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
		resource.SettingsService = reader
		router := chi.NewRouter()
		router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))

		req := httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+subdomain, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, status, rr.Code, "Body: %s", rr.Body.String())

		if status != http.StatusOK {
			assert.NotContains(t, rr.Body.String(), `"data"`)
			return resolveCareOfferingsResponse{}
		}
		var response resolveCareOfferingsResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		return response
	}

	t.Run("registry default stays enabled", func(t *testing.T) {
		response := request(t, authRoute.SettingsService, http.StatusOK)
		assert.True(t, response.Data.CareOfferingsEnabled)
	})

	t.Run("tenant override disables editor", func(t *testing.T) {
		ctx := testpkg.TenantContext(tenantID)
		require.NoError(t, tenantSettings.SetValue(ctx, settings.KeyEnrollmentCareOfferingsEnabled, false, nil, nil))
		t.Cleanup(func() {
			cleanupExec(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`, tenantID, settings.KeyEnrollmentCareOfferingsEnabled)
		})

		response := request(t, authRoute.SettingsService, http.StatusOK)
		assert.False(t, response.Data.CareOfferingsEnabled)
	})

	t.Run("resolution error fails closed", func(t *testing.T) {
		settingsErr := errors.New("settings unavailable")
		settings := &settingsDouble{
			ResolveIntForTenantFn: func(_ context.Context, _ int64, key string) (int, error) {
				require.Equal(t, settings.KeyEnrollmentGradeLevelMax, key)
				return 4, nil
			},
			ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
				if key == settings.KeyEnrollmentCareOfferingsEnabled {
					return false, settingsErr
				}
				return false, nil
			},
		}

		request(t, settings, http.StatusInternalServerError)
	})
}
