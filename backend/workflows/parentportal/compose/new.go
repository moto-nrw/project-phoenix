// Package compose binds the guardian portal workflow to the owners it
// coordinates (#3420), including its read, command and pure policy ports.
// It holds no database; every flow joins the request's tenant unit of work.
package compose

import (
	"cmp"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

// Dependencies are the collaborators of the guardian portal. Runtime is
// required; optional notifiers may be nil. Tests bind the ports their flows reach.
type Dependencies struct {
	GuardianChanges auditlog.GuardianChangeLog
	Signatures      parentportal.Signatures
	CarePlanPolicy  care.CarePlanPolicy
	GuardianPolicy  care.GuardianPolicy
	DeparturePolicy care.DeparturePolicy
	Fingerprints    messaging.ContentFingerprints
	Declarations    messaging.DeclarationPolicy
	Locales         care.Locales
	Runtime         care.Runtime
	Logger          *slog.Logger
	Now             func() time.Time

	ChildRepo             care.ChildReads
	EnrollablePhaseRepo   care.EnrollablePhaseReads
	EnrollmentSettings    care.EnrollmentSettings
	EnrollmentRequestRepo care.EnrollmentRequestReads
	Settings              care.SettingsService

	StudentRepo         care.StudentReads
	PersonRepo          care.PersonReads
	GuardianProfileRepo messaging.GuardianProfiles
	GuardianPhoneRepo   care.GuardianPhoneReads
	StudentGuardianRepo care.StudentGuardianReads
	ChangeRequestRepo   care.DataRequestReads
	DataRequests        care.StudentDataRequestCommands
	StatusDayRepo       care.StatusDayReads
	Attendance          care.AttendanceReader

	ArrivalSchedules care.ArrivalSchedules
	PickupSchedules  care.PickupSchedules
	CareRequests     carerequests.Service
	ExcusedRequests  care.ExcusedAbsenceRequests
	// CarePlan supplies native day locks, absence reports and pickup commands.
	CarePlan parentportal.CarePlanCommands
	// CareExceptions overrides the day-exception reads and locks (nil: CarePlan).
	CareExceptions care.CareExceptions

	CarePeriods      care.CarePeriodReads
	OfferingHistory  care.OfferingHistoryReads
	CareOfferingRepo care.CareOfferingReads
	OfferingChanges  care.OfferingChangeRequests

	// CareProfiles writes the child's care profile (health information, live
	// absence flags).
	CareProfiles careplan.StudentProfileCommands
	// People is People Directory: the guardian rows and the child's photo
	// consent the flows write.
	People PeopleDirectory

	StudentAudit        care.StudentChangeRecorder
	StudentConsents     care.StudentConsentService
	StudentPhotos       care.StudentPhotoUnlinker
	ChildDataAccess     care.ChildDataAccessLog
	ParentRequestEvents messaging.RequestEvents

	// Document inbox (#4671, ADR 0126).
	BillingDocuments    care.BillingDocuments
	TraegerRuntime      care.TraegerRuntime
	SchoolTraeger       care.SchoolTraeger
	ParentAccounts      care.ParentAccounts
	DocumentNotifier    care.BillingDocumentNotifier
	GuardianInvites     care.GuardianAccess
	GuardianInvitations care.GuardianInvitationReads

	// Messaging: conversation, announcements, surveys, request-sharing reads.
	MessageThreadRepo         messaging.MessageThreadAccess
	MessageRepo               messaging.MessageReads
	MessageReadRepo           messaging.MessageInboxReads
	Conversations             messaging.ConversationCore
	MessagingEnabled          messaging.MessagingGate
	GroupThreads              messaging.GroupThreads
	Surveys                   messaging.Surveys
	AnnouncementRepo          messaging.AnnouncementAccess
	CareRequestRepo           messaging.CareRequestReads
	ExcusedRequestRepo        messaging.ExcusedRequestReads
	OfferingChangeRequestRepo messaging.OfferingChangeRequestReads
	FamilyProtectionEvents    messaging.FamilyProtectionReads
	ParentRequestShares       messaging.RequestShareRecords

	// Platform: the meal plan, live events and notifiers.
	MealPlan              MealPlanProvider
	StudentUpdates        care.StudentUpdates
	AbsenceNotifier       care.AbsenceNotifier
	ParentMessageNotifier care.StaffParentMessageNotifier
	Emitter               care.ChildEvents
}

// New composes the guardian portal. messaging resolves children through the
// child flows and the child flows share requests through messaging, so the
// resolver reaches the child flows once both exist.
func New(deps Dependencies) *parentportal.Portal {
	var childFlows *care.Service
	messagingFlows := parentportal.NewMessagingFlows(messagingConfig(deps, messaging.NewChildResolver(&childFlows))).MessagingFlows
	childFlows = parentportal.NewChildFlows(careConfig(deps, bindOwners(deps), messagingFlows))
	return &parentportal.Portal{ChildFlows: childFlows, MessagingFlows: messagingFlows}
}

func careConfig(deps Dependencies, bound owners, messagingFlows *messaging.Service) care.Config {
	cfg := care.Config{
		CareExceptions: bound.careExceptions, DataRequests: deps.DataRequests,
		GuardianAbsences: bound.guardianAbsences, GuardianPickups: bound.guardianPickups,
		Guardians: bound.guardians, Students: bound.students,
		Logger: deps.Logger, Now: deps.Now, Runtime: deps.Runtime,
		ChildRepo: deps.ChildRepo, EnrollablePhaseRepo: deps.EnrollablePhaseRepo,
		EnrollmentSettings: deps.EnrollmentSettings, EnrollmentRequestRepo: deps.EnrollmentRequestRepo,
		StudentRepo: deps.StudentRepo, PersonRepo: deps.PersonRepo, GuardianProfileRepo: deps.GuardianProfileRepo,
		GuardianPhoneRepo: deps.GuardianPhoneRepo, StudentGuardianRepo: deps.StudentGuardianRepo,
		ChangeRequestRepo: deps.ChangeRequestRepo, StatusDayRepo: deps.StatusDayRepo, Settings: deps.Settings,
		Attendance:       deps.Attendance,
		ArrivalSchedules: deps.ArrivalSchedules, PickupSchedules: deps.PickupSchedules, CareRequests: deps.CareRequests,
		ExcusedRequests: deps.ExcusedRequests, CarePlanPolicy: deps.CarePlanPolicy,
		CarePeriods: deps.CarePeriods, OfferingHistory: deps.OfferingHistory,
		CareOfferingRepo: deps.CareOfferingRepo, OfferingChanges: deps.OfferingChanges,
		StudentAudit: deps.StudentAudit, StudentConsents: deps.StudentConsents, StudentPhotos: deps.StudentPhotos, ChildDataAccess: deps.ChildDataAccess,
		ParentRequestEvents: deps.ParentRequestEvents,
		BillingDocuments:    deps.BillingDocuments, TraegerRuntime: deps.TraegerRuntime, SchoolTraeger: deps.SchoolTraeger,
		ParentAccounts: deps.ParentAccounts, DocumentNotifier: deps.DocumentNotifier,
		GuardianChanges: newGuardianChanges(deps.GuardianChanges), PickupSignatures: deps.Signatures,
		GuardianInvites: deps.GuardianInvites, GuardianInvitations: deps.GuardianInvitations,
		Locales: deps.Locales, StudentUpdates: deps.StudentUpdates, GuardianPolicy: deps.GuardianPolicy, DeparturePolicy: deps.DeparturePolicy,
		AbsenceNotifier: deps.AbsenceNotifier, Emitter: deps.Emitter,
		SelfService:    messagingFlows,
		RequestSharing: messagingFlows.ChildFlowSharing(),
	}
	if deps.MealPlan != nil {
		cfg.MealPlan = newMealPlan(deps.MealPlan)
	}
	return cfg
}

func messagingConfig(deps Dependencies, resolver messaging.ChildResolver) messaging.Config {
	return messaging.Config{
		SharePolicy: NewRequestSharePolicy(), Logger: deps.Logger, Now: deps.Now, Runtime: deps.Runtime, Children: resolver, GuardianPolicy: deps.GuardianPolicy, Fingerprints: deps.Fingerprints, Declarations: deps.Declarations, Signatures: deps.Signatures,
		ChildRepo: deps.ChildRepo, EnrollmentRequestRepo: deps.EnrollmentRequestRepo, Settings: deps.Settings,
		StudentRepo: deps.StudentRepo, StudentGuardianRepo: deps.StudentGuardianRepo, GuardianProfileRepo: deps.GuardianProfileRepo, AnnouncementRepo: deps.AnnouncementRepo,
		MessageThreadRepo: deps.MessageThreadRepo, MessageRepo: deps.MessageRepo, MessageReadRepo: deps.MessageReadRepo,
		Conversations: deps.Conversations, MessagingEnabled: deps.MessagingEnabled, GroupThreads: deps.GroupThreads, Surveys: deps.Surveys, ParentMessageNotifier: deps.ParentMessageNotifier, Emitter: deps.Emitter,
		ChangeRequestRepo: deps.ChangeRequestRepo, CareRequestRepo: deps.CareRequestRepo,
		ExcusedRequestRepo: deps.ExcusedRequestRepo, OfferingChangeRequestRepo: deps.OfferingChangeRequestRepo,
		FamilyProtectionEvents: deps.FamilyProtectionEvents, ParentRequestShares: deps.ParentRequestShares,
		ParentRequestEvents: deps.ParentRequestEvents,
	}
}

// owners are native command ports, bound to the same owner capabilities at Root.
type owners struct {
	careExceptions   care.CareExceptions
	guardianAbsences care.GuardianAbsenceReports
	guardianPickups  care.GuardianPickupExceptions
	guardians        care.GuardianRecords
	students         care.StudentRecords
}

func bindOwners(deps Dependencies) owners {
	var bound owners
	if deps.CarePlan != nil {
		bound = owners{
			careExceptions:   deps.CarePlan,
			guardianAbsences: deps.CarePlan,
			guardianPickups:  deps.CarePlan,
		}
	}
	if deps.CareExceptions != nil {
		bound.careExceptions = deps.CareExceptions
	}
	if deps.People != nil {
		bound.guardians = newGuardianRecords(deps.People)
	}
	if deps.People != nil || deps.CareProfiles != nil {
		bound.students = newStudentRecords(deps.CareProfiles, deps.People)
	}
	return bound
}

// NewDocumentNotices builds the push of a new document (ADR 0126) for the
// Delivery worker over the children, their Träger, the accounts, the
// guardian's language and the notifier of deps; it needs no other flow.
func NewDocumentNotices(deps Dependencies) parentportal.DocumentNotices {
	if deps.Runtime == nil || deps.ChildRepo == nil || deps.SchoolTraeger == nil || deps.ParentAccounts == nil || deps.DocumentNotifier == nil {
		panic("parent portal: document notices need the runtime, the children, the Träger, the accounts and the notifier")
	}
	return &care.Service{Config: care.Config{Runtime: deps.Runtime, ChildRepo: deps.ChildRepo, SchoolTraeger: deps.SchoolTraeger,
		ParentAccounts: deps.ParentAccounts, GuardianProfileRepo: deps.GuardianProfileRepo, DocumentNotifier: deps.DocumentNotifier,
		Logger: cmp.Or(deps.Logger, slog.Default())}}
}
