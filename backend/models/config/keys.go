package config

// Settings key constants. Use these instead of string literals to ensure
// compile-time safety when referencing settings keys across the codebase.

// Security settings.
const (
	KeyOGSDevicePIN = "security.ogs_device_pin"

	// MFA settings (issue #1308 — Phase 3).
	KeyMFAMode                 = "security.mfa_mode"
	KeyMFATrustedDeviceEnabled = "security.mfa_trusted_device_enabled"
	KeyMFATrustedDeviceDays    = "security.mfa_trusted_device_days"
)

// MFAMode option values for KeyMFAMode.
const (
	MFAModeOff            = "off"
	MFAModeRequiredAdmins = "required_admins"
	MFAModeRequiredAll    = "required_all"
)

// GDPR / data cleanup settings.
const (
	KeyDataCleanupEnabled        = "gdpr.data_cleanup_enabled"
	KeyDataCleanupTime           = "gdpr.data_cleanup_time"
	KeyDataCleanupTimeoutMinutes = "gdpr.data_cleanup_timeout_minutes"
)

// Attendance log / Raumverlauf (student attendance history) settings.
const (
	KeyAttendanceLogEnabled  = "gdpr.attendance_log_enabled"
	KeyAttendanceVisibleDays = "gdpr.attendance_visible_days"
	KeyRoomDetailVisibleDays = "gdpr.room_detail_visible_days"
	KeyAttendanceLogScope    = "gdpr.attendance_log_scope"
)

// AttendanceLogScope option values for KeyAttendanceLogScope.
const (
	AttendanceLogScopeGroupSupervisorsOnly = "group_supervisors_only"
	AttendanceLogScopeAllStaff             = "all_staff"
)

// Student data scope / who can read full student profile data.
const (
	KeyStudentDataScope = "gdpr.student_data_scope"
)

// StudentDataScope option values for KeyStudentDataScope.
const (
	StudentDataScopeGroupSupervisorsOnly = "group_supervisors_only"
	StudentDataScopeAllStaff             = "all_staff"
)

// Feedback settings.
const (
	KeyFeedbackEnabled           = "feedback.enabled"
	KeyFeedbackDataRetentionDays = "feedback.data_retention_days"
)

// Checkout button settings (devices tab).
const (
	KeyCheckoutRaumwechselEnabled = "checkout.raumwechsel_enabled"
	KeyCheckoutSchulhofEnabled    = "checkout.schulhof_enabled"
	KeyCheckoutWCEnabled          = "checkout.wc_enabled"
)

// Tracking indicator settings (student card activity indicators).
const (
	KeyTrackingIndicatorsEnabled = "tracking.indicators_enabled"
	KeyTrackingIndicator1        = "tracking.indicator_1"
	KeyTrackingIndicator2        = "tracking.indicator_2"
	KeyTrackingIndicator3        = "tracking.indicator_3"
)

// Operations settings.
const (
	KeySessionEndEnabled              = "operations.session_end_enabled"
	KeySessionEndTime                 = "operations.session_end_time"
	KeySessionEndTimeoutMinutes       = "operations.session_end_timeout_minutes"
	KeyStudentDailyCheckoutTime       = "operations.student_daily_checkout_time"
	KeyPerStudentCheckoutEnabled      = "operations.per_student_checkout_enabled"
	KeyPerStudentCheckoutDeltaMinutes = "operations.per_student_checkout_delta_minutes"
	KeySessionCleanupEnabled          = "operations.session_cleanup_enabled"
	KeySessionCleanupIntervalMinutes  = "operations.session_cleanup_interval_minutes"
	KeySessionAbandonedThresholdMin   = "operations.session_abandoned_threshold_minutes"
	KeyAdminSupervisionOverview       = "operations.admin_supervision_overview"
	KeyStatusFlagClearTime            = "operations.status_flag_clear_time"
	KeySickClearMode                  = "operations.sick_clear_mode"
	KeyExcusedClearMode               = "operations.excused_clear_mode"
	KeyPresenceMode                   = "operations.presence_mode"
	KeyWebCheckinAccess               = "attendance.web_checkin_access"
	KeyWebSpontaneousActivities       = "attendance.web_spontaneous_activities_enabled"
	KeyStudentPhotosEnabled           = "operations.student_photos_enabled"
)

// PresenceMode option values for KeyPresenceMode.
const (
	PresenceModeDetailed = "detailed"
	PresenceModeBinary   = "binary"
)

// WebCheckinAccess option values for KeyWebCheckinAccess.
const (
	WebCheckinAccessGroupSupervisors = "group_supervisors"
	WebCheckinAccessAllStaff         = "all_staff"
)

// StatusFlagClearMode option values for KeySickClearMode and KeyExcusedClearMode.
const (
	ClearModeManual      = "manual"
	ClearModeNextCheckin = "next_checkin"
	ClearModeEndOfDay    = "end_of_day"
)

// Timetable settings (WP-B7). Per-tenant configuration for the activity
// template → instance materialization pipeline and the staff-facing
// auto-start behaviour. All definitions live in defaults/timetable.go.
const (
	KeyTimetableEnabled                   = "timetable.enabled"
	KeyTimetableMaterializationEnabled    = "timetable.materialization_enabled"
	KeyTimetableMaterializationWeekday    = "timetable.materialization_weekday"
	KeyTimetableMaterializationWeeksAhead = "timetable.materialization_weeks_ahead"
	KeyTimetableAutoStartPlanned          = "timetable.auto_start_planned"
	KeyTimetableOverdueThresholdMinutes   = "timetable.overdue_threshold_minutes"
	KeyTimetableShowExpectedChildrenCount = "timetable.show_expected_children_count"
	KeyGDPRTimetableRetentionDays         = "gdpr.timetable_retention_days"
	// Display range for the admin weekly calendar (Apple-style grid).
	// Both are HH:MM strings; the UI renders the visible window between them
	// and scrolls if events fall outside.
	KeyTimetableDayStartTime = "timetable.day_start_time"
	KeyTimetableDayEndTime   = "timetable.day_end_time"
)
