//go:build testhooks

package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

type StudentTestModule struct {
	ActiveTestModule
	GradeTransitionTestModule
	PeopleDirectory    peopledirectory.Capability
	Audit              auditModels.Command
	Schools            organizationtenancy.Capability
	CareLifecycle      careplan.CareLifecycle
	StudentAudit       peoplePorts.StudentAuditService
	PartialAbsence     careplan.PartialAbsenceService
	EnrollmentDecision enrollmentOwner.Decisions
	CareRequests       carerequests.Service
	OfferingChanges    careplan.OfferingChangeCapability
	PickupAdjustments  careplan.PickupAdjustments
	ExcusedRequests    careplan.ExcusedAbsenceRequests
	MasterDataReview   masterdatarequests.Decisions
	ParentRequests     parentrequests.Coordinator
	OGSGroupLive       grouplive.Query
	StudentPhotos      peoplePorts.StudentPhotoService
	// NewStudentPhotos rebinds the photo lifecycle to the caller's broadcaster
	// and file cleanup. Adapter tests assert on both, and the stored files are
	// an api-layer concern this graph cannot supply.
	NewStudentPhotos func(PhotoBroadcaster, peoplePorts.PhotoUnlinker) peoplePorts.StudentPhotoService
	// NewPickupAdjustments rebinds the pickup adjustment to the caller's
	// offering adjustments, so a route test can fail an apply after the
	// offering write.
	NewPickupAdjustments func(careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error)
}

// ManualPartialAbsences binds the owner projection without constructing another service graph.
func (m StudentTestModule) ManualPartialAbsences(source careplan.Capability) presenceservice.ManualPartialAbsenceReader {
	return NewManualPartialAbsenceDates(source)
}

// DataAccessAudit supplies the access-evidence writer from this module's audit command.
func (m StudentTestModule) DataAccessAudit() presenceservice.DataAccessAudit {
	return NewDataAccessAudit(studentAccessAuditWriter{m.Audit})
}

type studentAccessAuditWriter struct{ auditModels.Command }

func (w studentAccessAuditWriter) Create(ctx context.Context, entry *auditModels.DataAccessLog) error {
	return w.Append(ctx, entry)
}

func NewStudentTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, feedbackCounter studentdeletioncompose.Feedback, clocks ...func() time.Time) (StudentTestModule, error) {
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return StudentTestModule{}, err
	}
	repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return StudentTestModule{}, err
	}
	live, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return StudentTestModule{}, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, err
	}
	// The offering-roster resync is provided by the enrollment decision
	// service constructed below; the closure reads it once it exists.
	var offeringResync schoolStructure.OfferingSourceResyncer
	grade, err := NewGradeTransitionTestModule(db, func(ctx context.Context, effectiveFrom timezone.Date) error {
		if offeringResync == nil {
			return nil
		}
		return offeringResync.ResyncOfferingSourcedTemplates(ctx, effectiveFrom)
	}, clocks...)
	if err != nil {
		return StudentTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, err
	}
	guardian, err := NewGuardianTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, err
	}
	persons := guardian.PeopleDirectory
	contextRepos, err := NewUserContextTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestModule{}, err
	}
	logger := slog.Default()
	now := OptionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	settingsService := live.Settings
	// Care Plan is composed after the live graph's person directory; the
	// module hands out the copy that reaches it.
	usersService := live.Users.WithCareParticipation(CareParticipationResolver(func() careplan.CareParticipation {
		return care.CareLifecycle
	}))
	live.Users = usersService
	userContextService := live.UserContext
	educationService := live.Education
	activeService := live.Active
	instanceService := live.Instance
	pickupScheduleService := live.PickupSchedule
	arrivalScheduleService := live.ArrivalSchedule
	careDayService := live.CareDay
	careLifecycleService := care.CareLifecycle
	studentAuditService := care.StudentAudit
	emailOutboxService := delivery.EmailOutbox
	frontendURL := CurrentServiceConfig().FrontendURL
	parentsURL := CurrentServiceConfig().ParentsURL
	studentConsentService := NewStudentConsents(db)
	// The stored files live in the API layer, so a services-only graph binds
	// no unlinker: the runtime skips the cleanup instead of guessing a path.
	newStudentPhotos := func(broadcaster PhotoBroadcaster, unlinker peoplePorts.PhotoUnlinker) peoplePorts.StudentPhotoService {
		return NewStudentPhotos(persons, guardian.PhotoRuntime, StudentPhotoRuntimeDependencies{
			Settings: settingsService, Broadcaster: broadcaster, Unlinker: unlinker,
			Consents: studentConsentService, Logger: logger,
			ActingAccount: actingAccountID,
		})
	}
	studentPhotoService := newStudentPhotos(realtimeHub, nil)
	careplanCompose.WireCareParticipation(careDayService, careLifecycleService)
	approvedOfferings := enrollmentCompose.NewApprovedOfferingProjection(repos.Enrollment(), OfferingStudents{Query: persons})
	pickupBaselines, err := careplanCompose.NewPickupBaselines(repos.CarePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: repos.CarePlan, Baselines: pickupBaselines, Blocks: NewLoggedStudentPresence(db, logger), Preview: NewPickupExcusalTimetable(repos.Timetable, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	rosterReconciler := timetableCompose.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, logger, now)
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return StudentTestModule{}, err
	}
	pillEmitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{

		Runtime:     NewCommunicationEventRuntime(db.Pool() != nil, unit),
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		Settings:    settingsValues.NewParentEventValues(settingsService),
		Broadcaster: realtimeHub,
		Logger:      logger.With("service", "parent-events"),
	})
	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db.Pool(), repos.CarePlan, NewLoggedStudentPresence(db, logger), pickupAutoExcusal)
	if err != nil {
		return StudentTestModule{}, err
	}
	guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return StudentTestModule{}, err
	}
	offeringLinks, err := NewCareOfferingCatalogTestModule(db, unit, CareOfferingCatalogTestOptions{
		Settings: settingsService, LockRecurrence: recurrenceLock.LockRecurrenceWrites,
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	careBookings, err := newBookingMaterialization(bookingMaterializationInputs{
		Catalog: offeringLinks.Catalog, Timetable: repos.Timetable, Rosters: rosterReconciler, Students: persons,
		Periods: repos.SchoolCalendar(), Enrollment: repos.Enrollment(), Approved: approvedOfferings,
		Settings: settingsService, Bookings: repos.CarePlan, Withdrawals: careLifecycleService,
		Adjustments: repos.EnrollmentOfferingAdjustment, Persons: repos.Person, Accounts: guardianAccess,
		Pickup: pickupBaselines, PickupRows: repos.CarePlan,
		LockRecurrence:           recurrenceLock.LockRecurrenceWrites,
		ResyncPickupAutoExcusals: pickupAutoExcusal.ResyncFutureExceptions,
		Broadcaster:              realtimeHub,
		GuardianNotifier:         pillEmitter,
		Today:                    today,
		Logger:                   logger.With("service", "care-plan-bookings"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	enrollmentDecisionService := enrollmentCompose.PublicDecisions(NewEnrollmentDecisions(EnrollmentDecisionSources{
		Requests:               repos.Enrollment(),
		Children:               repos.Enrollment(),
		Guardians:              repos.Enrollment(),
		LateInvites:            repos.Enrollment(),
		CareOfferings:          enrollmentCompose.NewCareOfferingRecords(repos.CarePlan),
		Phases:                 repos.Enrollment(),
		Schemas:                repos.Enrollment(),
		DataAccessLog:          repos.DataAccessLog,
		OfferingAdjustments:    repos.EnrollmentOfferingAdjustment,
		Restorations:           repos.EnrollmentRestorationAudit,
		Notifications:          NewEnrollmentNotifications(repos.Enrollment(), settingsService, OutboxEnqueuer{Outbox: emailOutboxService}, EnrollmentSchoolDirectory{Schools: repos.School}),
		Persons:                repos.Person,
		Staff:                  repos.Staff,
		Students:               repos.Student,
		StudentGuardians:       repos.StudentGuardian,
		GuardianFinancialAudit: repos.GuardianFinancialChange,
		GuardianProfiles:       repos.GuardianProfile,
		GuardianPhones:         repos.GuardianPhoneNumber,
		PickupSchedules:        timetableCompose.NewEnrollmentPickupSchedules(repos.StudentPickupSchedule),
		ArrivalSchedules:       timetableCompose.NewEnrollmentArrivalSchedules(repos.StudentArrivalSchedule),
		CareBookings:           careBookings,
		GuardianAccess:         guardianAccess,
		StudentEnrollment:      persons,
		Companions:             NewStudentCompanionRepository(repos.CarePlan),
		DeleteCompanions:       repos.CarePlan.DeleteCompanionEdges,
		StudentAudit:           studentAuditService,
		StudentConsents:        studentConsentService,
		CareWithdrawal:         careLifecycleService,
		Broadcaster:            realtimeHub,
		FrontendURL:            frontendURL,
		ParentsURL:             parentsURL,
		Settings:               settingsService,
		LockTemplateRecurrence: recurrenceLock.LockRecurrenceWrites,
		Pickups: enrollmentCompose.WeeklyPickupHooks{
			LockStudents:  careplanCompose.LockWeeklyPickupStudents(pickupStudentLocks{students: persons}),
			ResyncExcusal: pickupAutoExcusal.ResyncFutureExceptions,
		},
		Logger: logger.With("service", "enrollment-decision"),
		Today:  today,
	}))
	offeringResync = careBookings
	requestReviewPolicy := NewParentRequestReviewPolicy(userContextService.Caller().ParentRequestReviews)
	parentRequestEvents := repos.ParentRequestEvent
	careRequestService := NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		repos.CarePlan,
		persons,
		arrivalScheduleService,
		pickupScheduleService,
		NewLoggedStudentPresence(db, logger),
		pickupAutoExcusal,
		repos.CarePlan,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		requestTokenReads.Permissions,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
		WithCareRequestToday(today),
	)
	offeringChanges, err := newOfferingChanges(OfferingChangeInputs{
		CarePlan: repos.CarePlan, Enrollment: repos.Enrollment(), Students: repos.Student,
		Withdrawals: repos.CareWithdrawal, Settings: settingsService,
		Planning: ManualPlanningReader{Query: PlanningQuery{DB: db}, CourseGroups: repos.Timetable},
		Bookings: careBookings, Reviews: requestReviewPolicy, Permissions: requestTokenReads.Permissions, Emitter: pillEmitter,
		Events: parentRequestEvents, Today: today, Logger: logger.With("service", "offering-change-requests"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	newPickupAdjustmentsFor := func(offerings careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error) {
		return NewPickupAdjustments(PickupAdjustmentInputs{
			CarePlan: repos.CarePlan, PickupSchedules: pickupScheduleService, ArrivalSchedules: arrivalScheduleService,
			Baselines: pickupBaselines, Offerings: offerings, Settings: settingsService,
			Audit: studentAuditService, Students: repos.Student, Today: today,
		})
	}
	pickupAdjustments, err := newPickupAdjustmentsFor(offeringChanges)
	if err != nil {
		return StudentTestModule{}, err
	}
	excusedRequestService, err := NewExcusedAbsenceRequestService(ExcusedRequestServiceWiring{
		CarePlan: repos.CarePlan, Students: repos.Student, Persons: repos.Person,
		Scope:   ParentRequestReviewScope(requestReviewPolicy, requestTokenReads.Permissions),
		Emitter: pillEmitter, Broadcaster: realtimeHub, Events: parentRequestEvents,
		Logger: logger.With("service", "excused-requests"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	masterDataDecisions, err := NewMasterDataDecisions(MasterDataDecisionWiring{
		CarePlan: repos.CarePlan, Fields: persons, Students: repos.Student, Persons: repos.Person,
		Audit: studentAuditService, Scope: ParentRequestWriteScope(requestReviewPolicy, requestTokenReads.Permissions),
		Emitter: pillEmitter, Broadcaster: realtimeHub, Events: parentRequestEvents,
		Logger: logger.With("service", "master-data-review"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	parentRequestCoordinator, err := NewParentRequestCoordinator(masterDataDecisions, excusedRequestService, careRequestService,
		newOfferingChangeConflicts(offeringChanges), parentRequestEvents, requestTokenReads.Permissions)
	if err != nil {
		return StudentTestModule{}, err
	}
	scheduleSubstitution, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: live.Deviations, Staff: NewPlanStaffDirectory(repos.Staff), Broadcaster: realtimeHub, Logger: logger.With("service", "schedule-substitution"),
		ActivityInstances: timetableCompose.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:     timetableCompose.NewTimetableInstanceStaffReads(repos.InstanceStaff),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	substitutionService := schoolStructure.NewSubstitutionModule(schoolStructure.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: NewEducationHandovers(contextRepos.Substitutions, NewTestGroupHandovers(db)),
		Persons: NewEducationPersonQuery(persons), Teachers: NewEducationCaregivers(repos.Teacher),
		Staff: NewEducationStaff(repos.Staff), Actors: SubstitutionActorResolver{Identity: userContextService.Caller()},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   NewEducationSubstitutionAudit(repos.SubstitutionChange),
		Runtime:                 schoolStructure.NewLegacyRepositoryRuntime(db.Pool()), Broadcaster: NewGroupEvents(realtimeHub, logger),
		Logger:   logger.With("service", "substitution"),
		Schedule: scheduleSubstitution,
		CanSeeAll: func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
			if assignmentBound {
				return false, nil
			}
			scope, err := settingsService.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
			if err != nil {
				return false, fmt.Errorf("resolve operational overview scope: %w", err)
			}
			return admin || (scope == configModels.OverviewScopeAllStaff && hasStaff), nil
		},
		Now: now,
	})
	studentStatusDayService := presenceservice.NewStatusDays(
		repos.StudentStatusDay,
		NewManualPartialAbsenceDates(repos.CarePlan),
		db.Pool(),
		repos.CarePlan.LockExceptionDay,
		now,
	)
	ogsGroupLiveService, err := newGroupLive(groupLiveSources{
		Presence:          NewLoggedStudentPresence(db, logger),
		People:            GroupRosterPeople(usersService),
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       GroupLiveCaller{CallerRows: userContextService},
		Active:            activeService,
		Settings:          settingsService,
		Pickups:           pickupScheduleService,
		Arrivals:          arrivalScheduleService,
		PlannedStudentIDs: instanceService.GetPlannedStudentIDsByDate,
		CareDays:          careDayService,
		CareParticipation: careLifecycleService,
		ExcusedRequests:   excusedRequestService,
		StatusDays:        studentStatusDayService,
		Logger:            logger.With("service", "ogs-group-live"),
		Now:               now,
	})
	if err != nil {
		return StudentTestModule{}, fmt.Errorf("compose OGS group live projection: %w", err)
	}
	return StudentTestModule{
		ActiveTestModule: live, GradeTransitionTestModule: grade, PeopleDirectory: persons, Audit: auditCommand,
		StudentPhotos: studentPhotoService, NewStudentPhotos: newStudentPhotos, NewPickupAdjustments: newPickupAdjustmentsFor,
		Schools: repos.School, CareLifecycle: careLifecycleService, StudentAudit: studentAuditService,
		PartialAbsence: partialAbsenceService, EnrollmentDecision: enrollmentDecisionService, CareRequests: careRequestService,
		OfferingChanges: offeringChanges, PickupAdjustments: pickupAdjustments, ExcusedRequests: excusedRequestService,
		MasterDataReview: masterDataDecisions, ParentRequests: parentRequestCoordinator, OGSGroupLive: ogsGroupLiveService,
	}, nil
}

// StatusDayOverviewPeople serves the absence overview's people reads from the
// module's users service.
func (m StudentTestModule) StatusDayOverviewPeople() presenceservice.StatusDayOverviewPeople {
	return StatusDayOverviewPeople(m.Users)
}

// HistorySlots supplies the same owner projection to narrow student route fixtures.
func (m StudentTestModule) HistorySlots(records timetablePorts.AttendanceHistoryRecords) presenceservice.HistorySlotReader {
	return NewHistorySlots(records)
}
