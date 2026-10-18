package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// composeDelivery binds the Delivery platform: every outbox kind's renderer
// is known at startup, so the registry is complete before the worker claims
// anything. Enrollment keeps one renderer per kind so subjects and templates
// stay independent; calendar appointments share one renderer for all four
// kinds.
func composeDelivery(b graphBuild) (graphBuild, error) {
	defaultFrom := b.graph.DefaultFrom
	appointments := b.TimetableWiring.AppointmentRenderer(defaultFrom)
	renderers := map[string]deliveryCompose.MailRenderer{
		communication.MailKindParentAnnouncement: deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: communication.MailAddress(defaultFrom),
		}))),
		communication.MailKindParentMessage:           deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: communication.MailAddress(defaultFrom)}, deliveryPolicy.ParentDeliveryPolicy{}))),
		calendarService.EmailKindAppointmentPublished: appointments,
		calendarService.EmailKindAppointmentUpdated:   appointments,
		calendarService.EmailKindAppointmentCancelled: appointments,
		calendarService.EmailKindAppointmentReminder:  appointments,
	}
	for kind, renderer := range b.Enrollment.MailRenderers(defaultFrom) {
		renderers[kind] = renderer
	}
	for kind, renderer := range b.identity.MailRenderers {
		renderers[kind] = renderer
	}
	b.vapid = deliveryCompose.VAPIDConfig{
		PublicKey: strings.TrimSpace(b.Config.VAPIDPublicKey), PrivateKey: strings.TrimSpace(b.Config.VAPIDPrivateKey),
		Subscriber: strings.TrimSpace(b.Config.VAPIDSubscriber),
	}
	if err := b.vapid.Validate(); err != nil {
		return b, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(b.DB.Pool())
	tenants := deliveryCompose.NewTenantTransactions(b.DB.Pool())
	runtime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: b.DB.Pool(), People: b.Platform.GuardianDisplays(b.graph.Guardian),
		Provider: &deliveryCompose.MailPushProvider{
			Registry: deliveryCompose.NewEmailTemplateRegistry(renderers), Mailer: b.mailer, MailIdentity: b.tenantMailIdentity,
			Push: deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{
				Subscriber: b.vapid.Subscriber, PublicKey: b.vapid.PublicKey, PrivateKey: b.vapid.PrivateKey,
			}, deliveryCompose.NewExpiredPushSubscriptionCleaner(tenants, pushSubscriptions)),
			Logger: b.Logger.With("service", "delivery"), Tenants: tenants,
			PushAuthorized: deliveryCompose.NewPushAuthorizationChecker(tenants, pushSubscriptions),
		},
		Observe: func(observation deliveryModule.Observation) {
			b.ObserveDurableDelivery(string(observation.Transport), observation.Template, observation.Operation, observation.Duration, observation.Count, observation.Err)
		},
	})
	if err != nil {
		return b, fmt.Errorf("initialize delivery module: %w", err)
	}
	b.late.delivery = runtime
	b.graph.EmailOutboxWorker = runtime.Worker
	b.graph.Delivery = runtime.Module
	b.late.emailOutbox = deliveryCompose.NewEmailOutbox(deliveryCompose.NewDurableEmails(runtime.Module))
	return b, nil
}

// outbox serves the retained outbox contract of the producers over the
// Delivery enqueue facade.
func (b *graphBuild) outbox() OutboxEnqueuer { return OutboxEnqueuer{Outbox: b.late.emailOutbox} }

// composeCaregivers binds the caregiver capability, the substitutions and
// the presence cleanup.
func composeCaregivers(b graphBuild) (graphBuild, error) {
	people, repos := b.People, b.TimetableWiring.Repositories
	b.graph.CaregiverCapability = peopleCompose.NewCaregiverCapabilityService(peoplePorts.CaregiverCapabilityServiceDependencies{
		RequestActorScope: b.Identity.ActorScope, Identity: people.CaregiverIdentity(b.identityAccess),
		Audit: people.CaregiverCapabilityAudit(b.audits.AuthEvent), PersonRepo: people.Persons, StaffRepo: people.Staff,
		CaregiverBindingLock: people.CaregiverBindingLock, TeacherRepo: people.Teachers, GroupTeacherRepo: people.GroupTeachers,
		GroupSubstitutionRepo:  b.groupSubstitutions,
		GroupSupervisorRepo:    people.SupervisionBlockers(b.groupSupervisions),
		ActivitySupervisorRepo: CaregiverPlannedSupervisionBlockers{Supervisors: b.Timetable},
		RoleAssignments:        b.identityAccess,
	})
	// Terminvertretungen move Betreuungsplan staffing across the Workforce
	// and Timetable line: the shift-plan-sync workflow serves the
	// substitution module's schedule port (#3418).
	schedule, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: b.staffDeviations, Staff: NewPlanStaffDirectory(people.Staff), Broadcaster: b.graph.RealtimeHub,
		Logger:            b.Logger.With("service", "schedule-substitution"),
		ActivityInstances: arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:     arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
	})
	if err != nil {
		return b, err
	}
	b.graph.Substitution = education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: people.Groups, Substitutions: NewEducationHandovers(b.groupSubstitutions, shiftplansync.NewGroupHandovers(b.WorkTime)),
		Persons: people.SubstitutionPersons, Teachers: people.Caregivers,
		Staff: people.StaffChecks, ExternalCaregivers: people.ExternalCaregivers,
		Actors:       SubstitutionActorResolver{Identity: b.callerContext},
		ActiveGroups: b.Presence.SessionRecords, ActiveSupervisors: b.groupSupervisions,
		ActiveSupervisorCreator: b.graph.Active,
		Audit:                   people.SubstitutionAudit(b.audits.SubstitutionChange),
		Runtime:                 education.NewLegacyRepositoryRuntime(b.DB.Pool()), Broadcaster: NewGroupEvents(b.graph.RealtimeHub, b.Logger.With("service", "substitution")),
		Logger: b.Logger.With("service", "substitution"), Schedule: schedule,
		CanSeeAll: b.canSeeAllSubstitutions, Now: b.now,
	})
	_, sessionSupervisors := presenceCompose.SessionRepositories(b.Presence.SessionRecords)
	b.graph.ActiveCleanup = presenceservice.NewPresenceCleanup(
		b.Presence.Presence, sessionSupervisors, b.Platform.DeletionAudit(b.audits.DataDeletion),
		settingsCompose.PresenceSettings(b.settingsService), b.today,
	)
	return b, nil
}

// canSeeAllSubstitutions asks Workforce, which owns
// education.group_substitution, who sees all of them; the Settings Platform
// answers its overview-scope port.
func (b *graphBuild) canSeeAllSubstitutions(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
	return workforceModule.CanSeeAllSubstitutions(ctx, func(ctx context.Context) (bool, error) {
		scope, err := b.settingsService.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
		if err != nil {
			return false, fmt.Errorf("resolve operational overview scope: %w", err)
		}
		return configModels.OverviewOpenToAllStaff(scope), nil
	}, assignmentBound, admin, hasStaff)
}

// composeCareLifecycle binds the Enrollment notifications, deletions and
// phase administration, the care lifecycle and Care Plan's booking
// materialization (#3560).
func composeCareLifecycle(b graphBuild) (graphBuild, error) {
	lock := b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites
	// Enrollment brands its mails and notifies decisions through the Delivery
	// outbox.
	b.enrollmentNotifications = b.Enrollment.Notifications(b.settingsService, b.outbox())
	// The admin deletion and the retention cleanup of rejected enrollments
	// share one impact preview (#3564).
	deletions := b.Enrollment.Deletions(EnrollmentDeletionSources{
		Owner: b.Enrollment.Bookings, Guardians: b.Enrollment.Guardians,
		CountAuditAdjustments: b.audits.EnrollmentOfferingAdjustment.CountForDeletion,
		CountBookings:         b.CarePlan.CountCareOfferingBookings, Audit: b.audits.EnrollmentDeletionAudit,
		Delivery: b.Platform.EnrollmentDeliveries(b.graph.Delivery), Settings: b.settingsService, Logger: b.Logger,
	})
	b.graph.EnrollmentDeletion, b.graph.EnrollmentRejectedCleanup = deletions.Deletions, deletions.Cleanup
	// A phase service-window change re-bounds every roster row derived from
	// the phase's offerings; the phase administration resolves the booking
	// materialization composed below on every edit (#2147 review).
	b.enrollmentPhases = b.Enrollment.Phases(EnrollmentPhaseSources{
		LockTemplateRecurrence:          lock,
		ValidateCareOfferingPhaseChange: b.CarePlanWiring.OfferingPhaseGuard(b.careOfferings),
		SourcedTemplates:                func() enrollmentOwner.SourcedTemplateResyncer { return b.late.phaseSourcedTemplates },
		Settings:                        b.settingsService, Logger: b.Logger.With("service", "enrollment-phase"),
	})
	b.graph.EnrollmentPhase = b.enrollmentPhases
	b.graph.StudentAudit = peopleCompose.NewStudentAuditService(b.Identity.AuditActor, b.People.StudentAudit)
	lifecycle, err := careplanCompose.NewCareLifecycle(careplanCompose.CareLifecycleDependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               b.DB.Pool(), Records: b.CarePlan, Owners: b.CarePlanWiring.LifecycleOwners, LockCareBookingWrites: lock,
		BookingsAuthoritative: b.bookingsAuthoritative, Fingerprint: securityruntime.Fingerprint,
		Logger: b.Logger.With("service", "care_lifecycle"),
	})
	if err != nil {
		return b, fmt.Errorf("compose care lifecycle: %w", err)
	}
	b.graph.CareLifecycle = lifecycle
	b.late.careParticipation = lifecycle
	careplanCompose.WireCareParticipation(b.graph.CareDay, lifecycle)
	return composeCareBookings(b)
}

// composeCareBookings binds Care Plan's booking materialization (#3560): the
// rosters an approval derives, the offering-sourced Regeltermine, the dated
// offering adjustments and the offering pickup times. Offering-sourced
// weekly Gehzeit changes re-derive the auto excusals of the students'
// future day exceptions (#2360) in Care Plan's own unit of work.
func composeCareBookings(b graphBuild) (graphBuild, error) {
	bookings, err := b.CarePlanWiring.BookingMaterialization(BookingMaterializationSources{
		Catalog: b.careOfferings, Rosters: b.rosterReconciler, Enrollment: b.Enrollment.Bookings,
		Approved: b.Enrollment.ApprovedOfferings, Settings: b.settingsService, Withdrawals: b.graph.CareLifecycle,
		Adjustments: b.audits.EnrollmentOfferingAdjustment, Accounts: b.identityAccess, Pickup: b.pickupBaselines,
		LockRecurrence:           b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites,
		ResyncPickupAutoExcusals: b.pickupAutoExcusal.ResyncFutureExceptions,
		Broadcaster:              b.graph.RealtimeHub, GuardianNotifier: b.graph.ParentEventEmitter,
		Today: b.today, Logger: b.Logger.With("service", "care-plan-bookings"),
	})
	if err != nil {
		return b, fmt.Errorf("compose care plan booking materialization: %w", err)
	}
	b.late.careBookings = bookings
	return b, nil
}

// composeEnrollmentDecisions binds Enrollment's decision flow and the grade
// transition workflow (#2711), which re-reconciles the offering-sourced
// templates' Jahrgang-filtered rosters (#2137) through the booking
// materialization.
func composeEnrollmentDecisions(b graphBuild) (graphBuild, error) {
	// Care Plan's offerings in the enrollment rows the decision flow, the
	// intake, the change requests, the capacity gate and the reports read.
	b.enrollmentCareOfferings = enrollmentCompose.NewCareOfferingRecords(b.CarePlan)
	b.enrollmentDecisions = b.Enrollment.Decisions(b.enrollmentDecisionSources())
	b.graph.EnrollmentDecision = enrollmentCompose.PublicDecisions(b.enrollmentDecisions)
	structure, presence, err := NewGradeTransitionOwners(b.DB)
	if err != nil {
		return b, fmt.Errorf("compose grade transition owners: %w", err)
	}
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: b.DB.Pool(), Structure: structure, Presence: presence, Directory: b.Persons, Membership: b.Membership, Rosters: b.rosterReconciler,
		LockRecurrenceWrites:  b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites,
		ResyncOfferingRosters: b.late.careBookings.ResyncOfferingSourcedTemplates,
		Logger:                b.Logger.With("workflow", "grade_transition"), Audit: b.graph.Audit, ClassListEntryRecord: GradeTransitionClassListRecord,
		Clock: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose grade transition workflow: %w", err)
	}
	b.graph.GradeTransition = workflow
	// A care-offering edit changes the wanted roster of every template
	// sourcing it; a phase service-window change re-bounds the rosters
	// derived from its offerings (#2147 review).
	b.late.phaseSourcedTemplates = b.late.careBookings
	return b, nil
}

// enrollmentDecisionSources are the owners and retained rows the decision
// flow is bound over.
func (b *graphBuild) enrollmentDecisionSources() EnrollmentDecisionSources {
	people, records := b.People, b.Enrollment.Bookings
	return EnrollmentDecisionSources{
		Requests: records, Children: records, Guardians: records, LateInvites: records, Phases: records, Schemas: records,
		CareOfferings: b.enrollmentCareOfferings, DataAccessLog: b.audits.DataAccessLog,
		OfferingAdjustments: b.audits.EnrollmentOfferingAdjustment, Restorations: b.audits.EnrollmentRestorationAudit,
		Notifications: b.enrollmentNotifications, Persons: people.Persons, Staff: people.Staff, Students: people.Students,
		StudentEnrollment: b.Persons, Companions: NewStudentCompanionRepository(b.CarePlan), DeleteCompanions: b.CarePlan.DeleteCompanionEdges,
		StudentGuardians: people.StudentGuardians, GuardianFinancialAudit: b.audits.GuardianFinancialChange,
		GuardianProfiles: people.GuardianProfiles, GuardianPhones: peopleCompose.NewGuardianPhoneNumberRepository(b.DB.Pool()),
		PickupSchedules:  arrivalTimetable.NewEnrollmentPickupSchedules(people.PickupSchedules),
		ArrivalSchedules: arrivalTimetable.NewEnrollmentArrivalSchedules(people.ArrivalSchedules),
		CareBookings:     b.late.careBookings, GuardianAccess: b.identityAccess, StudentAudit: b.graph.StudentAudit,
		StudentConsents: b.graph.StudentConsents, CareWithdrawal: b.graph.CareLifecycle, Broadcaster: b.graph.RealtimeHub,
		FrontendURL: b.frontendURL, ParentsURL: b.parentsURL, Settings: b.settingsService,
		LockTemplateRecurrence: b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites,
		Pickups:                b.weeklyPickupHooks(),
		Logger:                 b.Logger.With("service", "enrollment-decision"), Today: b.today,
	}
}

// weeklyPickupHooks bind an approved weekly Gehzeit plan to Care Plan: the
// student locks in Care Plan's lock order before its weekly rows (#2360
// review), the auto-excusal resync and the change history.
func (b *graphBuild) weeklyPickupHooks() enrollmentCompose.WeeklyPickupHooks {
	excusal := b.pickupAutoExcusal
	return enrollmentCompose.WeeklyPickupHooks{
		LockStudents:  careplanCompose.LockWeeklyPickupStudents(b.CarePlanWiring.PickupStudentLocks),
		ResyncExcusal: excusal.ResyncFutureExceptions,
		Snapshot: func(ctx context.Context, studentID int64, date timezone.Date) (map[int]string, error) {
			return excusal.SnapshotWeeklyPickups(ctx, studentID, date)
		},
		RecordChanges: func(ctx context.Context, studentID int64, date timezone.Date, before map[int]string) error {
			return excusal.RecordWeeklyPickupChanges(ctx, studentID, date, careplan.WeeklyPickupSnapshot(before))
		},
	}
}

// composeEnrollmentRequests binds the capacity gate, the intake with its
// opt-in "Neue Anmeldung" mail (#3780), the reports and the school portal's
// class-day capability (#2701, #2527, #2970).
func composeEnrollmentRequests(b graphBuild) (graphBuild, error) {
	b.enrollmentCapacity = b.Enrollment.Capacity(b.settingsService)
	// Consent rows back both the notification preferences and the opt-in
	// "Neue Anmeldung" mail of the intake.
	consent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
		DB: b.DB.Pool(), Observe: b.ObserveCommunication,
	})
	if err != nil {
		return b, err
	}
	b.notificationConsent = consent
	people, records := b.People, b.Enrollment.Bookings
	b.graph.EnrollmentRequest = b.Enrollment.Intake(EnrollmentIntakeSources{
		Requests: records, ParentChanges: records, Children: records, Bookings: b.Enrollment.CareBookings,
		Guardians: records, LateInviteRepo: records, CareOfferingRepo: b.enrollmentCareOfferings,
		Capacity: b.enrollmentCapacity, Catalog: records, SchoolRepo: people.EnrollmentSchools,
		Notifications: b.enrollmentNotifications, StudentRepo: people.Students, GuardianAuthorizer: people.StudentGuardians,
		RateLimitRepo: records, OutboxEnqueuer: intakeMailOutbox(b.outbox()), Settings: b.settingsService,
		AdminSubscribers: b.Enrollment.AdminSubscribers(consent, b.identityAccess), ManualDecider: b.enrollmentDecisions,
		FrontendURL: b.frontendURL, ParentsURL: b.parentsURL, Logger: b.Logger.With("service", "enrollment-request"),
	})
	b.graph.EnrollmentReport = b.Enrollment.Reports(EnrollmentReportSources{
		Owner: records, Offerings: b.enrollmentCareOfferings, AccessLog: b.audits.DataAccessLog,
		Students: people.Students, Persons: people.Persons, Groups: education.NewGroupNames(people.Groups),
		StudentGuardians: people.StudentGuardians, Companions: NewStudentCompanionRepository(b.CarePlan),
		ClassListEntries: b.Presence.ClassListEntries, PickupSchedules: b.graph.PickupSchedule,
		CareParticipation: b.graph.CareLifecycle, Settings: b.settingsService,
	})
	b.graph.ClassDayArrivalExceptions = b.Presence.ClassDay(ClassDaySources{
		Caller: b.userContext, Reports: b.graph.EnrollmentReport, StatusDays: b.CarePlanWiring.StatusDays,
		PickupTimes: b.graph.PickupSchedule, ArrivalTimes: b.graph.ArrivalSchedule, CareDays: b.graph.CareDay,
		Companions: NewStudentCompanionRepository(b.CarePlan), Students: people.Students, Persons: people.Persons,
		StudentGuardians: people.StudentGuardians, AccessLog: b.audits.DataAccessLog,
		ClassArrivalExceptions: b.graph.ArrivalSchedule, Settings: b.settingsService,
		BlockStarts: b.graph.TimetableOperations, Broadcaster: b.graph.RealtimeHub,
		Logger: b.Logger.With("service", "class-day-arrival-exceptions"),
	})
	return b, nil
}

// composeEnrollmentChanges binds the child documents (#777), the change
// requests and the rollover, which approves auto-renewed rows through the
// decision flow on the rollover_auto_approve=true deadline path.
func composeEnrollmentChanges(b graphBuild) (graphBuild, error) {
	documents, err := b.CarePlanWiring.StudentDocuments(StudentDocumentSources{
		Caller: b.userContext, Edits: b.audits.StudentFieldEdit, AccessLog: b.audits.DataAccessLog,
	})
	if err != nil {
		return b, fmt.Errorf("compose care plan documents: %w", err)
	}
	b.graph.StudentDocuments = documents
	people, records := b.People, b.Enrollment.Bookings
	b.graph.EnrollmentChangeRequest = b.Enrollment.ChangeRequests(EnrollmentChangeRequestSources{
		Bookings: b.Enrollment.CareBookings, Requests: records, Children: records, Guardians: records,
		LateInviteRepo: records, CareOfferingRepo: b.enrollmentCareOfferings, Capacity: b.enrollmentCapacity,
		Catalog: records, Notifications: b.enrollmentNotifications, GuardianProfileRepo: people.GuardianProfiles,
		GuardianPhoneRepo: peopleCompose.NewGuardianPhoneNumberRepository(b.DB.Pool()), GuardianInvitations: b.Enrollment.GuardianInvitations(b.identityAccess),
		PersonRepo: people.Persons, StudentRepo: people.Students, GuardianAuthorizer: people.StudentGuardians,
		Decisions: b.enrollmentDecisions, BookingGates: b.late.careBookings, CompanionGraphLocker: b.CarePlanWiring.CompanionGraph,
		Settings: b.settingsService, OutboxEnqueuer: intakeMailOutbox(b.outbox()), FrontendURL: b.frontendURL, ParentsURL: b.parentsURL,
		Logger: b.Logger.With("service", "enrollment-change-request"),
	})
	b.graph.EnrollmentRollover = b.Enrollment.Rollovers(EnrollmentRolloverSources{
		Bookings: b.Enrollment.CareBookings, Phases: records, Requests: records, Children: records,
		Catalog: b.careOfferings, Notifications: b.enrollmentNotifications, PhaseEligibility: b.enrollmentPhases,
		Outbox: intakeMailOutbox(b.outbox()), Settings: b.settingsService, Decisions: b.enrollmentDecisions, ParentsURL: b.parentsURL,
		Logger: b.Logger.With("service", "enrollment-rollover"),
	})
	b.reviewPolicy = b.CarePlanWiring.ReviewPolicy(b.callerContext.ParentRequestReviews)
	return b, nil
}

// composeCareRequests binds the care-schedule change requests (#1803), the
// post-enrollment offering changes (#1665, #3561) and Care Plan's pickup
// adjustments. One append-only ledger keeps every parent request's history
// (#2267). The sharing rules live in the parents domain, which is composed
// later, so the sharing port resolves it at call time (#2267, story 47).
func composeCareRequests(b graphBuild) (graphBuild, error) {
	b.requestShares = shareVisibilityFunc(func(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
		if b.late.requestShareVisibility == nil {
			return nil, nil
		}
		return b.late.requestShareVisibility.SharedRecipientAccountIDs(ctx, studentID, requestType, requestID)
	})
	b.graph.CareRequests = b.CarePlanWiring.CareRequests(CareRequestSources{
		Arrivals: b.graph.ArrivalSchedule, Pickups: b.graph.PickupSchedule, AutoExcusal: b.pickupAutoExcusal,
		Staff: b.userContext, Emitter: b.graph.ParentEventEmitter, Broadcaster: b.graph.RealtimeHub,
		Reviews: b.reviewPolicy, Permissions: b.Identity.Permissions, Events: peopleCompose.NewParentRequestEventRepository(b.DB.Pool()),
		Audit: b.graph.StudentAudit, Shares: b.requestShares, Today: b.today,
		Logger: b.Logger.With("service", "care-requests"),
	})
	offeringChanges, err := b.Enrollment.OfferingChanges(OfferingChangeInputs{
		CarePlan: b.CarePlan, Enrollment: b.Enrollment.Bookings, Students: b.People.Students,
		Withdrawals: NewCareWithdrawalCompletionRepository(b.CarePlan), Settings: b.settingsService, Planning: b.Enrollment.OfferingPlanning,
		Bookings: b.late.careBookings, Reviews: b.reviewPolicy, Permissions: b.Identity.Permissions,
		Emitter: b.graph.ParentEventEmitter, Events: peopleCompose.NewParentRequestEventRepository(b.DB.Pool()), Shares: b.requestShares, Today: b.today,
		Logger: b.Logger.With("service", "offering-change-requests"),
	})
	if err != nil {
		return b, fmt.Errorf("compose care plan offering changes: %w", err)
	}
	b.offeringChanges = offeringChanges
	adjustments, err := b.CarePlanWiring.PickupAdjustments(PickupAdjustmentSources{
		PickupSchedules: b.graph.PickupSchedule, ArrivalSchedules: b.graph.ArrivalSchedule,
		Baselines: b.pickupBaselines, Offerings: offeringChanges, Settings: b.settingsService,
		Audit: b.graph.StudentAudit, Today: b.today,
	})
	if err != nil {
		return b, fmt.Errorf("compose care plan pickup adjustments: %w", err)
	}
	b.graph.EnrollmentCareOffering = careplanCompose.NewCareOfferings(b.careOfferings, b.late.careBookings, offeringChanges, adjustments)
	return b, nil
}

// composeNotifications binds the notification router, the consent-backed
// preferences, the staff recipients and the absence notices, and the
// excused-absence requests (#1845) over them. Decided change requests reach
// the parent's devices through the pill emitter, where all three request
// flows converge (#1671).
func composeNotifications(b graphBuild) (graphBuild, error) {
	if !b.vapid.Configured() {
		b.Logger.Info("web push disabled: VAPID keys not configured (VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY / VAPID_SUBSCRIBER)")
	}
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(b.DB.Pool())
	notifier := deliveryCompose.NewNotifier(
		b.settingsService, b.Logger.With("service", "notifications"), deliveryCompose.NotificationDeliveryObserver(b.ObserveDelivery),
		deliveryCompose.NewSSEChannel(b.graph.RealtimeHub, deliveryCompose.WithGuardianChildAccess(
			b.DB.Pool(), b.People.StudentGuardians, b.Logger.With("channel", "sse"))),
		deliveryCompose.NewDurableWebPushChannel(
			b.DB.Pool(), pushSubscriptions, b.vapid, deliveryCompose.NewDurablePushOutbox(b.graph.Delivery),
			b.Logger.With("channel", "web_push"),
		),
	)
	b.graph.Notifications = notifier
	preferences := deliveryCompose.NewPreferenceService(b.notificationConsent, b.settingsService, b.DB.Pool(), b.identityAccess, securityruntime.HasPermission)
	b.graph.NotificationPreferences = preferences
	recipients := deliveryCompose.NewStaffRecipientResolver(preferences, deliveryStudentGroups{students: b.People.Students},
		b.People.Groups, b.People.Staff, b.identityAccess, b.settingsService,
		deliveryOnDutyStaff{presence: workforceCompose.WorkPresence(b.WorkTime)})
	b.staffParentMessages = deliveryCompose.NewStaffParentMessageNotifier(notifier, recipients,
		b.Platform.ParentPortal.ParentMessageThread, b.DB.Pool(), b.Logger.With("producer", "staff_parent_message_notifications"))
	b.graph.ParentEventEmitter.WithDecisionNotifications(deliveryCompose.NewRequestDecisionNotifier(notifier, preferences))
	b.graph.AbsenceNotifier = deliveryCompose.NewAbsenceNotifier(notifier, recipients, b.DB.Pool(), b.Logger.With("producer", "absence_notifications"))
	// The workflow is owned by Care Plan (#3093); this root adapts the
	// retained directories, the review policy and the effect sinks.
	excused, err := b.CarePlanWiring.ExcusedRequests(ExcusedRequestSources{
		Reviews: b.reviewPolicy, Permissions: b.Identity.Permissions, Emitter: b.graph.ParentEventEmitter,
		Broadcaster: b.graph.RealtimeHub, Events: peopleCompose.NewParentRequestEventRepository(b.DB.Pool()), Notifier: b.graph.AbsenceNotifier,
		Shares: b.requestShares, Logger: b.Logger.With("service", "excused-requests"),
	})
	if err != nil {
		return b, fmt.Errorf("compose excused absence requests: %w", err)
	}
	b.excusedRequests = excused
	b.graph.ExcusedRequests = excused
	return b, nil
}

// composeMessaging binds the parent messenger, the OGS-internal colleague
// chat (#2598), which shares the transport but none of its authorization,
// and the staff and parent calendars.
func composeMessaging(b graphBuild) (graphBuild, error) {
	portal := b.Platform.ParentPortal
	b.graph.Messaging = communicationCompose.NewParentMessaging(communicationCompose.ParentMessagingConfig{
		ThreadRepo: portal.ParentMessageThread, MessageRepo: parentStore.NewParentMessageRepository(b.DB.Pool()),
		ReadRepo: portal.ParentMessageRead, Persons: BindParentMessagePeople(b.graph.Users), UserContext: StaffStudentReads{Caller: b.callerContext},
		Settings: settingsValues.NewParentMessagingValues(b.settingsService), Broadcaster: b.graph.RealtimeHub, DB: b.DB.Pool(),
		Logger: b.Logger.With("service", "messaging"), Notifier: BindCommunicationNotificationSender(b.graph.Notifications), Preferences: b.graph.NotificationPreferences,
		// E-Mail an den Sorgeberechtigten bei neuer OGS-Nachricht (#2307):
		// der Rueckfall fuer alle, die Push nicht eingerichtet haben.
		Outbox: b.outbox(), GuardianProfiles: BindParentMessageGuardianProfiles(b.People.GuardianProfiles), Schools: b.People.SchoolNames,
		LoginImages: b.settingsService, ParentsURL: b.parentsURL, Observe: b.ObserveCommunication,
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	b.graph.StaffMessaging = communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo:  staffStore.NewStaffMessageThreadRepository(b.DB.Pool()),
		MessageRepo: staffStore.NewStaffMessageRepository(b.DB.Pool()), ReadRepo: portal.StaffMessageRead,
		Persons: BindStaffSenderNames(b.graph.Users), Settings: settingsValues.NewStaffMessagingValues(b.settingsService), Broadcaster: b.graph.RealtimeHub,
		Logger: b.Logger.With("service", "staffmessaging"), Notifier: BindCommunicationNotificationSender(b.graph.Notifications),
		Preferences: b.graph.NotificationPreferences, Observe: b.ObserveCommunication,
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	calendar := b.TimetableWiring.Calendar(b.calendarDependencies())
	b.graph.Calendar, b.graph.CalendarFeedCleanup = calendar, calendar
	return b, nil
}

// calendarDependencies are the facts, the outbox and the notification
// router the staff and parent calendars bind over.
func (b *graphBuild) calendarDependencies() CalendarDependencies {
	people, repos := b.People, b.TimetableWiring.Repositories
	return CalendarDependencies{
		CalendarFacts: CalendarFacts{
			StaffRepo: people.Staff, StudentRepo: people.Students, GuardianProfileRepo: b.TimetableWiring.CalendarGuardians,
			StudentGuardianRepo: people.StudentGuardians,
			ChildRepo:           careplanCompose.NewParentChildLinks(b.Platform.ParentPortal.ParentChild),
			GroupRepo:           people.Groups, InstanceStaffRepo: repos.InstanceStaff, ActivityInstanceRepo: repos.ActivityInstance,
			RoomRepo: facilitiesCompose.NewRoomRepository(b.Rooms), StaffShiftRepo: b.Workforce.ShiftRows, ShiftTypeRepo: b.Workforce.ShiftTypeRows,
			SchoolRepo: b.Organizations, AccountRepo: b.identityAccess, StaffFeedRepo: b.identityAccess, PersonRepo: people.Persons,
		},
		Appointments: b.Appointments, UserContext: b.userContext.Caller(),
		Runtime: calendarCompose.NewTenantRuntime(b.DB.Pool()), CalendarRenderer: calendarCompose.NewCalendarRenderer(b.Calendar),
		Outbox: newCalendarOutbox(b.late.emailOutbox), PushOutbox: deliveryCompose.NewDurablePushOutbox(b.graph.Delivery),
		Settings: b.settingsService, CalDAVSettings: b.settingsService,
		StaffFeedTombstoneRepo: schoolCalendarCompose.NewFeedHistory(b.DB.Pool()),
		ParentsURL:             b.parentsURL, FrontendURL: b.frontendURL, CalDAVURL: b.publicAPIURL,
		Notifier: newCalendarNotifier(b.graph.Notifications), ReminderNotifier: newCalendarReminderNotifier(b.graph.Notifications),
		Preferences: b.graph.NotificationPreferences, Logger: b.Logger.With("service", "calendar"),
	}
}

// composeParentPortal binds the parents portal workflow and the parent
// announcements, whose cancellation notices the instance lifecycle
// publishes (#2601). A graph composed without the photo lifecycle hands the
// portal none.
func composeParentPortal(b graphBuild) (graphBuild, error) {
	profiles, err := careplanCompose.NewStudentProfiles(b.DB.Pool(), studentDirectoryView.NewReader(), func(careplanCompose.Observation) {})
	if err != nil {
		return b, fmt.Errorf("compose parent portal care profiles: %w", err)
	}
	b.graph.Parent = parentportalcompose.New(b.parentPortalDependencies(profiles))
	announcements := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo: b.Platform.ParentPortal.ParentAnnouncement, Settings: settingsValues.NewParentAnnouncementValues(b.settingsService), Outbox: OutboxEnqueuer{Outbox: b.late.emailOutbox},
		PushOutbox: deliveryCompose.NewDurablePushOutbox(b.graph.Delivery), Notifier: BindCommunicationNotificationSender(b.graph.Notifications),
		ReminderNotifier: BindCommunicationDurableNotificationSender(b.graph.Notifications), Preferences: b.graph.NotificationPreferences,
		Deliveries: b.Platform.AnnouncementDeliveries(b.graph.Delivery), ParentsURL: b.parentsURL,
		Fingerprints: SecurityFingerprints{}, Logger: b.Logger.With("service", "announcement"),
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	b.graph.ParentAnnouncement = announcements
	b.late.guardianNoticePublisher = announcements
	return b, nil
}

// parentPortalDependencies are the owners, rows and services the parents
// portal workflow reads and writes through.
func (b *graphBuild) parentPortalDependencies(profiles careplan.StudentProfileCommands) parentportalcompose.Dependencies {
	people, portal := b.People, b.Platform.ParentPortal
	messages := parentStore.NewParentMessageRepository(b.DB.Pool())
	return parentportalcompose.Dependencies{
		GuardianChanges: ParentGuardianChanges(),
		DeparturePolicy: ParentDeparturePolicy(),
		GuardianPolicy:  NewParentGuardianPolicy(),
		Fingerprints:    SecurityFingerprints{},
		Declarations:    ParentDeclarationPolicy(),
		Locales:         ParentPortalLocales(),
		Runtime:         ParentPortalRuntime{},
		CareProfiles:    profiles, ChildRepo: NewParentChildReads(careplanCompose.NewParentChildQueries(portal.ParentChild, securityruntime.GuardianPermissionGranted)), EnrollablePhaseRepo: NewParentPhaseReads(careplanCompose.NewParentPhaseQueries(portal.ParentEnrollablePhase)),
		EnrollmentSettings: b.settingsService, EnrollmentRequestRepo: NewParentEnrollmentReads(careplanCompose.NewParentEnrollmentQueries(portal.ParentEnrollmentRequest)),
		GuardianProfileRepo: NewParentGuardianProfileReads(people.GuardianProfiles), Attendance: BindParentAttendance(b.Presence.Presence), StatusDayRepo: b.CarePlanWiring.StatusDays,
		MealPlan: b.MealPlan, StudentRepo: NewParentStudentReads(people.Students), CarePlan: NewParentCarePlanCommands(b.CarePlan, b.pickupAutoExcusal), DataRequests: NewParentDataRequestCommands(b.CarePlan), People: b.Persons,
		Settings: BindParentSettings(b.settingsService), StudentUpdates: BindParentStudentUpdates(b.graph.RealtimeHub),
		PersonRepo: NewParentPersonReads(people.Persons), ChangeRequestRepo: NewParentDataRequestReads(b.CarePlan),
		CareRequestRepo: NewParentCareRequestReads(b.CarePlan), ExcusedRequestRepo: b.CarePlanWiring.ExcusedRequestRows,
		OfferingChangeRequestRepo: NewParentOfferingChangeReads(enrollmentCompose.NewOfferingChangeRecords(b.CarePlan)),
		FamilyProtectionEvents:    BindParentFamilyProtectionReads(peopleCompose.NewFamilyProtectionEventRepository(b.DB.Pool())),
		ParentRequestShares:       BindParentRequestShareRecords(peopleCompose.NewParentRequestShareEventRepository(b.DB.Pool())),
		ParentRequestEvents:       NewParentRequestEvents(peopleCompose.NewParentRequestEventRepository(b.DB.Pool())), StudentAudit: NewParentStudentAudit(b.graph.StudentAudit),
		MessageThreadRepo: BindParentMessageThreadAccess(portal.ParentMessageThread), MessageRepo: BindParentMessageReads(messages), MessageReadRepo: BindParentMessageInboxReads(portal.ParentMessageRead),
		Conversations: BindParentConversationCore(communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
			ThreadRepo: portal.ParentMessageThread, MessageRepo: messages, ReadRepo: portal.ParentMessageRead,
			Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "parent"),
		})),
		ParentMessageNotifier: bindParentMessageNotifier(b.staffParentMessages), ArrivalSchedules: NewParentArrivalSchedules(b.graph.ArrivalSchedule),
		PickupSchedules: NewParentPickupSchedules(b.graph.PickupSchedule), CareRequests: b.graph.CareRequests, ExcusedRequests: NewParentExcusedAbsenceRequests(b.excusedRequests),
		Emitter: BindParentChildEvents(b.graph.ParentEventEmitter), MessagingEnabled: BindParentMessagingGate(b.settingsService, b.Logger.With("service", "parent")), AnnouncementRepo: BindParentAnnouncementAccess(portal.ParentAnnouncement),
		GuardianInvites: b.identity.ParentGuardianAccess, GuardianInvitations: b.identity.GuardianInvitations,
		StudentGuardianRepo: NewParentStudentGuardianReads(people.StudentGuardians), GuardianPhoneRepo: NewParentGuardianPhoneReads(peopleCompose.NewGuardianPhoneNumberRepository(b.DB.Pool())),
		StudentConsents: NewParentStudentConsents(b.graph.StudentConsents), StudentPhotos: studentPhotoUnlinker(b.graph.StudentPhotos),
		AbsenceNotifier: bindParentAbsenceNotifier(b.graph.AbsenceNotifier), CarePeriods: parentportal.NewCarePeriodReads(b.Enrollment.ParentCarePeriods.CarePeriods),
		OfferingHistory: parentportal.NewOfferingHistoryReads(b.Enrollment.ParentOfferingHistory.OfferingHistory), CareOfferingRepo: NewParentCareOfferingReads(b.enrollmentCareOfferings),
		OfferingChanges: NewParentOfferingChangeRequests(b.offeringChanges), Logger: b.Logger.With("service", "parent"), Now: b.now,

		CarePlanPolicy: NewParentCarePlanPolicy(),
	}
}

// studentPhotoUnlinker hands the parents portal the photo lifecycle; a graph
// without one hands it none.
func studentPhotoUnlinker(photos peoplePorts.StudentPhotoService) func() parentportal.StudentPhotoUnlinker {
	return func() parentportal.StudentPhotoUnlinker {
		if photos == nil {
			return nil
		}
		return photos
	}
}

// composeFileStore binds school file storage (#2596) and the attachments of
// Elternmitteilungen (#2890) through the public File Storage capability
// (#2707). Die Datei gehört der Dateiablage, der Empfängerkreis der
// Mitteilung: beide Seiten zeigen aufeinander, also werden sie hier
// verbunden. Ohne Objektspeicher verweigern die Anhang-Pfade den Dienst,
// statt ohne Prüfung zu entscheiden.
func composeFileStore(b graphBuild) (graphBuild, error) {
	storage := b.FileStorage
	if storage.Objects == nil {
		return b, nil
	}
	if storage.Settings == nil || storage.Events == nil {
		return b, errors.New("file storage settings and events bindings are required with an object store")
	}
	store, err := filestorageCompose.New(filestorageCompose.Dependencies{
		DB: b.DB.Pool(), Objects: storage.Objects, Identity: b.identityAccess, People: b.Persons,
		Settings: storage.Settings(b.settingsService), Events: storage.Events(b.audits.FileEvent),
		HasPermission: securityruntime.HasPermission, Announcements: b.graph.ParentAnnouncement,
		GuardianAudience: b.graph.Parent, Observe: storage.Observe, Logger: b.Logger.With("service", "filestore"), Now: b.now,
	})
	if err != nil {
		return b, err
	}
	b.graph.ParentAnnouncement.SetAttachmentPurger(store)
	b.graph.FileStore = store
	return b, nil
}
