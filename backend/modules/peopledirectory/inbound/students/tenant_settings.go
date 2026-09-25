package students

import (
	"context"
	"log/slog"
	"strconv"
)

// TenantSettings is the consumer-owned port to the Settings Platform (#3356):
// the per-school settings these routes read. Every value resolves as the
// tenant's override, else the registry default; there is no third tier. The
// root binds the Settings Platform service, whose request-scoped snapshot and
// memo cache serve repeated reads of one request.
type TenantSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// Keys and values of the Settings Platform registry these routes read. The
// registry owns them; tenant_settings_test.go pins every one to the owner's
// public name so a renamed key fails there instead of resolving nothing.
const (
	settingAttendanceLogEnabled            = "gdpr.attendance_log_enabled"
	settingAttendanceVisibleDays           = "gdpr.attendance_visible_days"
	settingRoomDetailVisibleDays           = "gdpr.room_detail_visible_days"
	settingPrivacyConsentRetentionDays     = "gdpr.privacy_consent_retention_days"
	settingEnrollmentBookingsAuthoritative = "enrollment.bookings_authoritative"
	settingCareDefaultArrivalTime          = "care_times.default_arrival"
	settingCareDefaultPickupTime           = "care_times.default_pickup"
	settingParentRequestReasonPolicy       = "operations.parent_request_reason_policy"
	settingSessionEndTime                  = "operations.session_end_time"
	settingStudentAbsenceEditScope         = "operations.student_absence_edit_scope"
	settingPresenceMode                    = "operations.presence_mode"
	settingStudentPhotosEnabled            = "operations.student_photos_enabled"
	settingClassArrivalExceptionEditors    = "operations.class_arrival_exception_editors"
	settingFeedbackEnabled                 = "feedback.enabled"
	studentAbsenceEditScopeAllStaff        = "all_staff"
	classArrivalExceptionEditorsAllStaff   = "all_staff"
	schoolPeriodCount                      = 8
	schoolPeriodEndSettingPrefix           = "school_periods.end_"
)

// schoolPeriodEndSetting is the key of one lesson's end time; period is
// 1-based up to schoolPeriodCount.
func schoolPeriodEndSetting(period int) string {
	return schoolPeriodEndSettingPrefix + strconv.Itoa(period)
}

// resolveBoolSetting reads a boolean setting for a handler that degrades
// instead of failing: a missing port or a failed read answers fallback, which
// every caller sets to the registry default, and logs the failure.
func resolveBoolSetting(ctx context.Context, settings TenantSettings, key string, fallback bool, logger *slog.Logger) bool {
	if settings == nil {
		return fallback
	}
	value, err := settings.ResolveBool(ctx, key)
	if err != nil {
		logSettingFallback(logger, key, err)
		return fallback
	}
	return value
}

// resolveIntSetting mirrors resolveBoolSetting for integer settings.
func resolveIntSetting(ctx context.Context, settings TenantSettings, key string, fallback int, logger *slog.Logger) int {
	if settings == nil {
		return fallback
	}
	value, err := settings.ResolveInt(ctx, key)
	if err != nil {
		logSettingFallback(logger, key, err)
		return fallback
	}
	return value
}

func logSettingFallback(logger *slog.Logger, key string, err error) {
	if logger == nil {
		return
	}
	logger.Warn("settings resolve failed, using fallback",
		slog.String("key", key),
		slog.String("error", err.Error()),
	)
}
