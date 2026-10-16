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
	Attendance                      = care.Attendance
	AttendanceFilter                = care.AttendanceFilter
	AbsenceReport                   = care.AbsenceReport
	AbsenceNotifier                 = care.AbsenceNotifier
	StaffParentMessageReport        = care.StaffParentMessageReport
	StaffParentMessageNotifier      = care.StaffParentMessageNotifier
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
	ErrCareExceptionNotCareDay          = care.ErrCareExceptionNotCareDay
	ErrPollClosed                       = messaging.ErrPollClosed
	ErrRemoveDisabled                   = care.ErrRemoveDisabled
	ErrRequestSharingForbidden          = messaging.ErrRequestSharingForbidden
	ErrRequestSharingInvalid            = messaging.ErrRequestSharingInvalid
	ErrRequestSharingNotFound           = messaging.ErrRequestSharingNotFound
	ErrSickNoteDisabled                 = care.ErrSickNoteDisabled
)

// The owner values the child and messaging flows hand back unchanged, under
// the names those flows give them (#3421).
type (
	AllowedDepartureModes            = care.AllowedDepartureModes
	AnnouncementFeedDeclaration      = messaging.AnnouncementFeedDeclaration
	AnnouncementFeedDeclarationChild = messaging.AnnouncementFeedDeclarationChild
	AnnouncementFeedItem             = messaging.AnnouncementFeedItem
	ChildSummary                     = care.ChildSummary
	DeclarationAttachmentDigest      = messaging.DeclarationAttachmentDigest
	DeclarationSignerState           = messaging.DeclarationSignerState
	DeclarationSubmission            = messaging.DeclarationSubmission
	DeclarationVersion               = messaging.DeclarationVersion
	DepartureDays                    = care.DepartureDays
	DepartureMode                    = care.DepartureMode
	EnrollablePhase                  = care.EnrollablePhase
	EnrollmentRequestSummary         = care.EnrollmentRequestSummary
	GuardianSubmitStatus             = care.GuardianSubmitStatus
	InboxThread                      = messaging.InboxThread
	ParentMessage                    = messaging.ParentMessage
	RequestReasonPolicy              = care.RequestReasonPolicy
	SettingsService                  = care.SettingsService
	SettingsSnapshot                 = care.SettingsSnapshot
	BatchSettingsService             = care.BatchSettingsService
	StudentDataChangeRequest         = care.StudentDataChangeRequest
)

const (
	DataChangeStatusPending     = care.DataChangeStatusPending
	DataChangeTargetPerson      = care.DataChangeTargetPerson
	DataChangeTargetStudent     = care.DataChangeTargetStudent
	DeclarationActionAgreed     = messaging.DeclarationActionAgreed
	DeclarationActionDeclined   = messaging.DeclarationActionDeclined
	DeclarationActionRevoked    = messaging.DeclarationActionRevoked
	DeclarationKindConsent      = messaging.DeclarationKindConsent
	DeparturePickup             = care.DeparturePickup
	ParentMessageSenderGuardian = messaging.ParentMessageSenderGuardian
	ParentMessageSenderStaff    = messaging.ParentMessageSenderStaff
)

var (
	CanonicalDayToISOWeekday   = care.CanonicalDayToISOWeekday
	ErrGuardianProfileNotFound = care.ErrGuardianProfileNotFound
)

// Settings keys and policy values consumed by the guardian workflow.
const (
	KeyGuardianParentCanRemove         = care.KeyGuardianParentCanRemove
	KeyGuardianParentInviteMode        = care.KeyGuardianParentInviteMode
	KeyMealPlanEnabled                 = care.KeyMealPlanEnabled
	KeyParentCareArrivalRequestEnabled = care.KeyParentCareArrivalRequestEnabled
	KeyParentCareModeRequestEnabled    = care.KeyParentCareModeRequestEnabled
	KeyParentCarePickupRequestEnabled  = care.KeyParentCarePickupRequestEnabled
	KeyParentExcusedReportsEnabled     = care.KeyParentExcusedReportsEnabled
	KeyParentExcusedRequiresApproval   = care.KeyParentExcusedRequiresApproval
	KeyParentGuardianManagementEnabled = care.KeyParentGuardianManagementEnabled
	KeyParentMasterDataEditEnabled     = care.KeyParentMasterDataEditEnabled
	KeyParentMasterDataRequestEnabled  = care.KeyParentMasterDataRequestEnabled
	KeyParentNotesEnabled              = care.KeyParentNotesEnabled
	KeyParentPickupChangeCutoffTime    = care.KeyParentPickupChangeCutoffTime
	KeyParentPickupChangeEnabled       = care.KeyParentPickupChangeEnabled
	KeyParentRequestReasonPolicy       = care.KeyParentRequestReasonPolicy
	KeyParentSickNoteEnabled           = care.KeyParentSickNoteEnabled
	KeyParentSickReportsEnabled        = care.KeyParentSickReportsEnabled
	KeyParentSickRequiresApproval      = care.KeyParentSickRequiresApproval
	ParentInviteModeDirect             = care.ParentInviteModeDirect
	ParentInviteModeDisabled           = care.ParentInviteModeDisabled
	ParentInviteModeStaffApproval      = care.ParentInviteModeStaffApproval
	ReasonPolicyBoth                   = care.ReasonPolicyBoth
	ReasonPolicyGuardians              = care.ReasonPolicyGuardians
	ReasonPolicyNobody                 = care.ReasonPolicyNobody
	ReasonPolicyStaff                  = care.ReasonPolicyStaff
)
