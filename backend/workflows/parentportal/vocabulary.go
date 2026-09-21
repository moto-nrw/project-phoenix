package parentportal

// The workflow's vocabulary as its consumers name it: the guardian portal HTTP
// composition matches these result types and sentinel errors, and the
// composition root binds the ports. Each name is declared once, in the flow
// package that owns it.

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

type (
	AttendanceReader                = care.AttendanceReader
	CareException                   = care.CareException
	CareExceptions                  = care.CareExceptions
	CareOfferingSelection           = care.CareOfferingSelection
	PickupChangeRequest             = care.PickupChangeRequest
	CareRequestDiffEntry            = care.CareRequestDiffEntry
	CareScheduleRequestCapabilities = care.CareScheduleRequestCapabilities
	CareScheduleWeekday             = care.CareScheduleWeekday
	ChildCareOfferings              = care.ChildCareOfferings
	ChildCareSchedule               = care.ChildCareSchedule
	ChildConsent                    = care.ChildConsent
	ChildFeatureFlags               = care.ChildFeatureFlags
	ChildGuardian                   = care.ChildGuardian
	ChildMasterData                 = care.ChildMasterData
	CreateGuardianContactInput      = care.CreateGuardianContactInput
	GuardianAccess                  = care.GuardianAccess
	GuardianAccessRevocation        = care.GuardianAccessRevocation
	GuardianContactInput            = care.GuardianContactInput
	GuardianInvitationRecord        = care.GuardianInvitationRecord
	GuardianInviteOutcome           = care.GuardianInviteOutcome
	GuardianInviteRequest           = care.GuardianInviteRequest
	GuardianPhoneInput              = care.GuardianPhoneInput
	GuardianRelationshipInput       = care.GuardianRelationshipInput
	InviteRelatedAccountResult      = care.InviteRelatedAccountResult
	MasterDataFieldChange           = care.MasterDataFieldChange
	MealParticipationDay            = care.MealParticipationDay
	MealParticipationPlan           = care.MealParticipationPlan
	MealPlanEntry                   = care.MealPlanEntry
	MealWeekday                     = care.MealWeekday
	MessageThreadView               = messaging.MessageThreadView
	ParentRequestEventView          = messaging.ParentRequestEventView
	PendingOfferingChange           = care.PendingOfferingChange
	Profile                         = care.Profile
	RelatedAccount                  = care.RelatedAccount
	RequestSharingRecipient         = messaging.RequestSharingRecipient
	RequestSharingService           = messaging.RequestSharingService
	RequestSharingState             = messaging.RequestSharingState
	SickNoteResult                  = care.SickNoteResult
	StudentPhotoUnlinker            = care.StudentPhotoUnlinker
	TodayStatus                     = care.TodayStatus
)

const (
	DayStatePresent              = care.DayStatePresent
	DayStateUnknown              = care.DayStateUnknown
	RelatedAccountActive         = care.RelatedAccountActive
	RelatedAccountActiveNoAccess = care.RelatedAccountActiveNoAccess
	RelatedAccountNoAccount      = care.RelatedAccountNoAccount
	RelatedAccountPending        = care.RelatedAccountPending
	RequestShareCareSchedule     = care.RequestShareCareSchedule
	RequestShareExcused          = care.RequestShareExcused
	RequestShareMasterData       = care.RequestShareMasterData
)

var (
	ErrAnnouncementAckNotRequired       = messaging.ErrAnnouncementAckNotRequired
	ErrAnnouncementNotAPoll             = messaging.ErrAnnouncementNotAPoll
	ErrAnnouncementNotFound             = messaging.ErrAnnouncementNotFound
	ErrAnnouncementStale                = messaging.ErrAnnouncementStale
	ErrCannotRemoveOwnAccess            = care.ErrCannotRemoveOwnAccess
	ErrCannotRemovePayerGuardian        = care.ErrCannotRemovePayerGuardian
	ErrCannotRemovePrimaryGuardian      = care.ErrCannotRemovePrimaryGuardian
	ErrCannotRemoveStaffManagedGuardian = care.ErrCannotRemoveStaffManagedGuardian
	ErrCareDateTooFar                   = care.ErrCareDateTooFar
	ErrCareExceptionAlreadyLeft         = care.ErrCareExceptionAlreadyLeft
	ErrCareExceptionConflict            = care.ErrCareExceptionConflict
	ErrCareExceptionRaced               = care.ErrCareExceptionRaced
	ErrCareExceptionReasonRequired      = care.ErrCareExceptionReasonRequired
	ErrCareExceptionReasonTooLong       = care.ErrCareExceptionReasonTooLong
	ErrCareRequestAlreadyPending        = care.ErrCareRequestAlreadyPending
	ErrCareRequestBookingsAuthoritative = care.ErrCareRequestBookingsAuthoritative
	ErrCareRequestFieldDisabled         = care.ErrCareRequestFieldDisabled
	ErrCareRequestNotFound              = care.ErrCareRequestNotFound
	ErrCareRequestNotPending            = care.ErrCareRequestNotPending
	ErrChildCareEnded                   = care.ErrChildCareEnded
	ErrChildNotAnswerable               = messaging.ErrChildNotAnswerable
	ErrChildNotLinked                   = care.ErrChildNotLinked
	ErrEmailRequired                    = care.ErrEmailRequired
	ErrEmptyNote                        = care.ErrEmptyNote
	ErrExcusedRequestNotFound           = care.ErrExcusedRequestNotFound
	ErrExcusedRequestNotPending         = care.ErrExcusedRequestNotPending
	ErrExcusedRequestOverlap            = care.ErrExcusedRequestOverlap
	ErrGuardianContactInvalid           = care.ErrGuardianContactInvalid
	ErrGuardianEmailConflict            = care.ErrGuardianEmailConflict
	ErrGuardianHasOwnAccount            = care.ErrGuardianHasOwnAccount
	ErrGuardianManagementDisabled       = care.ErrGuardianManagementDisabled
	ErrGuardianNoChange                 = care.ErrGuardianNoChange
	ErrGuardianNotLinked                = care.ErrGuardianNotLinked
	ErrGuardianPermissionDenied         = care.ErrGuardianPermissionDenied
	ErrGuardianRelationshipInvalid      = care.ErrGuardianRelationshipInvalid
	ErrGuardianRoleManaged              = care.ErrGuardianRoleManaged
	ErrGuardianSharedAcrossFamilies     = care.ErrGuardianSharedAcrossFamilies
	ErrGuardianSocialWorkerManaged      = care.ErrGuardianSocialWorkerManaged
	ErrInvalidCareRequestPayload        = care.ErrInvalidCareRequestPayload
	ErrInvalidInviteInput               = care.ErrInvalidInviteInput
	ErrInvalidMealParticipation         = care.ErrInvalidMealParticipation
	ErrInvalidPollResponse              = messaging.ErrInvalidPollResponse
	ErrInvalidStatus                    = care.ErrInvalidStatus
	ErrInviteDisabled                   = care.ErrInviteDisabled
	ErrInviteSocialWorkerManaged        = care.ErrInviteSocialWorkerManaged
	ErrMasterDataDuplicatePending       = care.ErrMasterDataDuplicatePending
	ErrMasterDataEditDisabled           = care.ErrMasterDataEditDisabled
	ErrMasterDataFieldNotEditable       = care.ErrMasterDataFieldNotEditable
	ErrMasterDataInvalidValue           = care.ErrMasterDataInvalidValue
	ErrMasterDataNoChanges              = care.ErrMasterDataNoChanges
	ErrMasterDataRequestDisabled        = care.ErrMasterDataRequestDisabled
	ErrMealParticipationCutoff          = care.ErrMealParticipationCutoff
	ErrMealParticipationOutOfRange      = care.ErrMealParticipationOutOfRange
	ErrMealPlanDisabled                 = care.ErrMealPlanDisabled
	ErrMealPlanWeekOutOfRange           = care.ErrMealPlanWeekOutOfRange
	ErrMealRegistrationDisabled         = care.ErrMealRegistrationDisabled
	ErrNoCareException                  = care.ErrNoCareException
	ErrNoDates                          = care.ErrNoDates
	ErrNoteTooLong                      = care.ErrNoteTooLong
	ErrNotesDisabled                    = care.ErrNotesDisabled
	ErrPastCareDate                     = care.ErrPastCareDate
	ErrPhotoConsentNotWithdrawn         = care.ErrPhotoConsentNotWithdrawn
	ErrPickupChangeCutoffPassed         = care.ErrPickupChangeCutoffPassed
	ErrPickupChangeDisabled             = care.ErrPickupChangeDisabled
	ErrPollClosed                       = messaging.ErrPollClosed
	ErrRemoveDisabled                   = care.ErrRemoveDisabled
	ErrRequestSharingForbidden          = messaging.ErrRequestSharingForbidden
	ErrRequestSharingInvalid            = messaging.ErrRequestSharingInvalid
	ErrRequestSharingNotFound           = messaging.ErrRequestSharingNotFound
	ErrSickNoteDisabled                 = care.ErrSickNoteDisabled
)
