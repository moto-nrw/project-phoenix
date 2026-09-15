package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockDeviceTenantID int64 = 987654

type configurationResolverStub struct {
	ResolveStringForTenantFn func(context.Context, int64, string) (string, error)
	ResolveBoolForTenantFn   func(context.Context, int64, string) (bool, error)
	ResolveManyForTenantFn   func(context.Context, int64, []string) (*configurationSnapshot, error)
}

func (s *configurationResolverStub) ResolveStringForTenant(ctx context.Context, id int64, key string) (string, error) {
	return s.ResolveStringForTenantFn(ctx, id, key)
}
func (s *configurationResolverStub) ResolveBoolForTenant(ctx context.Context, id int64, key string) (bool, error) {
	return s.ResolveBoolForTenantFn(ctx, id, key)
}
func (s *configurationResolverStub) ResolveManyForTenant(ctx context.Context, id int64, keys []string) (*configurationSnapshot, error) {
	if s.ResolveManyForTenantFn == nil {
		return nil, nil
	}
	return s.ResolveManyForTenantFn(ctx, id, keys)
}

func TestConfigurationResolutionFailuresRemainErrors(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"operations.student_daily_checkout_time", "checkout.raumwechsel_enabled", "checkout.schulhof_enabled", "checkout.wc_enabled", "feedback.enabled", "operations.presence_mode"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			settings := newConfigMock(nil, nil)
			strings, bools := settings.ResolveStringForTenantFn, settings.ResolveBoolForTenantFn
			settings.ResolveStringForTenantFn = func(ctx context.Context, id int64, requested string) (string, error) {
				require.Equal(t, mockDeviceTenantID, id)
				if requested == key {
					return "", fmt.Errorf("settings unavailable")
				}
				return strings(ctx, id, requested)
			}
			settings.ResolveBoolForTenantFn = func(ctx context.Context, id int64, requested string) (bool, error) {
				require.Equal(t, mockDeviceTenantID, id)
				if requested == key {
					return false, fmt.Errorf("settings unavailable")
				}
				return bools(ctx, id, requested)
			}
			_, err := (configurationSettings{settings}).LoadConfiguration(t.Context(), mockDeviceTenantID)
			require.ErrorContains(t, err, "settings unavailable")
		})
	}
	settings := newConfigMock(nil, nil)
	settings.ResolveManyForTenantFn = func(_ context.Context, id int64, keys []string) (*configurationSnapshot, error) {
		require.Equal(t, mockDeviceTenantID, id)
		require.Len(t, keys, 6)
		return nil, fmt.Errorf("settings unavailable")
	}
	_, err := (configurationSettings{settings}).LoadConfiguration(t.Context(), mockDeviceTenantID)
	require.ErrorContains(t, err, "settings unavailable")
	_, err = (configurationSettings{}).LoadConfiguration(t.Context(), mockDeviceTenantID)
	require.ErrorContains(t, err, "device configuration requires settings service")
}

func newConfigMock(boolValues map[string]bool, stringValues map[string]string) *configurationResolverStub {
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
	return &configurationResolverStub{
		ResolveStringForTenantFn: func(ctx context.Context, _ int64, key string) (string, error) { return resolveString(ctx, key) },
		ResolveBoolForTenantFn:   func(ctx context.Context, _ int64, key string) (bool, error) { return resolveBool(ctx, key) },
	}
}

func TestGetDeviceConfig_AllDefaults(t *testing.T) {
	t.Parallel()

	rs := configurationSettings{settings: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{},
	)}

	value, err := rs.LoadConfiguration(context.Background(), mockDeviceTenantID)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	response := map[string]any{"data": payload}

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

	rs := configurationSettings{settings: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false, // typically off in binary (no rooms)
			"checkout.schulhof_enabled":    true,  // binary + schulhof → 3-button kiosk
			"checkout.wc_enabled":          false, // WC is visit-only; hidden in binary
			"feedback.enabled":             false,
		},
		map[string]string{
			"operations.presence_mode": "binary",
		},
	)}

	value, err := rs.LoadConfiguration(context.Background(), mockDeviceTenantID)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	response := map[string]any{"data": payload}

	data := response["data"].(map[string]any)
	assert.Equal(t, "binary", data["presence_mode"], "binary-mode tenants must advertise binary so the kiosk branches its UX")
}

func TestGetDeviceConfig_ButtonsDisabled(t *testing.T) {
	t.Parallel()

	rs := configurationSettings{settings: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": false,
			"checkout.schulhof_enabled":    false,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             false,
		},
		map[string]string{},
	)}

	value, err := rs.LoadConfiguration(context.Background(), mockDeviceTenantID)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	response := map[string]any{"data": payload}

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

	rs := configurationSettings{settings: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{
			"operations.student_daily_checkout_time": "16:30",
		},
	)}

	value, err := rs.LoadConfiguration(context.Background(), mockDeviceTenantID)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	response := map[string]any{"data": payload}

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Equal(t, "16:30", checkout["daily_checkout_time"])
}

func TestGetDeviceConfig_ExplicitEmptyCheckoutTime(t *testing.T) {
	t.Parallel()

	rs := configurationSettings{settings: newConfigMock(
		map[string]bool{
			"checkout.raumwechsel_enabled": true,
			"checkout.schulhof_enabled":    true,
			"checkout.wc_enabled":          true,
			"feedback.enabled":             true,
		},
		map[string]string{"operations.student_daily_checkout_time": ""},
	)}

	value, err := rs.LoadConfiguration(context.Background(), mockDeviceTenantID)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	response := map[string]any{"data": payload}

	data := response["data"].(map[string]any)
	checkout := data["checkout"].(map[string]any)
	assert.Nil(t, checkout["daily_checkout_time"], "an explicit empty setting permits checkout at any time")
}
