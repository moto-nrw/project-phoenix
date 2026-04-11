package config

// Settings key constants. Use these instead of string literals to ensure
// compile-time safety when referencing settings keys across the codebase.

// Security settings.
const (
	KeyOGSDevicePIN = "security.ogs_device_pin"
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

// Operations settings.
const (
	KeySessionEndEnabled             = "operations.session_end_enabled"
	KeySessionEndTime                = "operations.session_end_time"
	KeySessionEndTimeoutMinutes      = "operations.session_end_timeout_minutes"
	KeyStudentDailyCheckoutTime      = "operations.student_daily_checkout_time"
	KeySessionCleanupEnabled         = "operations.session_cleanup_enabled"
	KeySessionCleanupIntervalMinutes = "operations.session_cleanup_interval_minutes"
	KeySessionAbandonedThresholdMin  = "operations.session_abandoned_threshold_minutes"
	KeyAdminSupervisionOverview      = "operations.admin_supervision_overview"
)
