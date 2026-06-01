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
		"operations.status_flag_clear_time",
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
		// Timetable settings: top-level enable toggle + operations/GDPR details.
		"timetable.enabled",
		"timetable.materialization_enabled",
		"timetable.materialization_weekday",
		"timetable.materialization_weeks_ahead",
		"timetable.auto_start_planned",
		"timetable.overdue_threshold_minutes",
		"timetable.show_expected_children_count",
		"gdpr.timetable_retention_days",
		// Display range for the admin weekly calendar (Apple-style grid).
		"timetable.day_start_time",
		"timetable.day_end_time",
		// Presence-mode work package: tenant presence tracking model + who can check-in via web.
		"operations.presence_mode",
		"attendance.web_checkin_access",
		"attendance.web_spontaneous_activities_enabled",
		// Student photo feature (Datenverwaltung): per-school opt-in toggle.
		"operations.student_photos_enabled",
		// 2FA / MFA work package (issue #1308): mode toggle + trusted-device pair.
		"security.mfa_mode",
		"security.mfa_trusted_device_enabled",
		"security.mfa_trusted_device_days",
		// Parent-enrollment PR 2: activate-students scheduler interval.
		"operations.student_activation_interval_minutes",
		// Parent-enrollment PR 3: guardian invitation token expiry.
		"invitations.guardian_token_expiry_hours",
		// Parent-enrollment registry plumbing. open_window_*,
		// show_status_reason_to_parent, and care_overflow_mode moved
		// to per-phase columns on enrollment.phases - they're no
		// longer tenant-wide settings.
		"enrollment.enabled",
		"enrollment.collect_grade_level",
		"enrollment.care_offerings_enabled",
		"enrollment.care_offerings_required",
		"enrollment.default_activation_mode",
		"enrollment.notification_emails",
		"enrollment.auto_invite_guardian_on_approval",
		"enrollment.duplicate_handling",
		"enrollment.allow_submission_edit",
		"enrollment.require_captcha",
		"enrollment.rejected_retention_days",
		"enrollment.waitlist_enabled",
		"enrollment.notify_per_decision",
		"enrollment.outbox_max_attempts",
		"enrollment.outbox_worker_interval_seconds",
		"enrollment.status_token_ttl_days",
		// PR 7: public form additions.
		"enrollment.captcha_site_key",
		"enrollment.captcha_secret_key",
		"enrollment.grade_level_max",
	}

	for _, key := range expectedKeys {
		def := all[key]
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.NotEmpty(t, def.Label, "setting %q should have a label", key)
		assert.NotEmpty(t, def.Tab, "setting %q should have a tab", key)
		assert.NotEmpty(t, def.Category, "setting %q should have a category", key)
	}

	// The `>=` is intentional so later work packages can add more settings
	// without retrofitting this assertion.
	assert.GreaterOrEqual(t, len(all), len(expectedKeys), "all expected settings should be registered")
}

func TestPresenceModeSetting(t *testing.T) {
	def := config.GetDefinition(config.KeyPresenceMode)
	require.NotNil(t, def, "operations.presence_mode should be registered")
	assert.Equal(t, config.FieldSelect, def.Type)
	assert.Equal(t, config.PresenceModeDetailed, def.Default, "default must be detailed for backwards compatibility")
	assert.Equal(t, config.AccessOperatorOnly, def.AccessPolicy, "presence_mode is operator-only - cascading impact too large for tenant admins")
	assert.Equal(t, "operations", def.Tab)
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 2)
	values := []any{def.Options.Static[0].Value, def.Options.Static[1].Value}
	assert.Contains(t, values, config.PresenceModeDetailed)
	assert.Contains(t, values, config.PresenceModeBinary)
}

func TestWebCheckinAccessSetting(t *testing.T) {
	def := config.GetDefinition(config.KeyWebCheckinAccess)
	require.NotNil(t, def, "attendance.web_checkin_access should be registered")
	assert.Equal(t, config.FieldSelect, def.Type)
	assert.Equal(t, config.WebCheckinAccessGroupSupervisors, def.Default, "default must be the restrictive option (group supervisors only)")
	assert.Equal(t, config.AccessShared, def.AccessPolicy, "web_checkin_access is a tenant-admin decision, not operator-only")
	assert.Equal(t, "operations", def.Tab)
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 2)
}

func TestWebSpontaneousActivitiesSetting(t *testing.T) {
	def := config.GetDefinition(config.KeyWebSpontaneousActivities)
	require.NotNil(t, def, "attendance.web_spontaneous_activities_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "web spontaneous activities must default off")
	assert.Equal(t, config.AccessShared, def.AccessPolicy, "tenant admins and operators should both be able to manage this operational setting")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "anwesenheit", def.Category)
	assert.Equal(t, "config:manage", def.WritePermission)
}

func TestTimetableSettings_Types(t *testing.T) {
	tests := []struct {
		key      string
		expected config.FieldType
	}{
		{"timetable.enabled", config.FieldBoolean},
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
	enabledDef := config.GetDefinition("timetable.enabled")
	require.NotNil(t, enabledDef)
	assert.Equal(t, false, enabledDef.Default, "timetable must default to false")

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
	toggleDef := config.GetDefinition("timetable.enabled")
	require.NotNil(t, toggleDef)
	assert.Nil(t, toggleDef.DependsOn, "top-level timetable toggle must stand alone")

	// All timetable detail settings are hidden until the top-level feature is enabled.
	topLevelGatedKeys := []string{
		"timetable.materialization_enabled",
		"timetable.auto_start_planned",
		"timetable.overdue_threshold_minutes",
		"timetable.show_expected_children_count",
		"gdpr.timetable_retention_days",
		"timetable.day_start_time",
		"timetable.day_end_time",
	}
	for _, key := range topLevelGatedKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.DependsOn, "setting %q should have DependsOn", key)
		assert.Equalf(t, "timetable.enabled", def.DependsOn.Key, "setting %q should depend on timetable.enabled", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	// Materialization sub-settings are gated on the materialization toggle.
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

	// Overdue threshold is independent of materialization - it only depends on
	// the top-level feature toggle.
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

	toggleDef := config.GetDefinition("timetable.enabled")
	require.NotNil(t, toggleDef)
	assert.Equal(t, "operations", toggleDef.Tab)
	assert.Equal(t, "config:update", toggleDef.WritePermission)

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

	// Weekday option values must be ISO 8601 integers 1-7. Drifting to
	// time.Weekday's 0-6 convention would silently break any future
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
		{"operations.status_flag_clear_time", config.FieldTime},
		{"operations.sick_clear_mode", config.FieldSelect},
		{"operations.excused_clear_mode", config.FieldSelect},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

// TestEnrollmentSettings_AllRegistered_OnEnrollmentTab guards that every
// enrollment.* user-facing setting lands on the "enrollment" tab. The two
// system-tab keys (outbox_max_attempts, status_token_ttl_days) are pulled
// out separately so a future tab refactor can't silently move user-visible
// settings without breaking the test.
func TestEnrollmentSettings_AllRegistered_OnEnrollmentTab(t *testing.T) {
	enrollmentTabKeys := []string{
		config.KeyEnrollmentEnabled,
		config.KeyEnrollmentCollectGradeLevel,
		config.KeyEnrollmentCareOfferingsEnabled,
		config.KeyEnrollmentCareOfferingsRequired,
		config.KeyEnrollmentDefaultActivationMode,
		config.KeyEnrollmentNotificationEmails,
		config.KeyEnrollmentAutoInviteGuardianOnApprove,
		config.KeyEnrollmentDuplicateHandling,
		config.KeyEnrollmentAllowSubmissionEdit,
		config.KeyEnrollmentRequireCaptcha,
		config.KeyEnrollmentRejectedRetentionDays,
		config.KeyEnrollmentWaitlistEnabled,
		config.KeyEnrollmentNotifyPerDecision,
	}
	for _, key := range enrollmentTabKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.Equalf(t, "enrollment", def.Tab, "setting %q should be in enrollment tab", key)
		assert.NotEmptyf(t, def.Label, "setting %q must have a German label", key)
		assert.NotEmptyf(t, def.Description, "setting %q must have a German description", key)
	}

	systemTabKeys := []string{
		config.KeyEnrollmentOutboxMaxAttempts,
		config.KeyEnrollmentStatusTokenTTLDays,
	}
	for _, key := range systemTabKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.Equalf(t, "system", def.Tab, "setting %q should be in system tab", key)
	}
}

// TestEnrollmentEnabled_DefaultsOff guards that the master feature flag is
// off by default. Regressing this would silently expose a half-built
// feature to every tenant on next deploy.
func TestEnrollmentEnabled_DefaultsOff(t *testing.T) {
	def := config.GetDefinition(config.KeyEnrollmentEnabled)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "enrollment.enabled must default to false")
	assert.Equal(t, "config:update", def.WritePermission)
}

// TestEnrollmentSettings_DependencyOnEnabled guards that all per-feature
// enrollment settings hide when the master toggle is off (depends_on
// enrollment.enabled = true). Captures the rev-2.x intent that PR 4 is
// pure plumbing and nothing renders in the parent UI until enabled.
func TestEnrollmentSettings_DependencyOnEnabled(t *testing.T) {
	gatedOnEnabled := []string{
		config.KeyEnrollmentCollectGradeLevel,
		config.KeyEnrollmentCareOfferingsEnabled,
		config.KeyEnrollmentDefaultActivationMode,
		config.KeyEnrollmentNotificationEmails,
		config.KeyEnrollmentAutoInviteGuardianOnApprove,
		config.KeyEnrollmentDuplicateHandling,
		config.KeyEnrollmentAllowSubmissionEdit,
		config.KeyEnrollmentRequireCaptcha,
		config.KeyEnrollmentWaitlistEnabled,
		config.KeyEnrollmentNotifyPerDecision,
	}
	for _, key := range gatedOnEnabled {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		require.NotNilf(t, def.DependsOn, "setting %q must depend on enrollment.enabled", key)
		assert.Equal(t, config.KeyEnrollmentEnabled, def.DependsOn.Key, "setting %q parent should be enrollment.enabled", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	// Care-offerings-required hangs off care_offerings_enabled (nested gate),
	// not the master toggle.
	careRequired := config.GetDefinition(config.KeyEnrollmentCareOfferingsRequired)
	require.NotNil(t, careRequired.DependsOn)
	assert.Equal(t, config.KeyEnrollmentCareOfferingsEnabled, careRequired.DependsOn.Key)
}

// TestEnrollmentSelectOptions_AreCanonical guards the static option lists
// for the three select-typed enrollment settings. Drift here breaks the
// frontend select renderer silently.
func TestEnrollmentSelectOptions_AreCanonical(t *testing.T) {
	activation := config.GetDefinition(config.KeyEnrollmentDefaultActivationMode)
	require.NotNil(t, activation.Options)
	require.Len(t, activation.Options.Static, 2)
	activationValues := []any{activation.Options.Static[0].Value, activation.Options.Static[1].Value}
	assert.Contains(t, activationValues, config.EnrollmentActivationModeImmediate)
	assert.Contains(t, activationValues, config.EnrollmentActivationModeScheduled)

	dup := config.GetDefinition(config.KeyEnrollmentDuplicateHandling)
	require.NotNil(t, dup.Options)
	require.Len(t, dup.Options.Static, 3)
	dupValues := []any{
		dup.Options.Static[0].Value,
		dup.Options.Static[1].Value,
		dup.Options.Static[2].Value,
	}
	assert.Contains(t, dupValues, config.EnrollmentDuplicateHandlingBlock)
	assert.Contains(t, dupValues, config.EnrollmentDuplicateHandlingWarn)
	assert.Contains(t, dupValues, config.EnrollmentDuplicateHandlingIgnore)

	notify := config.GetDefinition(config.KeyEnrollmentNotifyPerDecision)
	require.NotNil(t, notify.Options)
	require.Len(t, notify.Options.Static, 2)
	notifyValues := []any{notify.Options.Static[0].Value, notify.Options.Static[1].Value}
	assert.Contains(t, notifyValues, config.EnrollmentNotifyPerDecisionDigest)
	assert.Contains(t, notifyValues, config.EnrollmentNotifyPerDecisionImmediate)
}

// (TestEnrollmentDateFields removed: the open-window settings moved to
// per-phase columns on enrollment.phases - no tenant-wide date pickers
// remain.)

// TestEnrollmentOutboxWorkerInterval guards the registry shape of the
// outbox worker polling interval setting (PR 5). Operator-only because
// the cadence is platform plumbing, not tenant-tunable.
func TestEnrollmentOutboxWorkerInterval(t *testing.T) {
	def := config.GetDefinition(config.KeyEnrollmentOutboxWorkerIntervalSeconds)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 30, def.Default)
	assert.Equal(t, "system", def.Tab)
	assert.Equal(t, "config:manage", def.WritePermission)
	assert.Equal(t, config.AccessOperatorOnly, def.AccessPolicy)
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
	assert.Equal(t, float64(10), *def.Validation.Min)
	assert.Equal(t, float64(600), *def.Validation.Max)
}

// TestEnrollmentStatusTokenTTL_OperatorOnly guards the §11 rule that this
// setting is operator-writable only - readable by tenant admins, not
// editable. We use AccessOperatorOnly + config:manage instead of
// introducing a new platform:config:update permission.
func TestEnrollmentStatusTokenTTL_OperatorOnly(t *testing.T) {
	def := config.GetDefinition(config.KeyEnrollmentStatusTokenTTLDays)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 365, def.Default)
	assert.Equal(t, "system", def.Tab)
	assert.Equal(t, "config:manage", def.WritePermission)
	assert.Equal(t, config.AccessOperatorOnly, def.AccessPolicy,
		"status_token_ttl_days must be operator-only - tenant admins should not extend their own status-link windows")
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
}

// TestEnrollmentSafetyPermissions guards that the captcha and retention
// settings - both with security/GDPR implications - use the stricter
// config:manage write permission, not the operational config:update.
func TestEnrollmentSafetyPermissions(t *testing.T) {
	captcha := config.GetDefinition(config.KeyEnrollmentRequireCaptcha)
	require.NotNil(t, captcha)
	assert.Equal(t, "config:manage", captcha.WritePermission)

	retention := config.GetDefinition(config.KeyEnrollmentRejectedRetentionDays)
	require.NotNil(t, retention)
	assert.Equal(t, "config:manage", retention.WritePermission)
}

// TestGuardianInvitationTokenExpiry guards the registry shape of the
// guardian-invitation token TTL. Operator-only, default 48h, validation 1-168h.
func TestGuardianInvitationTokenExpiry(t *testing.T) {
	def := config.GetDefinition(config.KeyGuardianInvitationTokenExpiryHours)
	require.NotNil(t, def, "invitations.guardian_token_expiry_hours should be registered")
	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 48, def.Default)
	assert.Equal(t, "system", def.Tab)
	assert.Equal(t, "config:manage", def.WritePermission)
	assert.Equal(t, config.AccessOperatorOnly, def.AccessPolicy,
		"guardian token TTL is auth plumbing - operators only")
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
	assert.Equal(t, float64(1), *def.Validation.Min)
	assert.Equal(t, float64(168), *def.Validation.Max)
}

// TestStudentActivationInterval guards the registry shape of the activate-
// students scheduler interval. Default 60 minutes, validation 5-1440.
func TestStudentActivationInterval(t *testing.T) {
	def := config.GetDefinition(config.KeyStudentActivationIntervalMin)
	require.NotNil(t, def, "operations.student_activation_interval_minutes should be registered")
	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 60, def.Default)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "config:update", def.WritePermission)
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
	assert.Equal(t, float64(5), *def.Validation.Min)
	assert.Equal(t, float64(1440), *def.Validation.Max)
	assert.Nil(t, def.DependsOn, "activate-students interval is independent of other settings")
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

// TestMFASettings_TypesAndDefaults locks down the field types, registry
// defaults, and select-options for the 4 MFA settings introduced in issue
// #1308. The trusted-device default is intentionally `true` so tenants on
// stock config opt into the cookie skip — flipping it would silently
// disable the feature for every existing school.
func TestMFASettings_TypesAndDefaults(t *testing.T) {
	t.Run("mfa_mode", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFAMode)
		require.NotNil(t, def)
		assert.Equal(t, config.FieldSelect, def.Type)
		assert.Equal(t, config.MFAModeOff, def.Default, "default must be off so existing tenants aren't surprise-locked into 2FA")
		assert.Equal(t, "security", def.Tab)
		assert.Equal(t, "mfa", def.Category)
		assert.Equal(t, "config:manage", def.WritePermission)
		require.NotNil(t, def.Options)
		require.Len(t, def.Options.Static, 3, "expected three options: off, required_admins, required_all")
	})

	t.Run("mfa_trusted_device_enabled", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFATrustedDeviceEnabled)
		require.NotNil(t, def)
		assert.Equal(t, config.FieldBoolean, def.Type)
		assert.Equal(t, true, def.Default, "default must be true — Yannick's #1308 review feedback explicitly asked for this")
		assert.Equal(t, "security", def.Tab)
		assert.Equal(t, "mfa", def.Category)
	})

	t.Run("mfa_trusted_device_days", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFATrustedDeviceDays)
		require.NotNil(t, def)
		assert.Equal(t, config.FieldNumber, def.Type)
		assert.Equal(t, 90, def.Default)
		require.NotNil(t, def.Validation)
		require.NotNil(t, def.Validation.Min)
		require.NotNil(t, def.Validation.Max)
		assert.Equal(t, float64(1), *def.Validation.Min)
		assert.Equal(t, float64(180), *def.Validation.Max)
	})

}

// TestMFASettings_DependsOnGraph locks the conditional-visibility rules so
// the settings UI hides irrelevant knobs when MFA is off or trusted devices
// are disabled.
func TestMFASettings_DependsOnGraph(t *testing.T) {
	t.Run("trusted_device_enabled hidden when mfa_mode is off", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFATrustedDeviceEnabled)
		require.NotNil(t, def)
		require.NotNil(t, def.DependsOn)
		assert.Equal(t, config.KeyMFAMode, def.DependsOn.Key)
		assert.Equal(t, "neq", def.DependsOn.Condition)
		assert.Equal(t, config.MFAModeOff, def.DependsOn.Value)
	})

	t.Run("trusted_device_days hidden when trusted_device_enabled is false", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFATrustedDeviceDays)
		require.NotNil(t, def)
		require.NotNil(t, def.DependsOn)
		assert.Equal(t, config.KeyMFATrustedDeviceEnabled, def.DependsOn.Key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	})

	t.Run("mfa_mode itself has no DependsOn", func(t *testing.T) {
		def := config.GetDefinition(config.KeyMFAMode)
		require.NotNil(t, def)
		assert.Nil(t, def.DependsOn, "mfa_mode is the root toggle and must not be hidden by another setting")
	})
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

func TestStatusFlagClearTime_Default(t *testing.T) {
	def := config.GetDefinition(config.KeyStatusFlagClearTime)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldTime, def.Type)
	assert.Equal(t, "18:00", def.Default, "status flag clear time should have a real default so end_of_day can run")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "abwesenheit", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
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
		{"operations.status_flag_clear_time", "18:00"},
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
		{"attendance.web_spontaneous_activities_enabled", false},
		{"operations.student_photos_enabled", false},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expectedDefault, def.Default, "setting %q default", tc.key)
	}
}

// TestStudentPhotosSetting guards the photo-feature toggle: defaults to OFF so
// no school surfaces photos until an admin opts in, sits in the operations tab
// alongside other Datenverwaltung-affecting toggles, and uses config:update
// (operational, not GDPR-scoped - consent itself is captured per student).
func TestStudentPhotosSetting(t *testing.T) {
	def := config.GetDefinition(config.KeyStudentPhotosEnabled)
	require.NotNil(t, def, "operations.student_photos_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "must default to false - feature is off until admin opts in")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "kinder", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
	assert.Nil(t, def.DependsOn, "stand-alone toggle, no DependsOn")
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
