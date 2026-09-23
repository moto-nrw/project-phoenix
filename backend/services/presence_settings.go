package services

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// settingsSource is the tenant-settings surface the presence adapters read
// through. config.SettingsService satisfies it.
type settingsSource interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// presenceSettings answers the presence and workforce services' settings
// questions by name. Holding the registry keys here rather than in
// modules/studentpresence/internal/application/presence keeps the settings vocabulary with its owner and turns a
// renamed key into a compile error in this file, instead of a runtime miss
// deep inside a presence flow.
type presenceSettings struct{ source settingsSource }

// PresenceSettings binds the tenant settings service behind the named
// questions the presence and workforce services ask.
func PresenceSettings(source settingsSource) presenceSettings {
	return presenceSettings{source: source}
}

// AbsenceApprovalEmailEnabled reports whether the tenant sends an email when
// an absence request is decided.
func (s presenceSettings) AbsenceApprovalEmailEnabled(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyNotificationsAbsenceApprovalEmail)
}

// AccountStartDate is the day the tenant's time accounts begin, as a
// YYYY-MM-DD string, or empty when the tenant has not set one.
func (s presenceSettings) AccountStartDate(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
}

// EnforcePlannedStart reports whether a check-in outside the planned shift is
// refused.
func (s presenceSettings) EnforcePlannedStart(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyTimeTrackingEnforcePlannedStart)
}

// RequireDeviationReason reports whether a self-edit that moves recorded times
// must carry a reason.
func (s presenceSettings) RequireDeviationReason(ctx context.Context) (bool, error) {
	return s.source.ResolveBool(ctx, configModels.KeyTimeTrackingRequireDeviationReason)
}

// DeviationToleranceMinutes is the grace window before a deviation counts.
func (s presenceSettings) DeviationToleranceMinutes(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyTimeTrackingDeviationToleranceMinutes)
}

// TimeTrackingRetentionDays is how long the time-tracking audit feed keeps its
// entries.
func (s presenceSettings) TimeTrackingRetentionDays(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeyGDPRTimeTrackingRetentionDays)
}

// PresenceMode is the tenant's attendance granularity.
func (s presenceSettings) PresenceMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyPresenceMode)
}

// SickClearMode says when a reported sick flag is cleared.
func (s presenceSettings) SickClearMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeySickClearMode)
}

// ExcusedClearMode says when a reported excused flag is cleared.
func (s presenceSettings) ExcusedClearMode(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyExcusedClearMode)
}

// SessionInactivityTimeoutMinutes is how long a kiosk session may idle.
func (s presenceSettings) SessionInactivityTimeoutMinutes(ctx context.Context) (int, error) {
	return s.source.ResolveInt(ctx, configModels.KeySessionInactivityTimeoutMin)
}

// AttendanceEditScope gates who may edit recorded attendance.
func (s presenceSettings) AttendanceEditScope(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyAttendanceEditScope)
}

// OperationalOverviewScope gates who sees the tenant-wide overview.
func (s presenceSettings) OperationalOverviewScope(ctx context.Context) (string, error) {
	return s.source.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
}

// TimeTrackingRetentionOverridden probes whether the tenant set its own
// retention window. The cleanup logs a failing probe and falls back.
func (s presenceSettings) TimeTrackingRetentionOverridden(ctx context.Context) (bool, error) {
	return s.source.HasTenantOverride(ctx, configModels.KeyGDPRTimeTrackingRetentionDays)
}
