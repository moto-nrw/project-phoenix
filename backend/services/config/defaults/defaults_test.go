package defaults_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/holidays"
	"github.com/moto-nrw/project-phoenix/models/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify that all settings are properly registered at init time.
// They run AFTER the defaults package init() functions execute (via blank import).

func TestAllSettingsRegistered(t *testing.T) {
	t.Parallel()

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
		// Default active-session inactivity timeout (issue #586, Rule 12).
		"operations.session_inactivity_timeout_minutes",
		"operations.operational_overview_scope",
		"operations.status_flag_clear_time",
		"operations.sick_clear_mode",
		"operations.excused_clear_mode",
		"operations.federal_state",
		"gdpr.data_cleanup_enabled",
		"gdpr.data_cleanup_time",
		"gdpr.data_cleanup_timeout_minutes",
		"gdpr.attendance_log_enabled",
		"gdpr.attendance_visible_days",
		"gdpr.room_detail_visible_days",
		// Per-child change-history retention (issue #1455).
		"gdpr.student_change_log_retention_days",
		// PWA standalone-usage retention (issue #2189).
		"gdpr.pwa_usage_retention_days",
		"feedback.enabled",
		"feedback.data_retention_days",
		"security.ogs_device_pin",
		"checkout.raumwechsel_enabled",
		"checkout.schulhof_enabled",
		"checkout.wc_enabled",
		"checkout.daily_checkout_from_all_rooms_enabled",
		// Capacity-detail disclosure toggles (issue #1879, devices tab).
		"checkin.activity_capacity_details_enabled",
		"checkin.room_capacity_details_enabled",
		// Device online/offline window for health monitoring (issue #586, Rule 12).
		"iot.device_online_window_minutes",
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
		// Pickup buckets for timetable-derived lists.
		"timetable.slot_list_short_day_cutoff",
		"timetable.slot_list_long_day_cutoff",
		// Presence-mode work package: tenant presence tracking model + who can check-in via web.
		"operations.presence_mode",
		"attendance.web_enabled",
		"attendance.nfc_enabled",
		"attendance.web_spontaneous_activities_enabled",
		"operations.group_mode",
		"operations.care_concept",
		"operations.require_pickup_offering_review",
		"operations.time_tracking_account_start_date",
		"operations.time_tracking_enforce_planned_start",
		// F9 deviation-reason gate (Planung redesign, Inkrement 6A).
		"operations.time_tracking_require_deviation_reason",
		"operations.time_tracking_deviation_tolerance_minutes",
		// Student photo feature (Datenverwaltung): per-school opt-in toggle.
		"operations.student_photos_enabled",
		// 2FA / MFA work package (issue #1308): mode toggle + trusted-device pair.
		"security.mfa_mode",
		"security.mfa_trusted_device_enabled",
		"security.mfa_trusted_device_days",
		// Account brute-force lockout policy (issue #586): threshold + duration.
		"security.account_lockout_threshold",
		"security.account_lockout_duration_minutes",
		// Privacy-consent visit-data retention default (issue #586, Rule 12).
		"gdpr.privacy_consent_retention_days",
		// Parent-enrollment PR 2: activate-students scheduler interval.
		"operations.student_activation_interval_minutes",
		// Parent-enrollment PR 3: guardian invitation token expiry.
		"invitations.guardian_token_expiry_hours",
		// Parent-enrollment registry plumbing. open_window_*,
		// show_status_reason_to_parent, care_overflow_mode, and
		// care_offerings_required moved to per-phase columns on
		// enrollment.phases - they're no longer tenant-wide settings.
		"enrollment.enabled",
		"enrollment.collect_grade_level",
		"enrollment.collect_school_class",
		"enrollment.care_offerings_enabled",
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
		// Parents-portal write features.
		"operations.parent_sick_note_enabled",
		"operations.parent_sick_requires_approval",
		"operations.parent_excused_requires_approval",
		"operations.parent_notes_enabled",
		"operations.parent_message_staff_name_visible",
		"operations.parent_pickup_change_enabled",
		"operations.parent_guardian_management_enabled",
		"operations.parent_master_data_edit_enabled",
		"operations.parent_master_data_request_enabled",
		"operations.parent_news_enabled",
		"operations.meal_plan_enabled",
		// Related-accounts management.
		"guardians.parent_invite_mode",
		"guardians.parent_can_remove",
		// Reminders work package (issue #1457): visual-only staff reminders.
		"reminders.pickup_upcoming_enabled",
		"reminders.pickup_upcoming_lead_minutes",
		"reminders.pickup_overdue_enabled",
		"reminders.activity_start_enabled",
		"reminders.activity_start_lead_minutes",
		"reminders.activity_overdue_enabled",
		// Guardian appointment reminders (issue #1671): the parent-facing half of
		// the Erinnerungen tab.
		"calendar.appointment_reminder_enabled",
		"calendar.appointment_reminder_lead_hours",
		// Info-point display feature (issue #1325): opt-in toggle, default off.
		"display.enabled",
		// Absence-approval email notifications (issue #1419 4d).
		"notifications.absence_approval_email",
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

func TestAbsenceApprovalEmailSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyNotificationsAbsenceApprovalEmail)
	require.NotNil(t, def, "notifications.absence_approval_email should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "absence emails must be opt-in")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "zeiterfassung", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
}

func TestPresenceModeSetting(t *testing.T) {
	t.Parallel()

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

func TestFederalStateSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyFederalState)
	require.NotNil(t, def, "operations.federal_state should be registered")
	assert.Equal(t, config.FieldSelect, def.Type)
	assert.Equal(t, holidays.DefaultRegion, def.Default, "default must stay DE-NW - existing schools are NRW")
	assert.Equal(t, config.AccessOperatorOnly, def.AccessPolicy, "federal_state is operator-only - changing it shifts the whole Arbeitszeitkonto")
	assert.Equal(t, "operations", def.Tab)

	// The select options and the holiday computation must know the same
	// regions, or a school could pick a state without a holiday calendar.
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 16)
	for _, opt := range def.Options.Static {
		value, ok := opt.Value.(string)
		require.True(t, ok, "federal_state option values must be strings")
		assert.True(t, holidays.ValidRegion(value), "federal_state option must have a holiday calendar")
	}
	assert.True(t, holidays.ValidRegion(holidays.DefaultRegion))
}

func TestDisplayEnabledSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyDisplayEnabled)
	require.NotNil(t, def, "display.enabled should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "info-point dashboard must be opt-in")
	assert.Equal(t, config.AccessShared, def.AccessPolicy, "tenant admins and operators can both enable it")
	assert.Equal(t, "config:update", def.WritePermission)
	assert.Equal(t, "operations", def.Tab)
}

func TestGuardianRelatedAccountsSettings(t *testing.T) {
	t.Parallel()

	inviteDef := config.GetDefinition(config.KeyGuardianParentInviteMode)
	require.NotNil(t, inviteDef, "guardians.parent_invite_mode should be registered")
	assert.Equal(t, config.FieldSelect, inviteDef.Type)
	assert.Equal(t, config.ParentInviteModeDisabled, inviteDef.Default, "default must be disabled (least privilege)")
	assert.Equal(t, "config:manage", inviteDef.WritePermission, "controls access to child data -> manage")
	assert.Equal(t, "operations", inviteDef.Tab)
	require.NotNil(t, inviteDef.Options)
	require.Len(t, inviteDef.Options.Static, 3)
	values := []any{
		inviteDef.Options.Static[0].Value,
		inviteDef.Options.Static[1].Value,
		inviteDef.Options.Static[2].Value,
	}
	assert.Contains(t, values, config.ParentInviteModeDisabled)
	assert.Contains(t, values, config.ParentInviteModeDirect)
	assert.Contains(t, values, config.ParentInviteModeStaffApproval)

	removeDef := config.GetDefinition(config.KeyGuardianParentCanRemove)
	require.NotNil(t, removeDef, "guardians.parent_can_remove should be registered")
	assert.Equal(t, config.FieldBoolean, removeDef.Type)
	assert.Equal(t, false, removeDef.Default, "default must be false (least privilege)")
	assert.Equal(t, "config:manage", removeDef.WritePermission)
	require.NotNil(t, removeDef.DependsOn, "parent_can_remove should be hidden when invites are disabled")
	assert.Equal(t, config.KeyGuardianParentInviteMode, removeDef.DependsOn.Key)
	assert.Equal(t, "neq", removeDef.DependsOn.Condition)
	assert.Equal(t, config.ParentInviteModeDisabled, removeDef.DependsOn.Value)
}

func TestParentMessageStaffNameVisibleSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyParentMessageStaffNameVisible)
	require.NotNil(t, def, "operations.parent_message_staff_name_visible should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	// Default ON: a messaging-active school attributes team replies to a person
	// by default. "Only from activation" is enforced per-message at send time
	// (staff_name_visible column), not by a restrictive default here.
	assert.Equal(t, true, def.Default, "staff-name visibility must default on")
	assert.Equal(t, "config:update", def.WritePermission)
	assert.Equal(t, "operations", def.Tab)
	// Hidden unless messaging itself is on — it only affects those messages.
	require.NotNil(t, def.DependsOn, "should be hidden when parent messaging is off")
	assert.Equal(t, config.KeyParentNotesEnabled, def.DependsOn.Key)
	assert.Equal(t, "eq", def.DependsOn.Condition)
	assert.Equal(t, true, def.DependsOn.Value)
}

// TestRemovedStudentGroupScopeSettings pins the #2329 deletion: per-child
// access no longer depends on a tenant scope setting, so re-registering one of
// these keys would silently reintroduce a policy the code no longer reads.
func TestRemovedStudentGroupScopeSettings(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"gdpr.student_data_scope",
		"gdpr.attendance_log_scope",
		"attendance.web_checkin_access",
	} {
		assert.Nilf(t, config.GetDefinition(key), "setting %q was removed in #2329 and must stay unregistered", key)
	}
}

func TestWebSpontaneousActivitiesSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyWebSpontaneousActivities)
	require.NotNil(t, def, "attendance.web_spontaneous_activities_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, true, def.Default, "web spontaneous activities must be opt-out")
	assert.Equal(t, config.AccessShared, def.AccessPolicy, "tenant admins and operators should both be able to manage this operational setting")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "anwesenheit", def.Category)
	assert.Equal(t, "config:manage", def.WritePermission)
	require.NotNil(t, def.DependsOn)
	assert.Equal(t, config.KeyCareConcept, def.DependsOn.Key)
	assert.Equal(t, "eq", def.DependsOn.Condition)
	assert.Equal(t, config.CareConceptOpenRooms, def.DependsOn.Value)
}

func TestAttendanceSetupSettings(t *testing.T) {
	t.Parallel()

	webDef := config.GetDefinition(config.KeyAttendanceWebEnabled)
	require.NotNil(t, webDef, "attendance.web_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, webDef.Type)
	assert.Equal(t, true, webDef.Default, "web attendance should default on")
	assert.Equal(t, config.AccessOperatorOnly, webDef.AccessPolicy, "web attendance is a provisioning flag, not a tenant-admin setting")
	assert.Equal(t, "operations", webDef.Tab)
	assert.Equal(t, "anwesenheit", webDef.Category)
	assert.Equal(t, "config:manage", webDef.WritePermission)

	nfcDef := config.GetDefinition(config.KeyAttendanceNFCEnabled)
	require.NotNil(t, nfcDef, "attendance.nfc_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, nfcDef.Type)
	assert.Equal(t, false, nfcDef.Default, "nfc attendance should default off")
	assert.Equal(t, config.AccessOperatorOnly, nfcDef.AccessPolicy, "nfc attendance is provisioned by operators after NFC setup")
	assert.Equal(t, "operations", nfcDef.Tab)
	assert.Equal(t, "anwesenheit", nfcDef.Category)
	assert.Equal(t, "config:manage", nfcDef.WritePermission)
}

// TestOperationalOverviewScopeSetting pins the one setting that decides who
// may see and operate every running module (#2380). The three option values
// are a wire contract with the backend gate, so a rename must fail here.
func TestOperationalOverviewScopeSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyOperationalOverviewScope)
	require.NotNil(t, def, "operations.operational_overview_scope should be registered")
	assert.Equal(t, config.FieldSelect, def.Type)
	assert.Equal(t, config.OverviewScopeOwn, def.Default, "the restrictive scope is the default")
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "aufsicht", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
	require.NotNil(t, def.Options)
	require.Len(t, def.Options.Static, 3)
	values := make([]any, 0, 3)
	for _, opt := range def.Options.Static {
		values = append(values, opt.Value)
	}
	assert.Contains(t, values, config.OverviewScopeOwn)
	assert.Contains(t, values, config.OverviewScopeAdmins)
	assert.Contains(t, values, config.OverviewScopeAllStaff)

	// The retired flag must not come back: two settings answering the same
	// question is exactly what #2380 removed.
	assert.Nil(t, config.GetDefinition("operations.admin_supervision_overview"))
}

func TestOrganizationSetupSettings(t *testing.T) {
	t.Parallel()

	groupDef := config.GetDefinition(config.KeyGroupMode)
	require.NotNil(t, groupDef, "operations.group_mode should be registered")
	assert.Equal(t, config.FieldSelect, groupDef.Type)
	assert.Equal(t, config.GroupModeFixedGroups, groupDef.Default)
	assert.Equal(t, config.AccessShared, groupDef.AccessPolicy)
	assert.Equal(t, "operations", groupDef.Tab)
	assert.Equal(t, "organisation", groupDef.Category)
	assert.Equal(t, "config:update", groupDef.WritePermission)
	require.NotNil(t, groupDef.Options)
	groupValues := []any{groupDef.Options.Static[0].Value, groupDef.Options.Static[1].Value}
	assert.Contains(t, groupValues, config.GroupModeFixedGroups)
	assert.Contains(t, groupValues, config.GroupModeOpenCare)

	careDef := config.GetDefinition(config.KeyCareConcept)
	require.NotNil(t, careDef, "operations.care_concept should be registered")
	assert.Equal(t, config.FieldSelect, careDef.Type)
	assert.Equal(t, config.CareConceptOpenRooms, careDef.Default)
	assert.Equal(t, config.AccessShared, careDef.AccessPolicy)
	assert.Equal(t, "operations", careDef.Tab)
	assert.Equal(t, "organisation", careDef.Category)
	assert.Equal(t, "config:update", careDef.WritePermission)
	require.NotNil(t, careDef.Options)
	careValues := []any{careDef.Options.Static[0].Value, careDef.Options.Static[1].Value}
	assert.Contains(t, careValues, config.CareConceptFixedSchedule)
	assert.Contains(t, careValues, config.CareConceptOpenRooms)
}

func TestTimeTrackingAccountStartDateSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyTimeTrackingAccountStartDate)
	require.NotNil(t, def, "operations.time_tracking_account_start_date should be registered")
	assert.Equal(t, config.FieldDate, def.Type)
	assert.Equal(t, "", def.Default)
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "zeiterfassung", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
}

func TestTimeTrackingEnforcePlannedStartSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyTimeTrackingEnforcePlannedStart)
	require.NotNil(t, def, "operations.time_tracking_enforce_planned_start should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default, "planned-start enforcement must be opt-in")
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "zeiterfassung", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
}

func TestTimeTrackingRequireDeviationReasonSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyTimeTrackingRequireDeviationReason)
	require.NotNil(t, def, "operations.time_tracking_require_deviation_reason should be registered")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, true, def.Default, "deviation-reason gate is on by default (#1844); schools can opt out per tenant")
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "zeiterfassung", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
	assert.Nil(t, def.DependsOn)
}

func TestTimeTrackingDeviationToleranceSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyTimeTrackingDeviationToleranceMinutes)
	require.NotNil(t, def, "operations.time_tracking_deviation_tolerance_minutes should be registered")
	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 15, def.Default, "default mirrors the fixed 15-minute frontend tolerance of getDeltaStatus")
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "zeiterfassung", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	assert.Equal(t, float64(0), *def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
	assert.Equal(t, float64(120), *def.Validation.Max)
	require.NotNil(t, def.DependsOn, "tolerance is only visible while the gate is active")
	assert.Equal(t, config.KeyTimeTrackingRequireDeviationReason, def.DependsOn.Key)
	assert.Equal(t, "eq", def.DependsOn.Condition)
	assert.Equal(t, true, def.DependsOn.Value)
}

func TestTimetableSettings_Types(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	enabledDef := config.GetDefinition("timetable.enabled")
	require.NotNil(t, enabledDef)
	assert.Equal(t, true, enabledDef.Default, "timetable must default to true so the feature is opt-out")

	// The top-level feature is opt-out, and so is materialization: instances
	// are prepared automatically unless an operator disables it per tenant.
	// Auto-start stays opt-in so live activity transitions do not begin
	// unless a tenant explicitly enables that behaviour.
	matDef := config.GetDefinition("timetable.materialization_enabled")
	require.NotNil(t, matDef)
	assert.Equal(t, true, matDef.Default, "materialization must default to true so it is opt-out")

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
	t.Parallel()

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
	t.Parallel()

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

func TestTimetableMaterializationSettings_OperatorOnly(t *testing.T) {
	t.Parallel()

	keys := []string{
		"timetable.materialization_enabled",
		"timetable.materialization_weekday",
		"timetable.materialization_weeks_ahead",
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equalf(t, config.AccessOperatorOnly, def.AccessPolicy,
			"setting %q is operator-only - materialization cadence is platform plumbing, not tenant-tunable", key)
	}
}

func TestTimetableSettings_WeekdayOptions(t *testing.T) {
	t.Parallel()

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

func TestTimetableChildrenPerStaffRatioSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition("timetable.children_per_staff_ratio")
	require.NotNil(t, def, "setting timetable.children_per_staff_ratio should exist")

	assert.Equal(t, config.FieldNumber, def.Type)
	assert.Equal(t, 12, def.Default, "children-per-staff ratio must default to 12")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "stundenplan", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)

	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.Min)
	require.NotNil(t, def.Validation.Max)
	assert.Equal(t, float64(1), *def.Validation.Min)
	assert.Equal(t, float64(30), *def.Validation.Max)

	require.NotNil(t, def.DependsOn, "must be gated behind the top-level timetable toggle")
	assert.Equal(t, "timetable.enabled", def.DependsOn.Key)
	assert.Equal(t, "eq", def.DependsOn.Condition)
	assert.Equal(t, true, def.DependsOn.Value)
}

func TestOperationsSettings_Types(t *testing.T) {
	t.Parallel()

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
		{"operations.operational_overview_scope", config.FieldSelect},
		{"operations.status_flag_clear_time", config.FieldTime},
		{"operations.sick_clear_mode", config.FieldSelect},
		{"operations.excused_clear_mode", config.FieldSelect},
		{"operations.parent_sick_note_enabled", config.FieldBoolean},
		{"operations.parent_sick_requires_approval", config.FieldBoolean},
		{"operations.parent_excused_requires_approval", config.FieldBoolean},
		{"operations.parent_notes_enabled", config.FieldBoolean},
		{"operations.parent_pickup_change_enabled", config.FieldBoolean},
		{"operations.parent_guardian_management_enabled", config.FieldBoolean},
		{"timetable.slot_list_short_day_cutoff", config.FieldTime},
		{"timetable.slot_list_long_day_cutoff", config.FieldTime},
	}

	for _, tc := range tests {
		def := config.GetDefinition(tc.key)
		require.NotNilf(t, def, "setting %q should exist", tc.key)
		assert.Equalf(t, tc.expected, def.Type, "setting %q should be type %s", tc.key, tc.expected)
	}
}

// TestParentPortalSettings_DefaultOn guards the opt-out contract: both
// parents-portal write toggles must default to true so schools with the
// portal get them without extra configuration.
func TestParentPortalSettings_DefaultOn(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"operations.parent_sick_note_enabled",
		"operations.parent_notes_enabled",
	} {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equalf(t, config.FieldBoolean, def.Type, "setting %q should be boolean", key)
		assert.Equalf(t, true, def.Default, "setting %q should default to true (opt-out)", key)
		assert.Equalf(t, "operations", def.Tab, "setting %q should be on the operations tab", key)
	}
}

// TestParentAbsenceApprovalSettings guards the opt-out approval contract for
// parent-submitted sick and excused absences (#2447/#2449). Both default ON and
// remain configurable independently while the parent absence feature is on.
func TestParentAbsenceApprovalSettings(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"operations.parent_sick_requires_approval",
		"operations.parent_excused_requires_approval",
	} {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "%s should exist", key)
		assert.Equal(t, config.FieldBoolean, def.Type, "approval toggle should be boolean")
		assert.Equal(t, true, def.Default, "approval toggle must default ON (opt-out)")
		assert.Equal(t, "operations", def.Tab, "approval toggle should be on the operations tab")
		assert.Equal(t, "config:manage", def.WritePermission, "changes the absence-approval workflow -> manage")
		require.NotNil(t, def.DependsOn, "should be hidden when the sick-note feature is off")
		assert.Equal(t, config.KeyParentSickNoteEnabled, def.DependsOn.Key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}
}

// TestParentPickupChangeSetting_DefaultOn guards that the pickup-change toggle
// defaults to true, in line with the other parents-portal write toggles
// (sick-note/notes). Schools that do not want parents to alter the care
// schedule must switch it off deliberately.
func TestParentPickupChangeSetting_DefaultOn(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition("operations.parent_pickup_change_enabled")
	require.NotNil(t, def, "operations.parent_pickup_change_enabled should exist")
	assert.Equal(t, config.FieldBoolean, def.Type, "pickup-change toggle should be boolean")
	assert.Equal(t, true, def.Default, "pickup-change toggle should default to true")
	assert.Equal(t, "operations", def.Tab, "pickup-change toggle should be on the operations tab")
}

func TestPickupOfferingReviewSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyRequirePickupOfferingReview)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, false, def.Default)
	assert.Equal(t, config.AccessShared, def.AccessPolicy)
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "betreuungszeiten", def.Category)
	assert.Equal(t, "config:read", def.ReadPermission)
	assert.Equal(t, "config:update", def.WritePermission)
	assert.Equal(t, "Angebotsabgleich für dauerhafte Gehzeiten", def.Label)
	assert.Equal(t, "Bei einer Abweichung wählen Sie ein anderes Angebot oder eine Ausnahme.", def.Description)
}

func TestParentPermanentCareRequestSettings_DefaultOnAndIndependent(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		config.KeyParentCarePickupRequestEnabled,
		config.KeyParentCareModeRequestEnabled,
	} {
		def := config.GetDefinition(key)
		require.NotNil(t, def, "%s should exist", key)
		assert.Equal(t, config.FieldBoolean, def.Type)
		assert.Equal(t, true, def.Default)
		assert.Equal(t, "operations", def.Tab)
		assert.Nil(t, def.DependsOn, "%s must not depend on messaging", key)
	}
}

// TestGuardianManagementSetting guards the guardian contact/pickup management
// toggle. It defaults ON like the other parents-portal write features - the
// safety-critical part (pickup authority) is constrained structurally
// (helpers-only + audit), not by this toggle - but keeps config:manage because
// enabling it can expose pickup-authority changes.
func TestGuardianManagementSetting(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition("operations.parent_guardian_management_enabled")
	require.NotNil(t, def, "operations.parent_guardian_management_enabled should exist")
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, true, def.Default, "defaults ON; pickup-flag safety is structural, not toggle-based")
	assert.Equal(t, "config:manage", def.WritePermission, "can expose pickup-authority changes -> manage")
	assert.Equal(t, "operations", def.Tab)
}

// TestEnrollmentSettings_AllRegistered_OnEnrollmentTab guards that every
// enrollment.* user-facing setting lands on the "enrollment" tab. The two
// system-tab keys (outbox_max_attempts, status_token_ttl_days) are pulled
// out separately so a future tab refactor can't silently move user-visible
// settings without breaking the test.
func TestEnrollmentSettings_AllRegistered_OnEnrollmentTab(t *testing.T) {
	t.Parallel()

	enrollmentTabKeys := []string{
		config.KeyEnrollmentEnabled,
		config.KeyEnrollmentCollectGradeLevel,
		config.KeyEnrollmentCollectSchoolClass,
		config.KeyEnrollmentCareOfferingsEnabled,
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

// TestEnrollmentLegalTexts guards the per-tenant legal document settings:
// all text fields are textarea-typed, default to empty, use the stricter
// config:manage write permission, and stay visible whenever enrollment is
// enabled so admins can enter text before activating a block.
func TestEnrollmentLegalTexts(t *testing.T) {
	t.Parallel()

	keys := []string{
		config.KeyEnrollmentLegalAGBText,
		config.KeyEnrollmentLegalDSGVOText,
		config.KeyEnrollmentLegalEmailContactText,
		config.KeyEnrollmentLegalPhotoText,
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.Equalf(t, config.FieldTextarea, def.Type, "setting %q should be textarea", key)
		assert.Equalf(t, "", def.Default, "setting %q should default to empty", key)
		assert.Equalf(t, "enrollment", def.Tab, "setting %q should be in enrollment tab", key)
		assert.Equalf(t, "rechtstexte", def.Category, "setting %q should be in rechtstexte category", key)
		assert.Equalf(t, "config:manage", def.WritePermission, "setting %q should use config:manage (legal/GDPR)", key)
		assert.NotEmptyf(t, def.Label, "setting %q must have a German label", key)
		assert.NotEmptyf(t, def.Description, "setting %q must have a German description", key)
		require.NotNilf(t, def.DependsOn, "setting %q must declare a DependsOn parent", key)
		assert.Equalf(t, config.KeyEnrollmentEnabled, def.DependsOn.Key, "setting %q has unexpected DependsOn parent", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	documentDef := config.GetDefinition(config.KeyEnrollmentLegalAGBDocumentURL)
	require.NotNil(t, documentDef, "AGB document URL setting should be registered")
	assert.Equal(t, config.FieldText, documentDef.Type)
	assert.Equal(t, "", documentDef.Default)
	assert.Equal(t, "enrollment", documentDef.Tab)
	assert.Equal(t, "rechtstexte", documentDef.Category)
	assert.Equal(t, "config:manage", documentDef.WritePermission)
	require.NotNil(t, documentDef.DependsOn)
	assert.Equal(t, config.KeyEnrollmentEnabled, documentDef.DependsOn.Key)

	modeDef := config.GetDefinition(config.KeyEnrollmentLegalAGBDisplayMode)
	require.NotNil(t, modeDef, "AGB display mode setting should be registered")
	assert.Equal(t, config.FieldSelect, modeDef.Type)
	assert.Equal(t, config.EnrollmentLegalAGBDisplayModeText, modeDef.Default)
	assert.Equal(t, "enrollment", modeDef.Tab)
	assert.Equal(t, "rechtstexte", modeDef.Category)
	assert.Equal(t, "config:manage", modeDef.WritePermission)
	require.NotNil(t, modeDef.DependsOn)
	assert.Equal(t, config.KeyEnrollmentEnabled, modeDef.DependsOn.Key)
	require.NotNil(t, modeDef.Options)
	require.Len(t, modeDef.Options.Static, 2)
	values := []any{modeDef.Options.Static[0].Value, modeDef.Options.Static[1].Value}
	assert.Contains(t, values, config.EnrollmentLegalAGBDisplayModeText)
	assert.Contains(t, values, config.EnrollmentLegalAGBDisplayModePDF)
}

func TestEnrollmentLegalBlockToggles(t *testing.T) {
	t.Parallel()

	keys := []string{
		config.KeyEnrollmentLegalTermsEnabled,
		config.KeyEnrollmentLegalDSGVOEnabled,
		config.KeyEnrollmentLegalEmailContactEnabled,
		config.KeyEnrollmentLegalPhotoEnabled,
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.Equalf(t, config.FieldBoolean, def.Type, "setting %q should be boolean", key)
		assert.Equalf(t, false, def.Default, "setting %q should default off", key)
		assert.Equalf(t, "enrollment", def.Tab, "setting %q should be in enrollment tab", key)
		assert.Equalf(t, "rechtstexte", def.Category, "setting %q should be in rechtstexte category", key)
		assert.Equalf(t, "config:manage", def.WritePermission, "setting %q should use config:manage", key)
		assert.NotEmptyf(t, def.Label, "setting %q must have a German label", key)
		assert.NotEmptyf(t, def.Description, "setting %q must have a German description", key)
		require.NotNilf(t, def.DependsOn, "setting %q must declare a DependsOn parent", key)
		assert.Equalf(t, config.KeyEnrollmentEnabled, def.DependsOn.Key, "setting %q has unexpected DependsOn parent", key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}
}

// TestEnrollmentEnabled_DefaultsOff guards that the master feature flag is
// off by default. Regressing this would silently expose a half-built
// feature to every tenant on next deploy.
func TestEnrollmentEnabled_DefaultsOff(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
}

// TestEnrollmentSelectOptions_AreCanonical guards the static option lists
// for the three select-typed enrollment settings. Drift here breaks the
// frontend select renderer silently.
func TestEnrollmentSelectOptions_AreCanonical(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	def := config.GetDefinition("security.ogs_device_pin")
	require.NotNil(t, def)
	assert.Equal(t, config.FieldPassword, def.Type)
	assert.Equal(t, "devices", def.Tab)
	assert.Equal(t, "pin", def.Category)
	assert.Equal(t, "config:manage", def.WritePermission)
	require.NotNil(t, def.Validation)
	assert.False(t, def.Validation.AllowEmpty)
	require.NotNil(t, def.DependsOn)
	assert.Equal(t, config.KeyAttendanceNFCEnabled, def.DependsOn.Key)
	assert.Equal(t, "eq", def.DependsOn.Condition)
	assert.Equal(t, true, def.DependsOn.Value)
}

// TestMFASettings_TypesAndDefaults locks down the field types, registry
// defaults, and select-options for the 4 MFA settings introduced in issue
// #1308. The trusted-device default is intentionally `true` so tenants on
// stock config opt into the cookie skip — flipping it would silently
// disable the feature for every existing school.
func TestMFASettings_TypesAndDefaults(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	dailyCheckoutDef := config.GetDefinition("operations.student_daily_checkout_time")
	require.NotNil(t, dailyCheckoutDef)
	require.NotNil(t, dailyCheckoutDef.DependsOn)
	assert.Equal(t, config.KeyAttendanceNFCEnabled, dailyCheckoutDef.DependsOn.Key)
	assert.Equal(t, "eq", dailyCheckoutDef.DependsOn.Condition)
	assert.Equal(t, true, dailyCheckoutDef.DependsOn.Value)

	toggleDef := config.GetDefinition("operations.per_student_checkout_enabled")
	require.NotNil(t, toggleDef)
	require.NotNil(t, toggleDef.DependsOn)
	assert.Equal(t, config.KeyAttendanceNFCEnabled, toggleDef.DependsOn.Key)
	assert.Equal(t, "eq", toggleDef.DependsOn.Condition)
	assert.Equal(t, true, toggleDef.DependsOn.Value)

	deltaDef := config.GetDefinition("operations.per_student_checkout_delta_minutes")
	require.NotNil(t, deltaDef)
	require.NotNil(t, deltaDef.DependsOn)
	assert.Equal(t, "operations.per_student_checkout_enabled", deltaDef.DependsOn.Key)
	assert.Equal(t, "eq", deltaDef.DependsOn.Condition)
	assert.Equal(t, true, deltaDef.DependsOn.Value)
}

func TestDependsOn_AttendanceLogGroup(t *testing.T) {
	t.Parallel()

	dependentKeys := []string{
		"gdpr.attendance_visible_days",
		"gdpr.room_detail_visible_days",
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
	t.Parallel()

	retentionDef := config.GetDefinition("feedback.data_retention_days")
	require.NotNil(t, retentionDef)
	require.NotNil(t, retentionDef.DependsOn)
	assert.Equal(t, "feedback.enabled", retentionDef.DependsOn.Key)
	assert.Equal(t, "eq", retentionDef.DependsOn.Condition)
	assert.Equal(t, true, retentionDef.DependsOn.Value)
}

func TestFeedbackSettings(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition("feedback.enabled")
	require.NotNil(t, def)
	assert.Equal(t, config.FieldBoolean, def.Type)
	assert.Equal(t, "gdpr", def.Tab)
	assert.Equal(t, "feedback", def.Category)
	assert.Equal(t, false, def.Default, "feedback should default to false (opt-in)")
	assert.Equal(t, "config:manage", def.WritePermission)
}

func TestDevicesSettings(t *testing.T) {
	t.Parallel()

	keys := []string{
		"checkout.raumwechsel_enabled",
		"checkout.schulhof_enabled",
		"checkout.wc_enabled",
		"checkout.daily_checkout_from_all_rooms_enabled",
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equal(t, config.FieldBoolean, def.Type, "setting %q should be boolean", key)
		assert.Equal(t, "devices", def.Tab, "setting %q should be in devices tab", key)
		assert.Equal(t, "checkout", def.Category, "setting %q should be in checkout category", key)
		assert.Equal(t, "config:update", def.WritePermission, "setting %q should use config:update", key)
		require.NotNil(t, def.DependsOn, "setting %q should be gated by nfc_enabled", key)
		assert.Equal(t, config.KeyAttendanceNFCEnabled, def.DependsOn.Key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	// Raumwechsel defaults to true (no system room required)
	raumwechsel := config.GetDefinition("checkout.raumwechsel_enabled")
	assert.Equal(t, true, raumwechsel.Default, "raumwechsel should default to true")

	// Schulhof and WC default to false (opt-in, system rooms created on activation)
	schulhof := config.GetDefinition("checkout.schulhof_enabled")
	assert.Equal(t, false, schulhof.Default, "schulhof should default to false (opt-in)")
	wc := config.GetDefinition("checkout.wc_enabled")
	assert.Equal(t, false, wc.Default, "wc should default to false (opt-in)")

	allRooms := config.GetDefinition("checkout.daily_checkout_from_all_rooms_enabled")
	assert.Equal(t, true, allRooms.Default, "new tenants should offer daily checkout from every room")
}

func TestCheckinCapacityDetailSettings(t *testing.T) {
	t.Parallel()

	keys := []string{
		config.KeyCheckinActivityCapacityDetailsEnabled,
		config.KeyCheckinRoomCapacityDetailsEnabled,
	}
	for _, key := range keys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should exist", key)
		assert.Equal(t, config.FieldBoolean, def.Type, "setting %q should be boolean", key)
		assert.Equal(t, "devices", def.Tab, "setting %q should be in devices tab", key)
		assert.Equal(t, "kapazität", def.Category, "setting %q should be in kapazität category", key)
		assert.Equal(t, "config:update", def.WritePermission, "setting %q should use config:update", key)
		require.NotNil(t, def.DependsOn, "setting %q should be gated by nfc_enabled", key)
		assert.Equal(t, config.KeyAttendanceNFCEnabled, def.DependsOn.Key)
		assert.Equal(t, "eq", def.DependsOn.Condition)
		assert.Equal(t, true, def.DependsOn.Value)
	}

	// Activity defaults to false: the rich kiosk message was never visible
	// before issue #1879, so it stays opt-in.
	activity := config.GetDefinition(config.KeyCheckinActivityCapacityDetailsEnabled)
	assert.Equal(t, false, activity.Default, "activity capacity details should default to false (opt-in)")

	// Room defaults to true: schools already see the rich room message today.
	room := config.GetDefinition(config.KeyCheckinRoomCapacityDetailsEnabled)
	assert.Equal(t, true, room.Default, "room capacity details should default to true (preserves existing behavior)")
}

func TestStudentDailyCheckoutTime_OptionalDefault(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition("operations.student_daily_checkout_time")
	require.NotNil(t, def)
	assert.Equal(t, "", def.Default, "daily checkout time should default to empty (always available)")
}

func TestStatusFlagClearTime_Default(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyStatusFlagClearTime)
	require.NotNil(t, def)
	assert.Equal(t, config.FieldTime, def.Type)
	assert.Equal(t, "18:00", def.Default, "status flag clear time should have a real default so end_of_day can run")
	assert.Equal(t, "operations", def.Tab)
	assert.Equal(t, "abwesenheit", def.Category)
	assert.Equal(t, "config:update", def.WritePermission)
}

func TestValidation_NumberFields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
		{"operations.operational_overview_scope", config.OverviewScopeOwn},
		{"operations.status_flag_clear_time", "18:00"},
		{"gdpr.data_cleanup_enabled", true},
		{"gdpr.data_cleanup_time", "02:00"},
		{"gdpr.data_cleanup_timeout_minutes", 30},
		{"gdpr.attendance_log_enabled", false},
		{"gdpr.attendance_visible_days", 30},
		{"gdpr.room_detail_visible_days", 7},
		{"feedback.enabled", false},
		{"feedback.data_retention_days", 90},
		{"security.ogs_device_pin", "1234"},
		{"checkout.raumwechsel_enabled", true},
		{"checkout.schulhof_enabled", false},
		{"checkout.wc_enabled", false},
		{"checkin.activity_capacity_details_enabled", false},
		{"checkin.room_capacity_details_enabled", true},
		{"tracking.indicators_enabled", false},
		{"tracking.indicator_1", ""},
		{"tracking.indicator_2", ""},
		{"tracking.indicator_3", ""},
		{"tracking.auto_checkout_enabled", false},
		{"tracking.auto_checkout_grace_minutes", 15},
		{"attendance.web_spontaneous_activities_enabled", true},
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestAutoCheckoutSettings guards the #1798 auto-checkout pair: opt-in toggle
// (must default to OFF so no school auto-closes sessions without consent) and
// a grace-minutes number gated behind the toggle.
func TestAutoCheckoutSettings(t *testing.T) {
	t.Parallel()

	enabled := config.GetDefinition(config.KeyTrackingAutoCheckoutEnabled)
	require.NotNil(t, enabled, "tracking.auto_checkout_enabled should be registered")
	assert.Equal(t, config.FieldBoolean, enabled.Type)
	assert.Equal(t, false, enabled.Default, "must default to false - pure opt-in")
	assert.Equal(t, "operations", enabled.Tab)
	assert.Equal(t, "zeiterfassung", enabled.Category)
	assert.Equal(t, "config:update", enabled.WritePermission)
	assert.Nil(t, enabled.DependsOn)

	grace := config.GetDefinition(config.KeyTrackingAutoCheckoutGraceMinutes)
	require.NotNil(t, grace, "tracking.auto_checkout_grace_minutes should be registered")
	assert.Equal(t, config.FieldNumber, grace.Type)
	assert.Equal(t, 15, grace.Default)
	require.NotNil(t, grace.DependsOn, "grace minutes should be gated behind the toggle")
	assert.Equal(t, config.KeyTrackingAutoCheckoutEnabled, grace.DependsOn.Key)
	assert.Equal(t, "eq", grace.DependsOn.Condition)
	assert.Equal(t, true, grace.DependsOn.Value)
	require.NotNil(t, grace.Validation)
	require.NotNil(t, grace.Validation.Min)
	require.NotNil(t, grace.Validation.Max)
	assert.Equal(t, float64(0), *grace.Validation.Min)
	assert.Equal(t, float64(240), *grace.Validation.Max)
}

// TestPayrollSettings pins the DATEV payroll foundation (#1417 Tranche 2b).
// The empty defaults are the point: Lohnartnummern are mandant-specific and
// a plausible preset would silently bill wrong. Any change that introduces a
// non-empty default here must be treated as a billing bug, not a convenience.
func TestPayrollSettings(t *testing.T) {
	t.Parallel()

	lohnarten := []string{
		config.KeyPayrollLohnartRegelarbeit,
		config.KeyPayrollLohnartPlusStunden,
		config.KeyPayrollLohnartAuszahlung,
		config.KeyPayrollLohnartFreizeitausgleich,
		config.KeyPayrollLohnartKrank,
		config.KeyPayrollLohnartUrlaub,
		config.KeyPayrollLohnartFortbildung,
	}
	for _, key := range lohnarten {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "%s should be registered", key)
		assert.Equal(t, config.FieldText, def.Type, key)
		assert.Equal(t, "", def.Default, "%s must default to EMPTY — never invent Lohnart numbers", key)
		assert.Equal(t, "abrechnung", def.Tab, key)
		assert.Equal(t, "lohnarten", def.Category, key)
		assert.Equal(t, "config:manage", def.WritePermission, key)
		assert.Equal(t, config.AccessAdminOnly, def.AccessPolicy, key)
		require.NotNil(t, def.Validation, key)
		require.NotNil(t, def.Validation.Pattern, key)
		assert.True(t, def.Validation.AllowEmpty, key)
		assert.Equal(t, `^\d{1,4}$`, *def.Validation.Pattern, key)
	}

	units := []string{
		config.KeyPayrollEinheitKrank,
		config.KeyPayrollEinheitUrlaub,
		config.KeyPayrollEinheitFortbildung,
	}
	for _, key := range units {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "%s should be registered", key)
		assert.Equal(t, config.FieldSelect, def.Type, key)
		assert.Equal(t, "", def.Default, "%s must default to 'not set' — the unit comes from the Träger's Lohnart definition", key)
		assert.Equal(t, "abrechnung", def.Tab, key)
		assert.Equal(t, "config:manage", def.WritePermission, key)
		require.NotNil(t, def.Options, key)
		require.Len(t, def.Options.Static, 3, key)
	}

	berater := config.GetDefinition(config.KeyPayrollDatevBeraternummer)
	require.NotNil(t, berater)
	assert.Equal(t, config.FieldText, berater.Type)
	assert.Equal(t, "", berater.Default)
	assert.Equal(t, "config:manage", berater.WritePermission)
	require.NotNil(t, berater.Validation)
	require.NotNil(t, berater.Validation.Pattern)
	assert.True(t, berater.Validation.AllowEmpty)
	assert.Equal(t, `^\d{1,7}$`, *berater.Validation.Pattern)

	mandant := config.GetDefinition(config.KeyPayrollDatevMandantennummer)
	require.NotNil(t, mandant)
	assert.Equal(t, config.FieldText, mandant.Type)
	assert.Equal(t, "", mandant.Default)
	assert.Equal(t, "config:manage", mandant.WritePermission)
	require.NotNil(t, mandant.Validation)
	require.NotNil(t, mandant.Validation.Pattern)
	assert.True(t, mandant.Validation.AllowEmpty)
	assert.Equal(t, `^\d{1,5}$`, *mandant.Validation.Pattern)
}
