package iot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/device"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeviceTenantID is an arbitrary tenant id for the fully mocked
// getDeviceConfig tests. They never touch the database, so they must not
// request a real per-test tenant.
const mockDeviceTenantID int64 = 987654

// newConfigMock supplies resolved settings for the handler tests.
// Registry default behavior is verified by the settings service's own tests.
func newConfigMock(boolValues map[string]bool, stringValues map[string]string) *configtest.Mock {
	resolveString := func(_ context.Context, key string) (string, error) {
		if v, ok := stringValues[key]; ok {
			return v, nil
		}
		switch key {
		case "operations.student_daily_checkout_time":
			return "", nil
		case "operations.presence_mode":
			return "detailed", nil
		}
		return "", fmt.Errorf("not found: %s", key)
	}
	resolveBool := func(_ context.Context, key string) (bool, error) {
		if v, ok := boolValues[key]; ok {
			return v, nil
		}
		switch key {
		case "checkout.raumwechsel_enabled", "checkout.schulhof_enabled", "checkout.wc_enabled":
			return true, nil
		case "feedback.enabled":
			return false, nil
		}
		return false, fmt.Errorf("not found: %s", key)
	}
	return &configtest.Mock{
		ResolveFn: func(_ context.Context, key string) (any, error) {
			if v, ok := stringValues[key]; ok {
				return v, nil
			}
			if v, ok := boolValues[key]; ok {
				return v, nil
			}
			return nil, fmt.Errorf("not found: %s", key)
		},
		ResolveStringFn: resolveString,
		ResolveStringForTenantFn: func(ctx context.Context, _ int64, key string) (string, error) {
			return resolveString(ctx, key)
		},
		ResolveBoolFn: resolveBool,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
		HasTenantOverrideFn: func(_ context.Context, key string) (bool, error) {
			_, exists := stringValues[key]
			return exists, nil
		},
	}
}

func TestGetDeviceConfig_AllDefaults(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "response should have data field")

	checkout, ok := data["checkout"].(map[string]any)
	require.True(t, ok, "data should have checkout field")
	assert.Equal(t, true, checkout["raumwechsel_enabled"])
	assert.Equal(t, true, checkout["schulhof_enabled"])
	assert.Equal(t, true, checkout["wc_enabled"])
	assert.Nil(t, checkout["daily_checkout_time"], "should be null when not configured")

	feedback, ok := data["feedback"].(map[string]any)
	require.True(t, ok, "data should have feedback field")
	assert.Equal(t, true, feedback["enabled"])

	// Default presence_mode is "detailed" (backwards-compatible for tenants
	// that never configured the setting).
	assert.Equal(t, "detailed", data["presence_mode"])
}

func TestGetDeviceConfig_PresenceModeBinary(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false, // typically off in binary (no rooms)
			"checkout.schulhof_enabled":    true,  // binary + schulhof → 3-button kiosk
			"checkout.wc_enabled":          false, // WC is visit-only; hidden in binary
			"feedback.enabled":             false,
		},
		map[string]string{
			"operations.presence_mode": "binary",
		},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	assert.Equal(t, "binary", data["presence_mode"], "binary-mode tenants must advertise binary so the kiosk branches its UX")
}

func TestGetDeviceConfig_ButtonsDisabled(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false,
			"checkout.schulhof_enabled":    false,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             false,
		},
		map[string]string{},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Equal(t, false, checkout["raumwechsel_enabled"])
	assert.Equal(t, false, checkout["schulhof_enabled"])
	assert.Equal(t, true, checkout["wc_enabled"])

	feedback := data["feedback"].(map[string]any)
	assert.Equal(t, false, feedback["enabled"])
}

func TestGetDeviceConfig_WithDailyCheckoutTime(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{
			"operations.student_daily_checkout_time": "16:30",
		},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Equal(t, "16:30", checkout["daily_checkout_time"])
}

func TestGetDeviceConfig_ExplicitEmptyCheckoutTime(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{"operations.student_daily_checkout_time": ""},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Nil(t, checkout["daily_checkout_time"], "an explicit empty setting permits checkout at any time")
}

func TestGetDeviceConfig_BatchFailure(t *testing.T) {
	t.Parallel()

	settings := newConfigMock(nil, nil)
	settings.ResolveManyForTenantFn = func(context.Context, int64, []string) (*configSvc.SettingsSnapshot, error) {
		return nil, fmt.Errorf("settings unavailable")
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: settings}}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to resolve device configuration")
}

func TestGetDeviceConfig_NoDeviceContext(t *testing.T) {
	t.Parallel()

	rs := &Resource{}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	// Should return 401 when no device context
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestGetDeviceConfig_NilSettingsService(t *testing.T) {
	t.Parallel()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: nil}}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to resolve device configuration")
}

func TestGetDeviceConfig_ResolutionFailure(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"operations.student_daily_checkout_time",
		"checkout.raumwechsel_enabled",
		"checkout.schulhof_enabled",
		"checkout.wc_enabled",
		"feedback.enabled",
		"operations.presence_mode",
	} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			settings := newConfigMock(nil, nil)
			strings := settings.ResolveStringForTenantFn
			bools := settings.ResolveBoolForTenantFn
			settings.ResolveStringForTenantFn = func(ctx context.Context, tenantID int64, requested string) (string, error) {
				require.Equal(t, mockDeviceTenantID, tenantID)
				if requested == key {
					return "", fmt.Errorf("settings unavailable")
				}
				return strings(ctx, tenantID, requested)
			}
			settings.ResolveBoolForTenantFn = func(ctx context.Context, tenantID int64, requested string) (bool, error) {
				require.Equal(t, mockDeviceTenantID, tenantID)
				if requested == key {
					return false, fmt.Errorf("settings unavailable")
				}
				return bools(ctx, tenantID, requested)
			}
			rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: settings}}
			req := httptest.NewRequest("GET", "/api/iot/config", nil)
			req = req.WithContext(context.WithValue(req.Context(), device.CtxDevice, &device.AuthenticatedDevice{TenantID: mockDeviceTenantID}))
			w := httptest.NewRecorder()
			rs.getDeviceConfig(w, req)
			require.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Contains(t, w.Body.String(), "failed to resolve device configuration")
		})
	}
}
