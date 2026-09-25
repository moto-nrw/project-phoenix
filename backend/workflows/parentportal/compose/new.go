// Package compose binds the guardian portal workflow to the owners it
// coordinates (#3420): the retained read models of the child, its school and
// its guardians, the owners' commands, and the platform's live events, meal
// plan and language catalog. It holds no database: every flow runs in the
// tenant unit of work of the request context.
package compose

import (
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	auditcompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplancompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

// CarePeriod and OfferingBooking are the Enrollment values the root binds
// behind CarePeriods and OfferingHistory.
type (
	CarePeriod      = care.CarePeriod
	OfferingBooking = care.OfferingBooking
)

// Dependencies are the collaborators of the guardian portal. Optional
// notifiers may be nil; tests set only what their flows reach.
type Dependencies struct {
	Logger *slog.Logger
	Now    func() time.Time

	ChildRepo             parentModels.ChildRepository
	EnrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	EnrollmentSettings    care.EnrollmentSettings
	EnrollmentRequestRepo parentModels.EnrollmentRequestRepository
	Settings              configService.SettingsService

	StudentRepo         usersModels.StudentRepository
	PersonRepo          usersModels.PersonRepository
	GuardianProfileRepo usersModels.GuardianProfileRepository
	GuardianPhoneRepo   usersModels.GuardianPhoneNumberRepository
	StudentGuardianRepo usersModels.StudentGuardianRepository
	ChangeRequestRepo   usersModels.StudentDataChangeRequestRepository
	StatusDayRepo       care.StatusDayReads
	Attendance          care.AttendanceReader

	ArrivalSchedules careplan.ArrivalScheduleService
	PickupSchedules  careplan.PickupScheduleService
	CareRequests     carerequests.Service
	ExcusedRequests  careplan.ExcusedAbsenceRequests
	// CarePlan is the Care Plan capability: the day exceptions, the status
	// days and the Stammdaten requests the flows read and command.
	CarePlan careplan.Capability
	// CareExceptions overrides the day-exception reads and locks; nil uses
	// CarePlan.
	CareExceptions    care.CareExceptions
	PickupAutoExcusal careplan.PickupAutoExcusal

	CarePeriods      care.CarePeriodReads
	OfferingHistory  care.OfferingHistoryReads
	CareOfferingRepo enrollmentModels.CareOfferingRepository
	OfferingChanges  care.OfferingChangeRequests

	// CareProfiles writes the child's care profile (health information, live
	// absence flags).
	CareProfiles careplan.StudentProfileCommands
	// People is People Directory: the guardian rows and the child's photo
	// consent the flows write.
	People PeopleDirectory

	StudentAudit        usersSvc.StudentChangeRecorder
	StudentConsents     care.StudentConsentService
	StudentPhotos       func() care.StudentPhotoUnlinker
	ParentRequestEvents usersSvc.ParentRequestEventRecorder
	GuardianInvites     care.GuardianAccess
	GuardianInvitations care.GuardianInvitationReads

	// Messaging: the parent-OGS conversation, announcements, and the reads the
	// request-sharing ledger joins.
	MessageThreadRepo         usersModels.ParentMessageThreadRepository
	MessageRepo               usersModels.ParentMessageRepository
	MessageReadRepo           usersModels.ParentMessageReadRepository
	Conversations             messaging.ConversationCore
	AnnouncementRepo          usersModels.ParentAnnouncementRepository
	CareRequestRepo           scheduleModels.CareScheduleChangeRequestRepository
	ExcusedRequestRepo        messaging.ExcusedRequestReads
	OfferingChangeRequestRepo enrollmentModels.OfferingChangeRequestRepository
	FamilyProtectionEvents    usersModels.FamilyProtectionEventRepository
	ParentRequestShares       usersModels.ParentRequestShareEventRepository

	// Platform: the meal plan, live events and notifiers.
	MealPlan              MealPlanProvider
	Broadcaster           realtime.Broadcaster
	AbsenceNotifier       notificationsSvc.AbsenceNotifier
	ParentMessageNotifier notificationsSvc.StaffParentMessageNotifier
	Emitter               *parentmessaging.Emitter
}

// New composes the guardian portal. messaging resolves children through the
// child flows and the child flows share requests through messaging, so the
// resolver reaches the child flows once both exist.
func New(deps Dependencies) *parentportal.Portal {
	var childFlows *care.Service
	messagingFlows := messaging.New(messagingConfig(deps, children{flows: &childFlows}))
	childFlows = care.New(careConfig(deps, messagingFlows))
	return &parentportal.Portal{ChildFlows: childFlows, MessagingFlows: messagingFlows}
}

func careConfig(deps Dependencies, messagingFlows *messaging.Service) care.Config {
	bound := bindOwners(deps)
	cfg := care.Config{
		CareExceptions: bound.careExceptions, DataRequests: bound.dataRequests,
		GuardianAbsences: bound.guardianAbsences, GuardianPickups: bound.guardianPickups,
		Guardians: bound.guardians, Students: bound.students,
		Logger: deps.Logger, Now: deps.Now,
		ChildRepo: deps.ChildRepo, EnrollablePhaseRepo: deps.EnrollablePhaseRepo,
		EnrollmentSettings: deps.EnrollmentSettings, EnrollmentRequestRepo: deps.EnrollmentRequestRepo,
		StudentRepo: deps.StudentRepo, PersonRepo: deps.PersonRepo, GuardianProfileRepo: deps.GuardianProfileRepo,
		GuardianPhoneRepo: deps.GuardianPhoneRepo, StudentGuardianRepo: deps.StudentGuardianRepo,
		ChangeRequestRepo: deps.ChangeRequestRepo, StatusDayRepo: deps.StatusDayRepo, Settings: deps.Settings,
		Attendance:       deps.Attendance,
		ArrivalSchedules: deps.ArrivalSchedules, PickupSchedules: deps.PickupSchedules, CareRequests: deps.CareRequests,
		ExcusedRequests: deps.ExcusedRequests, PickupAutoExcusal: deps.PickupAutoExcusal,
		CarePeriods: deps.CarePeriods, OfferingHistory: deps.OfferingHistory,
		CareOfferingRepo: deps.CareOfferingRepo, OfferingChanges: deps.OfferingChanges,
		StudentAudit: deps.StudentAudit, StudentConsents: deps.StudentConsents, StudentPhotos: deps.StudentPhotos,
		ParentRequestEvents: deps.ParentRequestEvents,
		GuardianChanges:     guardianChanges{log: auditcompose.NewGuardianChangeLog()},
		GuardianInvites:     deps.GuardianInvites, GuardianInvitations: deps.GuardianInvitations,
		Locales:         locales{},
		AbsenceNotifier: deps.AbsenceNotifier, Emitter: deps.Emitter,
		SelfService:    messagingFlows,
		RequestSharing: requestSharing{messaging: messagingFlows},
	}
	if deps.MealPlan != nil {
		cfg.MealPlan = mealPlan{provider: deps.MealPlan}
	}
	if deps.Broadcaster != nil {
		cfg.StudentUpdates = studentUpdates{broadcaster: deps.Broadcaster}
	}
	return cfg
}

func messagingConfig(deps Dependencies, resolver messaging.ChildResolver) messaging.Config {
	return messaging.Config{
		Logger: deps.Logger, Now: deps.Now, Children: resolver,
		ChildRepo: deps.ChildRepo, EnrollmentRequestRepo: deps.EnrollmentRequestRepo, Settings: deps.Settings,
		StudentRepo: deps.StudentRepo, StudentGuardianRepo: deps.StudentGuardianRepo, GuardianProfileRepo: deps.GuardianProfileRepo,
		AnnouncementRepo:  deps.AnnouncementRepo,
		MessageThreadRepo: deps.MessageThreadRepo, MessageRepo: deps.MessageRepo, MessageReadRepo: deps.MessageReadRepo,
		Conversations: deps.Conversations, ParentMessageNotifier: deps.ParentMessageNotifier, Emitter: deps.Emitter,
		ChangeRequestRepo: deps.ChangeRequestRepo, CareRequestRepo: deps.CareRequestRepo,
		ExcusedRequestRepo: deps.ExcusedRequestRepo, OfferingChangeRequestRepo: deps.OfferingChangeRequestRepo,
		FamilyProtectionEvents: deps.FamilyProtectionEvents, ParentRequestShares: deps.ParentRequestShares,
		ParentRequestEvents: deps.ParentRequestEvents,
	}
}

// owners are the owners' commands the flows write through. Care Plan
// constructs its guardian commands over its own capability; the pickup
// excusal stays optional, as in the flows.
type owners struct {
	careExceptions   care.CareExceptions
	dataRequests     care.StudentDataRequestCommands
	guardianAbsences careplan.GuardianAbsenceReports
	guardianPickups  careplan.GuardianPickupExceptions
	guardians        care.GuardianRecords
	students         care.StudentRecords
}

func bindOwners(deps Dependencies) owners {
	var bound owners
	if deps.CarePlan != nil {
		bound = owners{
			careExceptions:   deps.CarePlan,
			dataRequests:     deps.CarePlan,
			guardianAbsences: mustOwner(careplancompose.NewGuardianAbsences(deps.CarePlan)),
			guardianPickups:  mustOwner(careplancompose.NewGuardianPickupExceptions(deps.CarePlan, deps.PickupAutoExcusal)),
		}
	}
	if deps.CareExceptions != nil {
		bound.careExceptions = deps.CareExceptions
	}
	if deps.People != nil {
		bound.guardians = guardianRecords{people: deps.People}
	}
	if deps.People != nil || deps.CareProfiles != nil {
		bound.students = studentRecords{care: deps.CareProfiles, people: deps.People}
	}
	return bound
}

// mustOwner unwraps an owner constructor whose only failure is a missing
// dependency, which bindOwners has already ruled out.
func mustOwner[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
