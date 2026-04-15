package defaults_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify that all settings are properly registered at init time.
// They run AFTER the defaults package init() functions execute (via blank import).

func TestAllSettingsRegistered(t *testing.T) {
	all := config.AllDefinitions()

	expectedKeys := []string{
		"operations.session_end_enabled",
		"operations.session_end_time",
		"operations.session_end_timeout_minutes",
		"operations.student_daily_checkout_time",
		"operations.per_student_checkout_enabled",
		"operations.per_student_checkout_delta_minutes",
		"operations.session_cleanup_enabled",
		"operations.session_cleanup_interval_minutes",
		"operations.session_abandoned_threshold_minutes",
		"operations.admin_supervision_overview",
		"gdpr.data_cleanup_enabled",
		"gdpr.data_cleanup_time",
		"gdpr.data_cleanup_timeout_minutes",
		"gdpr.attendance_log_enabled",
		"gdpr.attendance_visible_days",
		"gdpr.room_detail_visible_days",
		"gdpr.attendance_log_scope",
		"gdpr.student_data_scope",
		"feedback.enabled",
		"feedback.data_retention_days",
		"security.ogs_device_pin",
		"checkout.raumwechsel_enabled",
		"checkout.schulhof_enabled",
		"checkout.wc_enabled",
		"tracking.indicators_enabled",
		"tracking.indicator_1",
		"tracking.indicator_2",
		"tracking.indicator_3",
	}

	for _, key := range expectedKeys {
		def := all[key]
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.NotEmpty(t, def.Label, "setting %q should have a label", key)
		assert.NotEmpty(t, def.Tab, "setting %q should have a tab", key)
		assert.NotEmpty(t, def.Category, "setting %q should have a category", key)
	}

	assert.GreaterOrEqual(t, len(all), 27, "at least 27 settings should be registered")
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
		{"operations.per_student_checkout_enabled", config.FieldBoolean},
		{"operations.per_student_checkout_delta_minutes", config.FieldNumber},
		{"operations.session_cleanup_enabled", config.FieldBoolean},
		{"operations.session_cleanup_interval_minutes", config.FieldNumber},
		{"operations.session_abandoned_threshold_minutes", config.FieldNumber},
		{"operations.admin_supervision_overview", config.FieldBoolean},
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
		{"gdpr.attendance_log_enabled", config.FieldBoolean},
		{"gdpr.attendance_visible_days", config.FieldNumber},
		{"gdpr.room_detail_visible_days", config.FieldNumber},
		{"gdpr.attendance_log_scope", config.FieldSelect},
		{"gdpr.student_data_scope", config.FieldSelect},
		{"feedback.enabled", config.FieldBoolean},
		{"feedback.data_retention_days", config.FieldNumber},
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
	assert.Equal(t, "devices", def.Tab)
	assert.Equal(t, "pin", def.Category)
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

func TestDependsOn_PerStudentCheckoutGroup(t *testing.T) {
	deltaDef := config.GetDefinition("operations.per_student_checkout_delta_minutes")
	require.NotNil(t, deltaDef)
	require.NotNil(t, deltaDef.DependsOn)
	assert.Equal(t, "operations.per_student_checkout_enabled", deltaDef.DependsOn.Key)
	assert.Equal(t, "eq", deltaDef.DependsOn.Condition)
	assert.Equal(t, true, deltaDef.DependsOn.Value)

	// The toggle itself has no DependsOn
	toggleDef := config.GetDefinition("operations.per_student_checkout_enabled")
	require.NotNil(t, toggleDef)
	assert.Nil(t, toggleDef.DependsOn)
}

func TestDependsOn_AttendanceLogGroup(t *testing.T) {
	dependentKeys := []string{
		"gdpr.attendance_visible_days",
		"gdpr.room_detail_visible_days",
		"gdpr.attendance_log_scope",
	}
	for _, key := range dependentKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.DependsOn, "setting %q should have DependsOn", key)
		assert.Equalf(t, "gdpr.attendance_log_enabled", def.DependsOn.Key, "setting %q should depend on attendance_log_enabled", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}
}

func TestDependsOn_FeedbackGroup(t *testing.T) {
	retentionDef := config.GetDefinition("feedback.data_retention_days")
	require.NotNil(t, retentionDef)
	require.NotNil(t, retentionDef.DependsOn)
	assert.Equal(t, "feedback.enabled", retentionDef.DependsOn.Key)
	assert.Equal(t, "eq", retentionDef.DependsOn.Condition)
	assert.Equal(t, true, retentionDef.DependsOn.Value)
}

func TestFeedbackSettings(t *testing.T) {
	def := config.GetDefinition("feedback.enabled")
	require.NotNil(t, def)
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, "gdpr", def.Tab)
	assert.Equal(t, "feedback", def.Category)
	assert.Equal(t, false, def.Default, "feedback should default to false (opt-in)")
	assert.Equal(t, "config:manage", def.WritePermission)
}

func TestAttendanceLogScope_Options(t *testing.T) {
	def := config.GetDefinition("gdpr.attendance_log_scope")
	require.NotNil(t, def)
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 2)
	values := []any{def.Options.Static[0].Value, def.Options.Static[1].Value}
	assert.Contains(t, values, config.AttendanceLogScopeGroupSupervisorsOnly)
	assert.Contains(t, values, config.AttendanceLogScopeAllStaff)
}

func TestStudentDataScope_Options(t *testing.T) {
	def := config.GetDefinition("gdpr.student_data_scope")
	require.NotNil(t, def)
	assert.Equal(t, "gdpr", def.Tab)
	assert.Equal(t, "schülerdaten", def.Category)
	assert.Equal(t, config.FieldSelect, def.Type)
	assert.Equal(t, config.StudentDataScopeGroupSupervisorsOnly, def.Default)
	assert.Equal(t, "config:manage", def.WritePermission)
	assert.Nil(t, def.DependsOn, "student_data_scope should stand alone, no DependsOn")

	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 2)
	values := []any{def.Options.Static[0].Value, def.Options.Static[1].Value}
	assert.Contains(t, values, config.StudentDataScopeGroupSupervisorsOnly)
	assert.Contains(t, values, config.StudentDataScopeAllStaff)
}

func TestDevicesSettings(t *testing.T) {
	keys := []string{
		"checkout.raumwechsel_enabled",
		"checkout.schulhof_enabled",
		"checkout.wc_enabled",
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equal(t, config.FieldBoolean, def.Type, "setting %q should be boolean", key)
		assert.Equal(t, "devices", def.Tab, "setting %q should be in devices tab", key)
		assert.Equal(t, "checkout", def.Category, "setting %q should be in checkout category", key)
		assert.Equal(t, "config:update", def.WritePermission, "setting %q should use config:update", key)
	}

	// Raumwechsel defaults to true (no system room required)
	raumwechsel := config.GetDefinition("checkout.raumwechsel_enabled")
	assert.Equal(t, true, raumwechsel.Default, "raumwechsel should default to true")

	// Schulhof and WC default to false (opt-in, system rooms created on activation)
	schulhof := config.GetDefinition("checkout.schulhof_enabled")
	assert.Equal(t, false, schulhof.Default, "schulhof should default to false (opt-in)")
	wc := config.GetDefinition("checkout.wc_enabled")
	assert.Equal(t, false, wc.Default, "wc should default to false (opt-in)")
}

func TestStudentDailyCheckoutTime_OptionalDefault(t *testing.T) {
	def := config.GetDefinition("operations.student_daily_checkout_time")
	require.NotNil(t, def)
	assert.Equal(t, "", def.Default, "daily checkout time should default to empty (always available)")
}

func TestValidation_NumberFields(t *testing.T) {
	// All number fields should have min/max validation
	numberKeys := []string{
		"operations.session_end_timeout_minutes",
		"operations.session_cleanup_interval_minutes",
		"operations.session_abandoned_threshold_minutes",
		"gdpr.data_cleanup_timeout_minutes",
		"gdpr.attendance_visible_days",
		"gdpr.room_detail_visible_days",
		"feedback.data_retention_days",
		"operations.per_student_checkout_delta_minutes",
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
		{"operations.student_daily_checkout_time", ""},
		{"operations.per_student_checkout_enabled", false},
		{"operations.per_student_checkout_delta_minutes", 15},
		{"operations.session_cleanup_enabled", false},
		{"operations.session_cleanup_interval_minutes", 15},
		{"operations.session_abandoned_threshold_minutes", 60},
		{"operations.admin_supervision_overview", false},
		{"gdpr.data_cleanup_enabled", true},
		{"gdpr.data_cleanup_time", "02:00"},
		{"gdpr.data_cleanup_timeout_minutes", 30},
		{"gdpr.attendance_log_enabled", false},
		{"gdpr.attendance_visible_days", 30},
		{"gdpr.room_detail_visible_days", 7},
		{"gdpr.attendance_log_scope", config.AttendanceLogScopeGroupSupervisorsOnly},
		{"gdpr.student_data_scope", config.StudentDataScopeGroupSupervisorsOnly},
		{"feedback.enabled", false},
		{"feedback.data_retention_days", 90},
		{"security.ogs_device_pin", "1234"},
		{"checkout.raumwechsel_enabled", true},
		{"checkout.schulhof_enabled", false},
		{"checkout.wc_enabled", false},
		{"tracking.indicators_enabled", false},
		{"tracking.indicator_1", ""},
		{"tracking.indicator_2", ""},
		{"tracking.indicator_3", ""},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expectedDefault, def.Default, "setting %q default", tc.key)
	}
}

func TestTrackingSettings_Types(t *testing.T) {
	tests := []struct {
		key      string
		expected config.FieldType
	}{
		{"tracking.indicators_enabled", config.FieldBoolean},
		{"tracking.indicator_1", config.FieldText},
		{"tracking.indicator_2", config.FieldText},
		{"tracking.indicator_3", config.FieldText},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

func TestDependsOn_TrackingGroup(t *testing.T) {
	dependentKeys := []string{
		"tracking.indicator_1",
		"tracking.indicator_2",
		"tracking.indicator_3",
	}
	for _, key := range dependentKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.DependsOn, "setting %q should have DependsOn", key)
		assert.Equalf(t, "tracking.indicators_enabled", def.DependsOn.Key, "setting %q should depend on indicators_enabled", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}
}

func TestTrackingIndicator_Validation(t *testing.T) {
	indicatorKeys := []string{
		"tracking.indicator_1",
		"tracking.indicator_2",
		"tracking.indicator_3",
	}
	for _, key := range indicatorKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.Validation, "setting %q should have validation", key)
		require.NotNilf(t, def.Validation.Pattern, "setting %q should have pattern", key)
		assert.Equal(t, `^[a-zA-ZäöüÄÖÜß\s]{0,30}$`, *def.Validation.Pattern, "setting %q pattern", key)
	}

	// Verify the toggle has no validation (boolean doesn't need it)
	enabledDef := config.GetDefinition("tracking.indicators_enabled")
	require.NotNil(t, enabledDef)
	assert.Nil(t, enabledDef.Validation, "boolean toggle should have no validation")
}
