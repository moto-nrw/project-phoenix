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

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// Keys of the tenant settings other owners read.
const (
	KeyAttendanceEditScope                = configModel.KeyAttendanceEditScope
	KeyAttendanceLogEnabled               = configModel.KeyAttendanceLogEnabled
	KeyAttendanceNFCEnabled               = configModel.KeyAttendanceNFCEnabled
	KeyAttendanceWebEnabled               = configModel.KeyAttendanceWebEnabled
	KeyCalendarCalDAVEnabled              = configModel.KeyCalendarCalDAVEnabled
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
	KeyStudentPhotosEnabled               = configModel.KeyStudentPhotosEnabled
	KeyTimetableEnabled                   = configModel.KeyTimetableEnabled
	KeyTimetableShowExpectedChildrenCount = configModel.KeyTimetableShowExpectedChildrenCount
)

// Values of the enumerated settings above.
const (
	AttendanceEditScopeOwn      = configModel.AttendanceEditScopeOwn
	AttendanceEditScopeAllStaff = configModel.AttendanceEditScopeAllStaff

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
)

// TenantReader resolves one tenant's settings outside its tenant middleware:
// the tenant override, else the registry default.
type TenantReader interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
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
