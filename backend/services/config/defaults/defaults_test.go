package defaults_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify that all 11 settings are properly registered at init time.
// They run AFTER the defaults package init() functions execute (via blank import).

func TestAllSettingsRegistered(t *testing.T) {
	all := config.AllDefinitions()

	expectedKeys := []string{
		"operations.session_end_enabled",
		"operations.session_end_time",
		"operations.session_end_timeout_minutes",
		"operations.student_daily_checkout_time",
		"operations.session_cleanup_enabled",
		"operations.session_cleanup_interval_minutes",
		"operations.session_abandoned_threshold_minutes",
		"gdpr.data_cleanup_enabled",
		"gdpr.data_cleanup_time",
		"gdpr.data_cleanup_timeout_minutes",
		"security.ogs_device_pin",
	}

	for _, key := range expectedKeys {
		def := all[key]
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.NotEmpty(t, def.Label, "setting %q should have a label", key)
		assert.NotEmpty(t, def.Tab, "setting %q should have a tab", key)
		assert.NotEmpty(t, def.Category, "setting %q should have a category", key)
	}

	assert.GreaterOrEqual(t, len(all), 11, "at least 11 settings should be registered")
}

func TestOperationsSettings_Types(t *testing.T) {
	tests := []struct {
		key      string
		expected config.FieldType
	}{
		{"operations.session_end_enabled", config.FieldBoolean},
		{"operations.session_end_time", config.FieldTime},
		{"operations.session_end_timeout_minutes", config.FieldNumber},
		{"operations.student_daily_checkout_time", config.FieldTime},
		{"operations.session_cleanup_enabled", config.FieldBoolean},
		{"operations.session_cleanup_interval_minutes", config.FieldNumber},
		{"operations.session_abandoned_threshold_minutes", config.FieldNumber},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

func TestGDPRSettings_Types(t *testing.T) {
	tests := []struct {
		key      string
		expected config.FieldType
	}{
		{"gdpr.data_cleanup_enabled", config.FieldBoolean},
		{"gdpr.data_cleanup_time", config.FieldTime},
		{"gdpr.data_cleanup_timeout_minutes", config.FieldNumber},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

func TestSecuritySettings(t *testing.T) {
	def := config.GetDefinition("security.ogs_device_pin")
	require.NotNil(t, def)
	assert.Equal(t, config.FieldPassword, def.Type)
	assert.Equal(t, "security", def.Tab)
	assert.Equal(t, "auth", def.Category)
	assert.Equal(t, "config:manage", def.WritePermission)
}

func TestDependsOn_SessionEndGroup(t *testing.T) {
	// session_end_time depends on session_end_enabled
	timeDef := config.GetDefinition("operations.session_end_time")
	require.NotNil(t, timeDef)
	require.NotNil(t, timeDef.DependsOn)
	assert.Equal(t, "operations.session_end_enabled", timeDef.DependsOn.Key)
	assert.Equal(t, "eq", timeDef.DependsOn.Condition)
	assert.Equal(t, true, timeDef.DependsOn.Value)

	// timeout also depends on session_end_enabled
	timeoutDef := config.GetDefinition("operations.session_end_timeout_minutes")
	require.NotNil(t, timeoutDef)
	require.NotNil(t, timeoutDef.DependsOn)
	assert.Equal(t, "operations.session_end_enabled", timeoutDef.DependsOn.Key)
}

func TestDependsOn_CleanupGroup(t *testing.T) {
	intervalDef := config.GetDefinition("operations.session_cleanup_interval_minutes")
	require.NotNil(t, intervalDef)
	require.NotNil(t, intervalDef.DependsOn)
	assert.Equal(t, "operations.session_cleanup_enabled", intervalDef.DependsOn.Key)

	thresholdDef := config.GetDefinition("operations.session_abandoned_threshold_minutes")
	require.NotNil(t, thresholdDef)
	require.NotNil(t, thresholdDef.DependsOn)
	assert.Equal(t, "operations.session_cleanup_enabled", thresholdDef.DependsOn.Key)
}

func TestDependsOn_GDPRGroup(t *testing.T) {
	timeDef := config.GetDefinition("gdpr.data_cleanup_time")
	require.NotNil(t, timeDef)
	require.NotNil(t, timeDef.DependsOn)
	assert.Equal(t, "gdpr.data_cleanup_enabled", timeDef.DependsOn.Key)

	timeoutDef := config.GetDefinition("gdpr.data_cleanup_timeout_minutes")
	require.NotNil(t, timeoutDef)
	require.NotNil(t, timeoutDef.DependsOn)
	assert.Equal(t, "gdpr.data_cleanup_enabled", timeoutDef.DependsOn.Key)
}

func TestValidation_NumberFields(t *testing.T) {
	// All number fields should have min/max validation
	numberKeys := []string{
		"operations.session_end_timeout_minutes",
		"operations.session_cleanup_interval_minutes",
		"operations.session_abandoned_threshold_minutes",
		"gdpr.data_cleanup_timeout_minutes",
	}

	for _, key := range numberKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.Validation, "setting %q should have validation", key)
		assert.NotNilf(t, def.Validation.Min, "setting %q should have min", key)
		assert.NotNilf(t, def.Validation.Max, "setting %q should have max", key)
	}
}

func TestDefaults_HaveReasonableValues(t *testing.T) {
	tests := []struct {
		key             string
		expectedDefault any
	}{
		{"operations.session_end_enabled", true},
		{"operations.session_end_time", "18:00"},
		{"operations.session_end_timeout_minutes", 10},
		{"operations.student_daily_checkout_time", "15:00"},
		{"operations.session_cleanup_enabled", true},
		{"operations.session_cleanup_interval_minutes", 15},
		{"operations.session_abandoned_threshold_minutes", 60},
		{"gdpr.data_cleanup_enabled", true},
		{"gdpr.data_cleanup_time", "02:00"},
		{"gdpr.data_cleanup_timeout_minutes", 30},
		{"security.ogs_device_pin", ""},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expectedDefault, def.Default, "setting %q default", tc.key)
	}
}
