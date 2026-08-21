package iot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeviceTenantID is an arbitrary tenant id for the fully mocked
// getDeviceConfig tests. They never touch the database, so they must not
// request a real per-test tenant: doing so would load the project .env
// mid-test and undo the os.Unsetenv these tests rely on.
const mockDeviceTenantID int64 = 987654

// newConfigMock builds a configtest.Mock reproducing the behavior of the
// former hand-rolled configMockSettings stub for testing getDeviceConfig:
// string values take precedence over bool values in Resolve, and missing
// keys return a "not found" error instead of a zero value.
func newConfigMock(boolValues map[string]bool, stringValues map[string]string) *configtest.Mock {
	resolveString := func(_ context.Context, key string) (string, error) {
		if v, ok := stringValues[key]; ok {
			return v, nil
		}
		return "", fmt.Errorf("not found: %s", key)
	}
	resolveBool := func(_ context.Context, key string) (bool, error) {
		if v, ok := boolValues[key]; ok {
			return v, nil
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

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestGetDeviceConfig_AllDefaults(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
		},
		map[string]string{},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
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

	// Default presence_mode is "detailed" (backwards-compatible for tenants
	// that never configured the setting).
	assert.Equal(t, "detailed", data["presence_mode"])
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestGetDeviceConfig_PresenceModeBinary(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false, // typically off in binary (no rooms)
			"checkout.schulhof_enabled":    true,  // binary + schulhof → 3-button kiosk
			"checkout.wc_enabled":          false, // WC is visit-only; hidden in binary
		},
		map[string]string{
			"operations.presence_mode": "binary",
		},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	assert.Equal(t, "binary", data["presence_mode"], "binary-mode tenants must advertise binary so the kiosk branches its UX")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestGetDeviceConfig_ButtonsDisabled(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false,
			"checkout.schulhof_enabled":    false,
			"checkout.wc_enabled":          true,
		},
		map[string]string{},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
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
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestGetDeviceConfig_WithDailyCheckoutTime(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
		},
		map[string]string{
			"operations.student_daily_checkout_time": "16:30",
		},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
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

// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDeviceConfig_EnvVarFallback(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
		},
		map[string]string{},
	)},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Equal(t, "14:00", checkout["daily_checkout_time"])
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDeviceConfig_EnvVarFallbackWhenBatchFails(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	settings := newConfigMock(nil, nil)
	settings.ResolveManyForTenantFn = func(context.Context, int64, []string) (*configSvc.SettingsSnapshot, error) {
		return nil, fmt.Errorf("settings unavailable")
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: settings}}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Equal(t, "14:00", checkout["daily_checkout_time"])
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

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestGetDeviceConfig_NilSettingsService(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{ServiceDependencies: ServiceDependencies{SettingsService: nil}}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: mockDeviceTenantID}})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)

	// Checkout buttons default to true when no settings service
	assert.Equal(t, true, checkout["raumwechsel_enabled"])
	assert.Equal(t, true, checkout["schulhof_enabled"])
	assert.Equal(t, true, checkout["wc_enabled"])
	assert.Nil(t, checkout["daily_checkout_time"])
}
