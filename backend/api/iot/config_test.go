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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configMockSettings implements configSvc.SettingsService for testing getDeviceConfig.
type configMockSettings struct {
	boolValues   map[string]bool
	stringValues map[string]string
}

func (m *configMockSettings) GetSchema(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *configMockSettings) GetSchemaForOperator(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *configMockSettings) Resolve(_ context.Context, key string) (any, error) {
	if v, ok := m.stringValues[key]; ok {
		return v, nil
	}
	if v, ok := m.boolValues[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}
func (m *configMockSettings) ResolveString(_ context.Context, key string) (string, error) {
	if v, ok := m.stringValues[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s", key)
}
func (m *configMockSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	return m.ResolveString(context.Background(), key)
}
func (m *configMockSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if v, ok := m.boolValues[key]; ok {
		return v, nil
	}
	return false, fmt.Errorf("not found: %s", key)
}
func (m *configMockSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return m.ResolveBool(context.Background(), key)
}
func (m *configMockSettings) ResolveInt(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *configMockSettings) ResolveIntForTenant(_ context.Context, _ int64, _ string) (int, error) {
	return 0, nil
}
func (m *configMockSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	_, exists := m.stringValues[key]
	return exists, nil
}
func (m *configMockSettings) SetValue(_ context.Context, _ string, _ any, _ *int64, _ []string) error {
	return nil
}
func (m *configMockSettings) ResetValue(_ context.Context, _ string, _ *int64, _ []string) error {
	return nil
}
func (m *configMockSettings) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (m *configMockSettings) SetLoginImageURL(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (m *configMockSettings) ClearLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func TestGetDeviceConfig_AllDefaults(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &configMockSettings{
			boolValues: map[string]bool{
				"checkout.raumwechsel_enabled": true,
				"checkout.schulhof_enabled":    true,
				"checkout.wc_enabled":          true,
				"feedback.enabled":             true,
			},
			stringValues: map[string]string{},
		},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &configMockSettings{
			boolValues: map[string]bool{
				"checkout.raumwechsel_enabled": false, // typically off in binary (no rooms)
				"checkout.schulhof_enabled":    true,  // binary + schulhof → 3-button kiosk
				"checkout.wc_enabled":          false, // WC is visit-only; hidden in binary
				"feedback.enabled":             false,
			},
			stringValues: map[string]string{
				"operations.presence_mode": "binary",
			},
		},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &configMockSettings{
			boolValues: map[string]bool{
				"checkout.raumwechsel_enabled": false,
				"checkout.schulhof_enabled":    false,
				"checkout.wc_enabled":          true,
				"feedback.enabled":             false,
			},
			stringValues: map[string]string{},
		},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &configMockSettings{
			boolValues: map[string]bool{
				"checkout.raumwechsel_enabled": true,
				"checkout.schulhof_enabled":    true,
				"checkout.wc_enabled":          true,
				"feedback.enabled":             true,
			},
			stringValues: map[string]string{
				"operations.student_daily_checkout_time": "16:30",
			},
		},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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

func TestGetDeviceConfig_EnvVarFallback(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{
		SettingsService: &configMockSettings{
			boolValues: map[string]bool{
				"checkout.raumwechsel_enabled": true,
				"checkout.schulhof_enabled":    true,
				"checkout.wc_enabled":          true,
				"feedback.enabled":             true,
			},
			stringValues: map[string]string{},
		},
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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
	rs := &Resource{}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	w := httptest.NewRecorder()

	rs.getDeviceConfig(w, req)

	// Should return 401 when no device context
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestGetDeviceConfig_NilSettingsService(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: nil,
	}

	req := httptest.NewRequest("GET", "/api/iot/config", nil)
	ctx := context.WithValue(req.Context(), device.CtxDevice, &iot.Device{TenantModel: base.TenantModel{TenantID: 1}})
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

	// Feedback defaults to false (opt-in / GDPR) when no settings service
	feedback := data["feedback"].(map[string]any)
	assert.Equal(t, false, feedback["enabled"])
}
