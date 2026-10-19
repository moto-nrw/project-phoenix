package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// EnrollmentNotices are Enrollment's mail branding and decision notices over
// the Delivery outbox, and the admin deletion with the retention cleanup of
// rejected enrollments, which share one impact preview (#3564).
type EnrollmentNotices struct {
	Notifications   enrollmentOwner.Notifications
	Deletions       enrollmentOwner.EnrollmentDeletions
	RejectedCleanup enrollmentOwner.RejectedEnrollmentCleaner
	CareOfferings   *enrollmentCompose.CareOfferingRecords
}

// BuildEnrollmentNotices binds Enrollment's notices, its deletions and Care
// Plan's offerings in the enrollment rows the decision flow, the intake, the
// change requests, the capacity gate and the reports read.
func BuildEnrollmentNotices(
	env RootEnvironment, enrollment EnrollmentRows, settings settingsCompose.BatchSettingsService, outbox MailOutbox,
	organizations organizationtenancy.Query, audit AuditOwner, carePlan careplan.Capability, module *deliveryModule.Module,
) EnrollmentNotices {
	deletions := NewEnrollmentDeletionModule(EnrollmentDeletionSources{
		Owner: enrollment.Bookings, Guardians: enrollment.Guardians,
		CountAuditAdjustments: audit.Persistence.EnrollmentOfferingAdjustment.CountForDeletion,
		CountBookings:         carePlan.CountCareOfferingBookings, Audit: audit.Persistence.EnrollmentDeletionAudit,
		Delivery: enrollmentDeliveryAdapter{module: module}, Settings: settings, Logger: env.Logger,
	})
	return EnrollmentNotices{
		Notifications:   NewEnrollmentNotifications(enrollment.Bookings, settings, outbox, EnrollmentSchoolDirectory{Schools: organizations}),
		Deletions:       deletions.Deletions,
		RejectedCleanup: deletions.Cleanup,
		CareOfferings:   enrollmentCompose.NewCareOfferingRecords(carePlan),
	}
}

// BuildEnrollmentPhases binds the phase administration. A phase
// service-window change re-bounds every roster row derived from the phase's
// offerings through the booking materialization (#2147 review).
func BuildEnrollmentPhases(
	env RootEnvironment, enrollment EnrollmentRows, carePlan careplan.Capability, persons peopledirectory.Capability,
	calendar enrollmentCompose.CalendarPeriods, owner TimetableOwner, offerings CarePlanOfferings, settings settingsCompose.BatchSettingsService,
) (*enrollmentCompose.Phases, error) {
	phases, err := enrollmentCompose.NewResyncingPhases(enrollmentCompose.PhaseDependencies{
		Records: enrollment.Bookings, Offerings: enrollmentPhaseOfferings{carePlan: carePlan}, Calendar: calendar,
		LockTemplateRecurrence: owner.RecurrenceLock.LockRecurrenceWrites, ValidateCareOfferingPhaseChange: careOfferingPhaseGuard(offerings.Catalog),
		Resyncer: offerings.Bookings, Settings: EnrollmentCollectionSettings{Settings: settings},
		Roster: phaseResponseRoster{query: persons}, CareExits: phaseResponseCareExits{query: carePlan},
		PortalAccounts: persons, Logger: env.Logger.With("service", "enrollment-phase"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose enrollment phases: %w", err)
	}
	return phases, nil
}

// BuildEnrollmentDecisions binds Enrollment's decision flow over the owners
// and the retained rows.
func BuildEnrollmentDecisions(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, enrollment EnrollmentRows, notices EnrollmentNotices,
	audit AuditOwner, people PeopleRows, careRows TimetableCareRows, persons peopledirectory.Capability, carePlan careplan.Capability,
	offerings CarePlanOfferings, identity *identityaccess.Module, services PeopleServices, schedules CarePlanSchedules,
	carePlanRows CarePlanRows, owner TimetableOwner, settings settingsCompose.BatchSettingsService,
) *enrollmentCompose.Decisions {
	records := enrollment.Bookings
	return NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
		Requests: records, Children: records, Guardians: records, LateInvites: records, Phases: records, Schemas: records,
		Offerings: notices.CareOfferings, Notifications: notices.Notifications,
		AccessLog:    EnrollmentDecisionAccessLog(audit.Persistence.DataAccessLog),
		Adjustments:  EnrollmentDecisionAdjustments(audit.Persistence.EnrollmentOfferingAdjustment),
		Restorations: EnrollmentDecisionRestorations(audit.Persistence.EnrollmentRestorationAudit),
		Payers:       EnrollmentDecisionPayers(audit.Persistence.GuardianFinancialChange),
		People: EnrollmentDecisionPeopleDirectory(
			people.Persons, people.Staff, people.Students, people.StudentGuardians, people.GuardianProfiles,
			peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()), persons,
		),
		StudentEnrollment: persons,
		Companions:        EnrollmentDecisionCompanions(NewStudentCompanionRepository(carePlan), carePlan.DeleteCompanionEdges),
		PickupSchedules:   arrivalTimetable.NewEnrollmentPickupSchedules(careRows.PickupSchedules),
		ArrivalSchedules:  arrivalTimetable.NewEnrollmentArrivalSchedules(careRows.ArrivalSchedules),
		Bookings:          offerings.Bookings, GuardianAccess: identity,
		StudentAudit:           EnrollmentDecisionStudentAudit(services.StudentAudit),
		StudentConsents:        EnrollmentDecisionStudentConsents(services.StudentConsents),
		CareWithdrawal:         schedules.Lifecycle,
		Broadcasts:             EnrollmentDecisionBroadcasts(hub),
		ParentsURL:             EnrollmentDecisionParentsURL(env.Links.ParentsURL, env.Links.FrontendURL),
		Settings:               EnrollmentDecisionSettingValues(settings),
		LockTemplateRecurrence: owner.RecurrenceLock.LockRecurrenceWrites,
		Pickups:                weeklyPickupHooks(schedules.AutoExcusal, carePlanRows.PickupStudentLocks),
		Logger:                 env.Logger.With("service", "enrollment-decision"), Today: env.Today,
	})
}

// weeklyPickupHooks bind an approved weekly Gehzeit plan to Care Plan: the
// student locks in Care Plan's lock order before its weekly rows (#2360
// review), the auto-excusal resync and the change history.
func weeklyPickupHooks(excusal careplan.PickupAutoExcusal, locks carePlanCompose.PickupStudentLocks) enrollmentCompose.WeeklyPickupHooks {
	return enrollmentCompose.WeeklyPickupHooks{
		LockStudents:  carePlanCompose.LockWeeklyPickupStudents(locks),
		ResyncExcusal: excusal.ResyncFutureExceptions,
		Snapshot: func(ctx context.Context, studentID int64, date timezone.Date) (map[int]string, error) {
			return excusal.SnapshotWeeklyPickups(ctx, studentID, date)
		},
		RecordChanges: func(ctx context.Context, studentID int64, date timezone.Date, before map[int]string) error {
			return excusal.RecordWeeklyPickupChanges(ctx, studentID, date, careplan.WeeklyPickupSnapshot(before))
		},
	}
}

// EnrollmentFlows are the intake with its opt-in "Neue Anmeldung" mail
// (#3780), the capacity gate, the reports, the change requests and the
// rollover, which approves auto-renewed rows through the decision flow on
// the rollover_auto_approve=true deadline path.
type EnrollmentFlows struct {
	Capacity       enrollmentOwner.OfferingCapacity
	Intake         *enrollmentCompose.Intake
	Reports        enrollmentOwner.Reports
	ChangeRequests enrollmentOwner.ChangeRequests
	Rollovers      enrollmentOwner.Rollovers
}

// BuildEnrollmentFlows binds the intake, the reports, the change requests and
// the rollover over the decision flow.
func BuildEnrollmentFlows(
	db transactionruntime.Database, env RootEnvironment, enrollment EnrollmentRows, notices EnrollmentNotices,
	decisions *enrollmentCompose.Decisions, phases *enrollmentCompose.Phases, offerings CarePlanOfferings,
	consent communication.NotificationConsentCapability, identity *identityaccess.Module, people PeopleRows,
	structure StructureRows, carePlan careplan.Capability, carePlanRows CarePlanRows, schedules CarePlanSchedules,
	membership schoolmembership.Capability, organizations organizationtenancy.Query, audit AuditOwner,
	settings settingsCompose.BatchSettingsService, outbox MailOutbox,
) EnrollmentFlows {
	records, links := enrollment.Bookings, env.Links
	capacity := NewEnrollmentOfferingCapacity(notices.CareOfferings, records, settings)
	return EnrollmentFlows{
		Capacity: capacity,
		Intake: NewEnrollmentIntake(EnrollmentIntakeSources{
			Requests: records, ParentChanges: records, Children: records, Bookings: enrollment.CareBookings,
			Guardians: records, LateInviteRepo: records, CareOfferingRepo: notices.CareOfferings,
			Capacity: capacity, Catalog: records, SchoolRepo: EnrollmentSchoolDirectory{Schools: organizations},
			Notifications: notices.Notifications, StudentRepo: people.Students, GuardianAuthorizer: people.StudentGuardians,
			RateLimitRepo: records, OutboxEnqueuer: intakeMailOutbox(outbox), Settings: settings,
			AdminSubscribers: NewEnrollmentAdminSubscribers(consent, identity), ManualDecider: decisions,
			FrontendURL: links.FrontendURL, ParentsURL: links.ParentsURL, Logger: env.Logger.With("service", "enrollment-request"),
		}),
		Reports: NewEnrollmentReports(EnrollmentReportSources{
			Owner: records, Offerings: notices.CareOfferings, AccessLog: audit.Persistence.DataAccessLog,
			Students: people.Students, Persons: people.Persons, Groups: education.NewGroupNames(structure.Groups),
			StudentGuardians: people.StudentGuardians, Companions: NewStudentCompanionRepository(carePlan),
			ClassListEntries: NewClassListEntryRosterReader(membership), PickupSchedules: schedules.Pickups,
			CareParticipation: schedules.Lifecycle, Settings: settings,
		}),
		ChangeRequests: NewEnrollmentChangeRequests(EnrollmentChangeRequestSources{
			Bookings: enrollment.CareBookings, Requests: records, Children: records, Guardians: records,
			LateInviteRepo: records, CareOfferingRepo: notices.CareOfferings, Capacity: capacity,
			Catalog: records, Notifications: notices.Notifications, GuardianProfileRepo: people.GuardianProfiles,
			GuardianPhoneRepo: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()), GuardianInvitations: enrollmentGuardianInvitationAvailability{invitations: identity},
			PersonRepo: people.Persons, StudentRepo: people.Students, GuardianAuthorizer: people.StudentGuardians,
			Decisions: decisions, BookingGates: offerings.Bookings,
			CompanionGraphLocker: companionGraphCoordinator{CompanionLocks: carePlanRows.Companions, strandings: people.Students},
			Settings:             settings, OutboxEnqueuer: intakeMailOutbox(outbox), FrontendURL: links.FrontendURL, ParentsURL: links.ParentsURL,
			Logger: env.Logger.With("service", "enrollment-change-request"),
		}),
		Rollovers: NewEnrollmentRollovers(EnrollmentRolloverSources{
			Bookings: enrollment.CareBookings, Phases: records, Requests: records, Children: records,
			Catalog: offerings.Catalog, Notifications: notices.Notifications, PhaseEligibility: phases,
			Outbox: intakeMailOutbox(outbox), Settings: settings, Decisions: decisions, ParentsURL: links.ParentsURL,
			Logger: env.Logger.With("service", "enrollment-rollover"),
		}),
	}
}
