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
		"operations.sick_clear_mode",
		"operations.excused_clear_mode",
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
		// Timetable settings (WP-B7): 6 in operations tab + 1 in gdpr tab.
		"timetable.materialization_enabled",
		"timetable.materialization_weekday",
		"timetable.materialization_weeks_ahead",
		"timetable.auto_start_planned",
		"timetable.overdue_threshold_minutes",
		"timetable.show_expected_children_count",
		"gdpr.timetable_retention_days",
	}

	for _, key := range expectedKeys {
		def := all[key]
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.NotEmpty(t, def.Label, "setting %q should have a label", key)
		assert.NotEmpty(t, def.Tab, "setting %q should have a tab", key)
		assert.NotEmpty(t, def.Category, "setting %q should have a category", key)
	}

	// 28 pre-WP-B7 settings + 7 timetable settings + 2 sick/excused clear-mode
	// settings == 37 minimum. The `>=` is intentional so later work packages can
	// add more settings without retrofitting this assertion.
	assert.GreaterOrEqual(t, len(all), 37, "at least 37 settings should be registered (28 existing + 7 timetable + 2 clear-mode)")
}

func TestTimetableSettings_Types(t *testing.T) {
	tests := []struct {
		key      string
		expected config.FieldType
	}{
		{"timetable.materialization_enabled", config.FieldBoolean},
		{"timetable.materialization_weekday", config.FieldSelect},
		{"timetable.materialization_weeks_ahead", config.FieldNumber},
		{"timetable.auto_start_planned", config.FieldBoolean},
		{"timetable.overdue_threshold_minutes", config.FieldNumber},
		{"timetable.show_expected_children_count", config.FieldBoolean},
		{"gdpr.timetable_retention_days", config.FieldNumber},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

func TestTimetableSettings_Defaults(t *testing.T) {
	// Both the materialization and auto-start flags default to FALSE so that
	// WP-B7 is a pure no-op until the consuming services (WP-B8 / B9) ship
	// AND a tenant explicitly opts in. Regressing either of these defaults
	// would silently activate incomplete features for every tenant.
	matDef := config.GetDefinition("timetable.materialization_enabled")
	require.NotNil(t, matDef)
	assert.Equal(t, false, matDef.Default, "materialization must default to false")

	autoStartDef := config.GetDefinition("timetable.auto_start_planned")
	require.NotNil(t, autoStartDef)
	assert.Equal(t, false, autoStartDef.Default, "auto-start must default to false")

	// Weekday defaults to Friday (ISO 8601: 5) so the RFC's recommended
	// "materialize Fri → next week ready Mon" cadence holds out of the box.
	weekdayDef := config.GetDefinition("timetable.materialization_weekday")
	require.NotNil(t, weekdayDef)
	assert.Equal(t, 5, weekdayDef.Default, "materialization weekday must default to Friday (5)")

	retentionDef := config.GetDefinition("gdpr.timetable_retention_days")
	require.NotNil(t, retentionDef)
	assert.Equal(t, 365, retentionDef.Default, "timetable retention must default to 365 days")
}

func TestTimetableSettings_DependsOn(t *testing.T) {
	// Materialization sub-settings are gated on the top-level toggle.
	gatedKeys := []string{
		"timetable.materialization_weekday",
		"timetable.materialization_weeks_ahead",
	}
	for _, key := range gatedKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.DependsOn, "setting %q should have DependsOn", key)
		assert.Equal(t, "timetable.materialization_enabled", def.DependsOn.Key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	// Retention hangs off the shared GDPR cleanup toggle — same pattern as
	// the other gdpr.* time/timeout settings.
	retentionDef := config.GetDefinition("gdpr.timetable_retention_days")
	require.NotNil(t, retentionDef)
	require.NotNil(t, retentionDef.DependsOn)
	assert.Equal(t, "gdpr.data_cleanup_enabled", retentionDef.DependsOn.Key)

	// Overdue threshold is independent — materialization can be off while
	// staff still see passive "this instance is overdue" indicators.
	overdueDef := config.GetDefinition("timetable.overdue_threshold_minutes")
	require.NotNil(t, overdueDef)
	assert.Nil(t, overdueDef.DependsOn, "overdue threshold must stand alone (no DependsOn)")
}

func TestTimetableSettings_Permissions(t *testing.T) {
	// Operational timetable settings use config:update (admin operational).
	// The GDPR retention setting uses config:manage (admin GDPR-scoped).
	opsKeys := []string{
		"timetable.materialization_enabled",
		"timetable.materialization_weekday",
		"timetable.materialization_weeks_ahead",
		"timetable.auto_start_planned",
		"timetable.overdue_threshold_minutes",
		"timetable.show_expected_children_count",
	}
	for _, key := range opsKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equal(t, "operations", def.Tab, "setting %q should be in operations tab", key)
		assert.Equal(t, "config:update", def.WritePermission, "setting %q should use config:update", key)
	}

	retentionDef := config.GetDefinition("gdpr.timetable_retention_days")
	require.NotNil(t, retentionDef)
	assert.Equal(t, "gdpr", retentionDef.Tab)
	assert.Equal(t, "config:manage", retentionDef.WritePermission, "GDPR settings must use config:manage")
}

func TestTimetableSettings_WeekdayOptions(t *testing.T) {
	def := config.GetDefinition("timetable.materialization_weekday")
	require.NotNil(t, def)
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 7, "all 7 weekdays must be offered")

	// Weekday option values must be ISO 8601 integers 1–7. Drifting to
	// time.Weekday's 0–6 convention would silently break any future
	// materialization consumer that compares to time.Weekday()+1.
	seen := map[int]string{}
	for _, opt := range def.Options.Static {
		v, ok := opt.Value.(int)
		require.Truef(t, ok, "weekday option %q value should be int, got %T", opt.Label, opt.Value)
		require.GreaterOrEqualf(t, v, 1, "weekday option %q value %d must be >= 1", opt.Label, v)
		require.LessOrEqualf(t, v, 7, "weekday option %q value %d must be <= 7", opt.Label, v)
		_, dup := seen[v]
		require.Falsef(t, dup, "weekday option value %d appears twice", v)
		seen[v] = opt.Label
	}
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
		{"operations.sick_clear_mode", config.FieldSelect},
		{"operations.excused_clear_mode", config.FieldSelect},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

// TestStatusFlagClearMode_Defaults guards that the clear-mode settings
// preserve existing behavior (sick clears on next check-in unconditionally,
// new Entschuldigt flow clears at end of day).
func TestStatusFlagClearMode_Defaults(t *testing.T) {
	sickDef := config.GetDefinition(config.KeySickClearMode)
	require.NotNil(t, sickDef)
	assert.Equal(t, "operations", sickDef.Tab)
	assert.Equal(t, "abwesenheit", sickDef.Category)
	assert.Equal(t, "config:update", sickDef.WritePermission)
	assert.Equal(t, config.ClearModeNextCheckin, sickDef.Default,
		"sick default must stay next_checkin to preserve prior behavior")

	excusedDef := config.GetDefinition(config.KeyExcusedClearMode)
	require.NotNil(t, excusedDef)
	assert.Equal(t, "operations", excusedDef.Tab)
	assert.Equal(t, "abwesenheit", excusedDef.Category)
	assert.Equal(t, "config:update", excusedDef.WritePermission)
	assert.Equal(t, config.ClearModeEndOfDay, excusedDef.Default,
		"excused default must be end_of_day per product spec")
}

// TestStatusFlagClearMode_Options confirms both settings offer the full set
// of three modes (manual / next_checkin / end_of_day) so the frontend can
// render a complete select.
func TestStatusFlagClearMode_Options(t *testing.T) {
	for _, key := range []string{config.KeySickClearMode, config.KeyExcusedClearMode} {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.Options, "setting %q should have options", key)
		require.Lenf(t, def.Options.Static, 3, "setting %q should have 3 options", key)

		values := make([]any, 0, 3)
		for _, opt := range def.Options.Static {
			values = append(values, opt.Value)
		}
		assert.Contains(t, values, config.ClearModeManual)
		assert.Contains(t, values, config.ClearModeNextCheckin)
		assert.Contains(t, values, config.ClearModeEndOfDay)
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
