package config

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// PresenceSettingsSource is the tenant-settings surface the presence adapters read
// through. config.SettingsService satisfies it.
type PresenceSettingsSource interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// PresenceSettings answers the presence and workforce services' settings
// questions by name. Holding the registry keys here rather than in
// modules/studentpresence/internal/application/presence keeps the settings vocabulary with its owner and turns a
// renamed key into a compile error in this file, instead of a runtime miss
// deep inside a presence flow.
type PresenceSettings struct{ source PresenceSettingsSource }

// PresenceSettings binds the tenant settings service behind the named
// questions the presence and workforce services ask.
func NewPresenceSettings(source PresenceSettingsSource) PresenceSettings {
	return PresenceSettings{source: source}
}

// AbsenceApprovalEmailEnabled reports whether the tenant sends an email when
// an absence request is decided.
func (s PresenceSettings) AbsenceApprovalEmailEnabled(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyNotificationsAbsenceApprovalEmail)
}

// AccountStartDate is the day the tenant's time accounts begin, as a
// YYYY-MM-DD string, or empty when the tenant has not set one.
func (s PresenceSettings) AccountStartDate(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
}

// EnforcePlannedStart reports whether a check-in before the planned shift
// start (minus its tolerance) is refused.
func (s PresenceSettings) EnforcePlannedStart(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyTimeTrackingEnforcePlannedStart)
}

// PlannedStartToleranceMinutes is how many minutes before the planned shift
// start a check-in opens while the planned start is enforced.
func (s PresenceSettings) PlannedStartToleranceMinutes(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyTimeTrackingPlannedStartToleranceMinutes)
}

// RequireDeviationReason reports whether a self-edit that moves recorded times
// must carry a reason.
func (s PresenceSettings) RequireDeviationReason(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyTimeTrackingRequireDeviationReason)
}

// DeviationToleranceMinutes is the grace window before a deviation counts.
func (s PresenceSettings) DeviationToleranceMinutes(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyTimeTrackingDeviationToleranceMinutes)
}

// TimeTrackingRetentionDays is how long the time-tracking audit feed keeps its
// entries.
func (s PresenceSettings) TimeTrackingRetentionDays(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyGDPRTimeTrackingRetentionDays)
}

// VisitRetentionDays is how long completed visits of a child without an
// accepted consent are kept.
func (s PresenceSettings) VisitRetentionDays(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyPrivacyConsentRetentionDays)
}

// PresenceMode is the tenant's attendance granularity.
func (s PresenceSettings) PresenceMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyPresenceMode)
}

// SickClearMode says when a reported sick flag is cleared.
func (s PresenceSettings) SickClearMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeySickClearMode)
}

// ExcusedClearMode says when a reported excused flag is cleared.
func (s PresenceSettings) ExcusedClearMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyExcusedClearMode)
}

// SessionInactivityTimeoutMinutes is how long a kiosk session may idle.
func (s PresenceSettings) SessionInactivityTimeoutMinutes(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeySessionInactivityTimeoutMin)
}

// AttendanceEditScope gates who may edit recorded attendance.
func (s PresenceSettings) AttendanceEditScope(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyAttendanceEditScope)
}

// OperationalOverviewScope gates who sees the tenant-wide overview.
func (s PresenceSettings) OperationalOverviewScope(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
}

// WebParticipantLimitEnforced reports whether web assignments respect the
// activity's participant limit. The stored setting says whether exceeding it
// is allowed, so the answer is its negation.
func (s PresenceSettings) WebParticipantLimitEnforced(ctx context.Context) (bool, error) {
	allowed, err := s.source.ResolveBool(ctx, configModels.KeyWebExceedParticipantLimit)
	if err != nil {
		return false, err
	}
	return !allowed, nil
}

// TimeTrackingRetentionOverridden probes whether the tenant set its own
// retention window. The cleanup logs a failing probe and falls back.
func (s PresenceSettings) TimeTrackingRetentionOverridden(ctx context.Context) (bool, error) {
	return s.source.HasTenantOverride(ctx, configModels.KeyGDPRTimeTrackingRetentionDays)
}
