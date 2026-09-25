// Package settings is the public contract of Settings Platform (#2736): the
// tenant settings other owners read, and the operator's management of one
// school's settings. The retained settings service in services/config
// serves both; its snapshot and error types are Settings Platform's own and
// appear here under the contract's names.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// Keys of the tenant settings other owners read.
const (
	KeyAnalyticsFreigabe                  = configModel.KeyAnalyticsFreigabe
	KeyAnalyticsRecordingSamplePercent    = configModel.KeyAnalyticsRecordingSamplePercent
	KeyAttendanceEditScope                = configModel.KeyAttendanceEditScope
	KeyAttendanceLogEnabled               = configModel.KeyAttendanceLogEnabled
	KeyAttendanceNFCEnabled               = configModel.KeyAttendanceNFCEnabled
	KeyAttendanceWebEnabled               = configModel.KeyAttendanceWebEnabled
	KeyCalendarCalDAVEnabled              = configModel.KeyCalendarCalDAVEnabled
	KeyCareConcept                        = configModel.KeyCareConcept
	KeyDisplayEnabled                     = configModel.KeyDisplayEnabled
	KeyEmergencyListHealthInfo            = configModel.KeyEmergencyListHealthInfo
	KeyEnrollmentCareOfferingsEnabled     = configModel.KeyEnrollmentCareOfferingsEnabled
	KeyEnrollmentGradeLevelMax            = configModel.KeyEnrollmentGradeLevelMax
	KeyEnrollmentLegalAGBDocumentURL      = configModel.KeyEnrollmentLegalAGBDocumentURL
	KeyEnrollmentWaitlistEnabled          = configModel.KeyEnrollmentWaitlistEnabled
	KeyGroupMode                          = configModel.KeyGroupMode
	KeyOperationalOverviewScope           = configModel.KeyOperationalOverviewScope
	KeyParentNotesEnabled                 = configModel.KeyParentNotesEnabled
	KeyParentRequestReasonPolicy          = configModel.KeyParentRequestReasonPolicy
	KeyPresenceMode                       = configModel.KeyPresenceMode
	KeyStaffMessagingEnabled              = configModel.KeyStaffMessagingEnabled
	KeyStudentAbsenceEditScope            = configModel.KeyStudentAbsenceEditScope
	KeyStudentPhotosEnabled               = configModel.KeyStudentPhotosEnabled
	KeyTimetableChildrenPerStaffRatio     = configModel.KeyTimetableChildrenPerStaffRatio
	KeyTimetableEnabled                   = configModel.KeyTimetableEnabled
	KeyTimetableEnforcePlannedEnd         = configModel.KeyTimetableEnforcePlannedEnd
	KeyTimetableShowExpectedChildrenCount = configModel.KeyTimetableShowExpectedChildrenCount
	KeyTrackingIndicatorsEnabled          = configModel.KeyTrackingIndicatorsEnabled
	KeyTrackingIndicator1                 = configModel.KeyTrackingIndicator1
	KeyTrackingIndicator2                 = configModel.KeyTrackingIndicator2
	KeyTrackingIndicator3                 = configModel.KeyTrackingIndicator3
	KeyWebSpontaneousActivities           = configModel.KeyWebSpontaneousActivities
)

// Values of the enumerated settings above.
const (
	AttendanceEditScopeOwn      = configModel.AttendanceEditScopeOwn
	AttendanceEditScopeAllStaff = configModel.AttendanceEditScopeAllStaff

	CareConceptFixedSchedule = configModel.CareConceptFixedSchedule
	CareConceptOpenRooms     = configModel.CareConceptOpenRooms

	GroupModeFixedGroups = configModel.GroupModeFixedGroups
	GroupModeOpenCare    = configModel.GroupModeOpenCare

	OverviewScopeAdmins   = configModel.OverviewScopeAdmins
	OverviewScopeAllStaff = configModel.OverviewScopeAllStaff
	OverviewScopeOwn      = configModel.OverviewScopeOwn

	PresenceModeDetailed = configModel.PresenceModeDetailed

	ReasonPolicyBoth      = configModel.ReasonPolicyBoth
	ReasonPolicyGuardians = configModel.ReasonPolicyGuardians
	ReasonPolicyNobody    = configModel.ReasonPolicyNobody
	ReasonPolicyStaff     = configModel.ReasonPolicyStaff

	StudentAbsenceEditScopeAllStaff = configModel.StudentAbsenceEditScopeAllStaff
)

// TenantReader resolves one tenant's settings outside its tenant middleware:
// the tenant override, else the registry default.
type TenantReader interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

// Resolver resolves the request tenant's settings inside its tenant
// middleware: the tenant override, else the registry default.
// HasTenantOverride tells the two apart for the Resolve*OrDefault helpers.
type Resolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// ResolveBoolOrDefault returns the tenant override of a boolean setting, or
// fallback when the tenant has none, the resolver is missing or the read
// fails. Failures are logged, never returned.
func ResolveBoolOrDefault(ctx context.Context, resolver Resolver, key string, fallback bool, logger *slog.Logger) bool {
	return configSvc.ResolveBoolOrDefault(ctx, resolver, key, fallback, logger)
}

// ResolveIntOrDefault is ResolveBoolOrDefault for an integer setting.
func ResolveIntOrDefault(ctx context.Context, resolver Resolver, key string, fallback int, logger *slog.Logger) int {
	return configSvc.ResolveIntOrDefault(ctx, resolver, key, fallback, logger)
}

// ResolveStringOrDefault is ResolveBoolOrDefault for a string setting; an
// empty override also yields fallback.
func ResolveStringOrDefault(ctx context.Context, resolver Resolver, key, fallback string, logger *slog.Logger) string {
	return configSvc.ResolveStringOrDefault(ctx, resolver, key, fallback, logger)
}

// Snapshot is a batch of one tenant's resolved settings. A reader that
// resolves many keys at once returns it from
// ResolveManyForTenant(ctx, tenantID, keys).
type Snapshot = configSvc.SettingsSnapshot

// ResolvePresenceModeForTenant returns the tenant's presence mode, or the
// detailed mode when the reader is missing or fails.
func ResolvePresenceModeForTenant(ctx context.Context, reader TenantReader, tenantID int64) string {
	if reader == nil {
		return PresenceModeDetailed
	}
	return configSvc.ResolvePresenceModeForTenant(ctx, reader, tenantID, nil)
}

// The settings errors the operator routes map to responses.
type (
	SettingsError           = configSvc.SettingsError
	DefinitionNotFoundError = configSvc.DefinitionNotFoundError
	InvalidValueError       = configSvc.InvalidValueError
	PermissionDeniedError   = configSvc.PermissionDeniedError
)

// ErrPresenceModeSwitchBlocked reports a presence-mode switch while
// attendance of the day is still open.
var ErrPresenceModeSwitchBlocked = configSvc.ErrPresenceModeSwitchBlocked

// ErrBookingAuthorityImpactUnavailable reports a deployment composed without
// the Care Plan booking authority.
var ErrBookingAuthorityImpactUnavailable = errors.New("booking authority impact is not configured")

// OperatorSchoolSettings is the operator's management of one school's
// settings. Operators bypass the per-setting permission checks except for
// admin-only settings, which CheckOperatorWritable refuses.
type OperatorSchoolSettings interface {
	// CheckOperatorWritable refuses a key the operator may not touch, with a
	// *DefinitionNotFoundError for an unknown key.
	CheckOperatorWritable(key string) error
	// Schema returns the school's full settings schema with its resolved
	// values, as the JSON the operator routes render.
	Schema(ctx context.Context, schoolID int64) (json.RawMessage, error)
	// Reveal returns the unmasked value of a school's setting.
	Reveal(ctx context.Context, schoolID int64, key string) (any, error)
	// SetValue writes a school's setting and runs its side effects; force
	// bypasses the presence-mode switch guard for operational recovery.
	SetValue(ctx context.Context, schoolID int64, key string, value any, changedBy int64, force bool) error
	// ResetValue removes a school's override.
	ResetValue(ctx context.Context, schoolID int64, key string, changedBy int64) error
	// BookingAuthorityImpact previews enabling booking-led care for the
	// school today.
	BookingAuthorityImpact(ctx context.Context, schoolID int64) (*careplan.BookingAuthorityImpact, error)
}
