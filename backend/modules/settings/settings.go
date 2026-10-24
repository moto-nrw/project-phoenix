// Package settings is the public contract of Settings Platform (#2736): the
// tenant settings other owners read, and the operator's management of one
// school's settings. The retained settings service in services/config
// serves both; its snapshot and error types are Settings Platform's own and
// appear here under the contract's names.
package settings

import (
	"context"
	"encoding/json"
	"log/slog"

	configSvc "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Keys of the tenant settings other owners read.
const (
	KeyAnalyticsEnabled                      = configModel.KeyAnalyticsEnabled
	KeyAnalyticsFreigabe                     = configModel.KeyAnalyticsFreigabe
	KeyAnalyticsRecordingSamplePercent       = configModel.KeyAnalyticsRecordingSamplePercent
	KeyAttendanceEditScope                   = configModel.KeyAttendanceEditScope
	KeyAttendanceLogEnabled                  = configModel.KeyAttendanceLogEnabled
	KeyAttendanceNFCEnabled                  = configModel.KeyAttendanceNFCEnabled
	KeyAttendanceVisibleDays                 = configModel.KeyAttendanceVisibleDays
	KeyAttendanceWebEnabled                  = configModel.KeyAttendanceWebEnabled
	KeyCalendarCalDAVEnabled                 = configModel.KeyCalendarCalDAVEnabled
	KeyCareDefaultArrivalTime                = configModel.KeyCareDefaultArrivalTime
	KeyCareDefaultPickupTime                 = configModel.KeyCareDefaultPickupTime
	KeyClassArrivalExceptionEditors          = configModel.KeyClassArrivalExceptionEditors
	KeyDisplayEnabled                        = configModel.KeyDisplayEnabled
	KeyEarlyCheckoutNoteEnabled              = configModel.KeyEarlyCheckoutNoteEnabled
	KeyEarlyCheckoutNoteToleranceMinutes     = configModel.KeyEarlyCheckoutNoteToleranceMinutes
	KeyEnrollmentBookingsAuthoritative       = configModel.KeyEnrollmentBookingsAuthoritative
	KeyEnrollmentAutoInviteGuardianOnApprove = configModel.KeyEnrollmentAutoInviteGuardianOnApprove
	KeyEnrollmentCareOfferingsEnabled        = configModel.KeyEnrollmentCareOfferingsEnabled
	KeyEnrollmentCollectGradeLevel           = configModel.KeyEnrollmentCollectGradeLevel
	KeyEnrollmentDuplicateHandling           = configModel.KeyEnrollmentDuplicateHandling
	KeyEnrollmentGradeLevelMax               = configModel.KeyEnrollmentGradeLevelMax
	KeyEnrollmentLegalAGBDocumentURL         = configModel.KeyEnrollmentLegalAGBDocumentURL
	KeyEnrollmentNotifyPerDecision           = configModel.KeyEnrollmentNotifyPerDecision
	KeyEnrollmentRejectedRetentionDays       = configModel.KeyEnrollmentRejectedRetentionDays
	KeyEnrollmentWaitlistEnabled             = configModel.KeyEnrollmentWaitlistEnabled
	KeyChildSearchOnlyNavigation             = configModel.KeyChildSearchOnlyNavigation
	KeyFeedbackEnabled                       = configModel.KeyFeedbackEnabled
	KeyGroupMode                             = configModel.KeyGroupMode
	KeyOGSDevicePIN                          = configModel.KeyOGSDevicePIN
	KeyGDPRParentMessageRetentionDays        = configModel.KeyGDPRParentMessageRetentionDays
	KeyGDPRWithdrawnEnrollmentRetentionDays  = configModel.KeyGDPRWithdrawnEnrollmentRetentionDays
	KeyGDPRStatusDayRetentionDays            = configModel.KeyGDPRStatusDayRetentionDays
	KeyGDPRAttendanceRetentionDays           = configModel.KeyGDPRAttendanceRetentionDays
	KeyGDPRMorningAttendanceRetentionDays    = configModel.KeyGDPRMorningAttendanceRetentionDays
	KeyGDPRStaffSickNoteRetentionDays        = configModel.KeyGDPRStaffSickNoteRetentionDays
	KeyGDPRStudentRetentionDays              = configModel.KeyGDPRStudentRetentionDays
	KeyGDPRTimetableRetentionDays            = configModel.KeyGDPRTimetableRetentionDays
	KeyGDPRGuardianRetentionDays             = configModel.KeyGDPRGuardianRetentionDays
	KeyGDPRTimeTrackingRetentionDays         = configModel.KeyGDPRTimeTrackingRetentionDays
	KeyGuardianParentCanRemove               = configModel.KeyGuardianParentCanRemove
	KeyGuardianParentInviteMode              = configModel.KeyGuardianParentInviteMode
	KeyOperationalOverviewScope              = configModel.KeyOperationalOverviewScope
	KeyParentAbsenceReviewScope              = configModel.KeyParentAbsenceReviewScope
	KeyParentCareModeRequestEnabled          = configModel.KeyParentCareModeRequestEnabled
	KeyParentCarePickupRequestEnabled        = configModel.KeyParentCarePickupRequestEnabled
	KeyParentExcusedRequiresApproval         = configModel.KeyParentExcusedRequiresApproval
	KeyParentNotesEnabled                    = configModel.KeyParentNotesEnabled
	KeyParentPickupChangeCutoffTime          = configModel.KeyParentPickupChangeCutoffTime
	KeyParentPickupChangeEnabled             = configModel.KeyParentPickupChangeEnabled
	KeyParentRequestGroupLeaderReviewEnabled = configModel.KeyParentRequestGroupLeaderReviewEnabled
	KeyParentRequestReasonPolicy             = configModel.KeyParentRequestReasonPolicy
	KeyParentRequestReviewScope              = configModel.KeyParentRequestReviewScope
	KeyParentSickRequiresApproval            = configModel.KeyParentSickRequiresApproval
	KeyPresenceMode                          = configModel.KeyPresenceMode
	KeyPrivacyConsentRetentionDays           = configModel.KeyPrivacyConsentRetentionDays
	KeyRequirePickupOfferingReview           = configModel.KeyRequirePickupOfferingReview
	KeyRoomDetailVisibleDays                 = configModel.KeyRoomDetailVisibleDays
	KeySessionEndTime                        = configModel.KeySessionEndTime
	KeySlotListLongDayCutoff                 = configModel.KeySlotListLongDayCutoff
	KeySlotListShortDayCutoff                = configModel.KeySlotListShortDayCutoff
	KeyStaffMessagingEnabled                 = configModel.KeyStaffMessagingEnabled
	KeyStudentAbsenceEditScope               = configModel.KeyStudentAbsenceEditScope
	KeyStudentPhotosEnabled                  = configModel.KeyStudentPhotosEnabled
	KeyTimetableChildrenPerStaffRatio        = configModel.KeyTimetableChildrenPerStaffRatio
	KeyTimetableEnabled                      = configModel.KeyTimetableEnabled
	KeyTimetableEnforcePlannedEnd            = configModel.KeyTimetableEnforcePlannedEnd
	KeyTimetableCompleteLeadMinutes          = configModel.KeyTimetableCompleteLeadMinutes
	KeyTimetableShowExpectedChildrenCount    = configModel.KeyTimetableShowExpectedChildrenCount
	KeyWeekendFollowsFriday                  = configModel.KeyWeekendFollowsFriday
	KeyTrackingIndicator1                    = configModel.KeyTrackingIndicator1
	KeyTrackingIndicator2                    = configModel.KeyTrackingIndicator2
	KeyTrackingIndicator3                    = configModel.KeyTrackingIndicator3
	KeyTrackingIndicatorsEnabled             = configModel.KeyTrackingIndicatorsEnabled
	KeyWebSpontaneousActivities              = configModel.KeyWebSpontaneousActivities
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

	ParentInviteModeDirect   = configModel.ParentInviteModeDirect
	ParentInviteModeDisabled = configModel.ParentInviteModeDisabled

	PresenceModeBinary   = configModel.PresenceModeBinary
	PresenceModeDetailed = configModel.PresenceModeDetailed

	ReasonPolicyBoth      = configModel.ReasonPolicyBoth
	ReasonPolicyGuardians = configModel.ReasonPolicyGuardians
	ReasonPolicyNobody    = configModel.ReasonPolicyNobody
	ReasonPolicyStaff     = configModel.ReasonPolicyStaff

	StudentAbsenceEditScopeAdmins   = configModel.StudentAbsenceEditScopeAdmins
	StudentAbsenceEditScopeAllStaff = configModel.StudentAbsenceEditScopeAllStaff

	ClassArrivalExceptionEditorsAllStaff = configModel.ClassArrivalExceptionEditorsAllStaff

	ParentAbsenceReviewScopeAdmins       = configModel.ParentAbsenceReviewScopeAdmins
	ParentAbsenceReviewScopeAllStaff     = configModel.ParentAbsenceReviewScopeAllStaff
	ParentAbsenceReviewScopeGroupLeaders = configModel.ParentAbsenceReviewScopeGroupLeaders

	ParentRequestReviewScopeAdmins       = configModel.ParentRequestReviewScopeAdmins
	ParentRequestReviewScopeAllStaff     = configModel.ParentRequestReviewScopeAllStaff
	ParentRequestReviewScopeGroupLeaders = configModel.ParentRequestReviewScopeGroupLeaders
	ParentRequestReviewScopeInherit      = configModel.ParentRequestReviewScopeInherit
)

// SchoolPeriodCount is the number of lessons a school day may name.
const SchoolPeriodCount = configModel.SchoolPeriodCount

// SchoolPeriodEndKey returns the key of one lesson's end time; period is
// 1-based up to SchoolPeriodCount.
var SchoolPeriodEndKey = configModel.SchoolPeriodEndKey

// TenantReader resolves one tenant's settings outside its tenant middleware:
// the tenant override, else the registry default.
type TenantReader interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

// Resolver resolves the request tenant's settings inside its tenant
// middleware: the tenant override, else the registry default.
// HasTenantOverride distinguishes explicit choices from registry defaults.
type Resolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// ResolveIntOrDefault resolves the tenant override or registry default.
// Only unavailable reads use fallback. Privacy and authorization callers use
// the typed resolver directly so they can propagate errors. Missing wiring panics.
func ResolveIntOrDefault(ctx context.Context, resolver Resolver, key string, fallback int, logger *slog.Logger) int {
	return configSvc.ResolveIntOrDefault(ctx, resolver, key, fallback, logger)
}

// Snapshot is a batch of one tenant's resolved settings. A reader that
// resolves many keys at once returns it from
// ResolveManyForTenant(ctx, tenantID, keys).
type Snapshot = configModel.SettingsSnapshot

// The settings errors the operator routes map to responses.
type (
	SettingsError           = configModel.SettingsError
	DefinitionNotFoundError = configModel.DefinitionNotFoundError
	InvalidValueError       = configModel.InvalidValueError
	PermissionDeniedError   = configModel.PermissionDeniedError
)

// AttendanceWindowError rejects a visible attendance window longer than the
// attendance retention window (#3957).
type AttendanceWindowError = configModel.AttendanceWindowError

// ErrPresenceModeSwitchBlocked reports a presence-mode switch while
// attendance of the day is still open.
var ErrPresenceModeSwitchBlocked = configModel.ErrPresenceModeSwitchBlocked

// BookingAuthorityBlockedError reports that the side effect of enabling
// booking-led care (KeyEnrollmentBookingsAuthoritative) refused the write
// because a cared-for child has no care day. The root's side-effect binding
// wraps the owner's refusal in it, so Settings Platform needs no domain
// vocabulary to answer the conflict; Error is the owner's message.
type BookingAuthorityBlockedError struct {
	Reason error
}

func (e *BookingAuthorityBlockedError) Error() string { return e.Reason.Error() }

func (e *BookingAuthorityBlockedError) Unwrap() error { return e.Reason }

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
}

// WithSettingsSnapshot attaches the immutable read snapshot used by this owner's
// resolver. Writes must not prefetch the keys they change.
func WithSettingsSnapshot(ctx context.Context, snapshot *Snapshot) context.Context {
	return configModel.WithSettingsSnapshot(ctx, snapshot)
}

// WithSettingsRequestCache attaches the tenant-keyed memo used by the resolver.
// Repeated attachment keeps the same cache; successful writes evict its entries.
func WithSettingsRequestCache(ctx context.Context) context.Context {
	return configModel.WithSettingsRequestCache(ctx)
}
