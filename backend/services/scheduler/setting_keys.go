package scheduler

import (
	"context"
	"log/slog"
)

// Keys and values of the Settings Platform registry the Worker reads
// (#2746). The registry owns them; the root refuses a Worker whose scheduler
// names a key the registry does not define, and api/scheduler_ports_test.go
// pins every key to the registry and keeps secrets out of the snapshot.
const (
	settingDataCleanupEnabled                    = "gdpr.data_cleanup_enabled"
	settingDataCleanupTime                       = "gdpr.data_cleanup_time"
	settingDataCleanupTimeoutMinutes             = "gdpr.data_cleanup_timeout_minutes"
	settingFeedbackDataRetentionDays             = "feedback.data_retention_days"
	settingSessionEndEnabled                     = "operations.session_end_enabled"
	settingSessionEndTime                        = "operations.session_end_time"
	settingSessionEndTimeoutMinutes              = "operations.session_end_timeout_minutes"
	settingSessionCleanupEnabled                 = "operations.session_cleanup_enabled"
	settingSessionCleanupIntervalMinutes         = "operations.session_cleanup_interval_minutes"
	settingSessionAbandonedThresholdMin          = "operations.session_abandoned_threshold_minutes"
	settingTrackingAutoCheckoutEnabled           = "tracking.auto_checkout_enabled"
	settingTrackingAutoCheckoutGraceMinutes      = "tracking.auto_checkout_grace_minutes"
	settingSickClearMode                         = "operations.sick_clear_mode"
	settingExcusedClearMode                      = "operations.excused_clear_mode"
	settingPresenceMode                          = "operations.presence_mode"
	settingTimetableEnabled                      = "timetable.enabled"
	settingTimetableMaterializationEnabled       = "timetable.materialization_enabled"
	settingTimetableMaterializationWeekday       = "timetable.materialization_weekday"
	settingTimetableMaterializationWeeksAhead    = "timetable.materialization_weeks_ahead"
	settingTimetableAutoStartPlanned             = "timetable.auto_start_planned"
	settingTimetableAutoEndEnabled               = "timetable.auto_end_enabled"
	settingTimetableAutoEndGraceMinutes          = "timetable.auto_end_grace_minutes"
	settingTimetableOverdueThresholdMinutes      = "timetable.overdue_threshold_minutes"
	settingNotificationsDispatchEnabled          = "notifications.dispatch_enabled"
	settingNotificationsOnDutyOnly               = "notifications.on_duty_only"
	settingRemindersPickupUpcomingEnabled        = "reminders.pickup_upcoming_enabled"
	settingRemindersPickupOverdueEnabled         = "reminders.pickup_overdue_enabled"
	settingRemindersActivityStartEnabled         = "reminders.activity_start_enabled"
	settingRemindersActivityOverdueEnabled       = "reminders.activity_overdue_enabled"
	settingRemindersPickupUpcomingLeadMinutes    = "reminders.pickup_upcoming_lead_minutes"
	settingRemindersActivityStartLeadMinutes     = "reminders.activity_start_lead_minutes"
	settingCalendarAppointmentReminderEnabled    = "calendar.appointment_reminder_enabled"
	settingCalendarAppointmentReminderLeadHours  = "calendar.appointment_reminder_lead_hours"
	settingEnrollmentWaitlistEnabled             = "enrollment.waitlist_enabled"
	settingEnrollmentAutoInviteGuardianOnApprove = "enrollment.auto_invite_guardian_on_approval"
	settingEnrollmentCareOfferingsEnabled        = "enrollment.care_offerings_enabled"
	settingEnrollmentDefaultActivationMode       = "enrollment.default_activation_mode"
	settingEnrollmentNotifyPerDecision           = "enrollment.notify_per_decision"
	settingEnrollmentOutboxMaxAttempts           = "enrollment.outbox_max_attempts"
	settingEnrollmentOutboxWorkerIntervalSeconds = "enrollment.outbox_worker_interval_seconds"

	clearModeEndOfDay = "end_of_day"
)

// SettingKeys returns every Settings Platform key the Worker reads: the
// polling keys of the minute snapshot plus the keys read outside a school.
func SettingKeys() []string {
	keys := append([]string(nil), schedulerPollingSettingKeys...)
	return append(keys, settingEnrollmentOutboxWorkerIntervalSeconds, settingEnrollmentOutboxMaxAttempts)
}

// The settings reads below keep the semantics of the Settings Platform's
// Resolve*OrDefault helpers: a school's override wins, everything else
// answers fallback, and a failed read is logged and answers fallback too.

func settingOverride(ctx context.Context, settings SettingsResolver, key string, logger *slog.Logger) bool {
	if settings == nil {
		return false
	}
	has, err := settings.HasTenantOverride(ctx, key)
	if err != nil {
		logSettingFallback(logger, "settings override check failed, using fallback", key, err)
		return false
	}
	return has
}

func resolveStringOrDefault(ctx context.Context, settings SettingsResolver, key, fallback string, logger *slog.Logger) string {
	if !settingOverride(ctx, settings, key, logger) {
		return fallback
	}
	value, err := settings.ResolveString(ctx, key)
	if err != nil {
		logSettingFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	if value == "" {
		return fallback
	}
	return value
}

func resolveBoolOrDefault(ctx context.Context, settings SettingsResolver, key string, fallback bool, logger *slog.Logger) bool {
	if !settingOverride(ctx, settings, key, logger) {
		return fallback
	}
	value, err := settings.ResolveBool(ctx, key)
	if err != nil {
		logSettingFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	return value
}

func resolveIntOrDefault(ctx context.Context, settings SettingsResolver, key string, fallback int, logger *slog.Logger) int {
	if !settingOverride(ctx, settings, key, logger) {
		return fallback
	}
	value, err := settings.ResolveInt(ctx, key)
	if err != nil {
		logSettingFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	return value
}

func logSettingFallback(logger *slog.Logger, message, key string, err error) {
	if logger == nil {
		return
	}
	logger.Warn(message,
		slog.String("key", key),
		slog.String("error", err.Error()),
	)
}
