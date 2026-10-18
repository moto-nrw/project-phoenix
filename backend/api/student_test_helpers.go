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

// StudentTestModule is the Care Plan side of the student graph: the care
// lifecycle, the partial absences, the parent requests and their reviews,
// the offering changes and the pickup adjustments. The other owners'
// services of the graph come beside it (NewStudentTestModule).
type StudentTestModule struct {
	CareLifecycle     careplan.CareLifecycle
	PartialAbsence    careplan.PartialAbsenceService
	CareRequests      carerequests.Service
	OfferingChanges   careplan.OfferingChangeCapability
	PickupAdjustments careplan.PickupAdjustments
	ExcusedRequests   careplan.ExcusedAbsenceRequests
	MasterDataReview  masterdatarequests.Decisions
	ParentRequests    parentrequests.Coordinator
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

// StudentTestPeople are the People Directory services of the student graph:
// the owner capability, the student audit and the photo lifecycle.
type StudentTestPeople struct {
	PeopleDirectory peopledirectory.Capability
	StudentAudit    peoplePorts.StudentAuditService
	StudentPhotos   peoplePorts.StudentPhotoService
}

// StudentTestDataAccessAudit supplies the access-evidence writer from the
// student graph's audit command.
func StudentTestDataAccessAudit(audit auditModels.Command) presenceservice.DataAccessAudit {
	return NewDataAccessAudit(studentAccessAuditWriter{audit})
}

type studentAccessAuditWriter struct{ auditModels.Command }

func (w studentAccessAuditWriter) Create(ctx context.Context, entry *auditModels.DataAccessLog) error {
	return w.Append(ctx, entry)
}

// NewStudentTestModule composes the student graph over db and returns its
// Care Plan side with the live graph it runs over, its People Directory
// services, the Audit command it records into, the Enrollment decisions and
// the OGS group live projection.
func NewStudentTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, feedbackCounter studentdeletioncompose.Feedback, clocks ...func() time.Time) (StudentTestModule, ActiveTestModule, StudentTestPeople, auditModels.Command, enrollmentOwner.Decisions, grouplive.Query, error) {
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	students, repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	studentPeople, err := NewStudentPeopleRows(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	studentAudits := NewStudentAuditRows(db, auditCommand)
	messages := NewStudentMessageRows(db)
	live, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	guardian, err := NewGuardianTestModule(db, unit)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	persons := guardian.PeopleDirectory
	contextRepos, _, err := NewUserContextTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
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
	pickupBaselines, err := careplanCompose.NewPickupBaselines(students.CarePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: students.CarePlan, Baselines: pickupBaselines, Blocks: NewLoggedStudentPresence(db, logger), Preview: NewPickupExcusalTimetable(repos.Timetable, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	rosterReconciler := timetableCompose.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, logger, now)
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	pillEmitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{

		Runtime:     NewCommunicationEventRuntime(db.Pool() != nil, unit),
		ThreadRepo:  messages.ParentMessageThread,
		MessageRepo: messages.ParentMessage,
		Settings:    settingsValues.NewParentEventValues(settingsService),
		Broadcaster: realtimeHub,
		Logger:      logger.With("service", "parent-events"),
	})
	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db.Pool(), students.CarePlan, NewLoggedStudentPresence(db, logger), pickupAutoExcusal)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	offeringLinks, err := NewCareOfferingCatalogTestModule(db, unit, CareOfferingCatalogTestOptions{
		Settings: settingsService, LockRecurrence: recurrenceLock.LockRecurrenceWrites,
	})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	careBookingDeps, err := bookingMaterializationOwners(
		repos.Timetable, rosterReconciler, persons, repos.SchoolCalendar(), repos.Enrollment(), approvedOfferings,
		studentAudits.EnrollmentOfferingAdjustment, repos.Person, guardianAccess, pickupBaselines, students.CarePlan,
	)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	careBookingDeps.Catalog = offeringLinks.Catalog
	careBookingDeps.Settings = settingsValues.NewCareBookingValues(settingsService)
	careBookingDeps.Bookings = students.CarePlan
	careBookingDeps.Withdrawals = careLifecycleService
	careBookingDeps.LockTemplateRecurrence = recurrenceLock.LockRecurrenceWrites
	careBookingDeps.ResyncPickupAutoExcusals = pickupAutoExcusal.ResyncFutureExceptions
	careBookingDeps.PickupGuardians = pillEmitter
	careBookingDeps.Today = today
	careBookingDeps.Logger = logger.With("service", "care-plan-bookings")
	careBookings, err := newBookingMaterialization(careBookingDeps, realtimeHub)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	enrollmentDecisionService := enrollmentCompose.PublicDecisions(NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
		Requests:       repos.Enrollment(),
		Children:       repos.Enrollment(),
		Guardians:      repos.Enrollment(),
		LateInvites:    repos.Enrollment(),
		Phases:         repos.Enrollment(),
		Schemas:        repos.Enrollment(),
		Offerings:      enrollmentCompose.NewCareOfferingRecords(students.CarePlan),
		Notifications:  NewEnrollmentNotifications(repos.Enrollment(), settingsService, OutboxEnqueuer{Outbox: emailOutboxService}, EnrollmentSchoolDirectory{Schools: repos.School}),
		Bookings:       careBookings,
		GuardianAccess: guardianAccess,
		People: EnrollmentDecisionPeopleDirectory(
			repos.Person, repos.Staff, repos.Student, repos.StudentGuardian, repos.GuardianProfile, repos.GuardianPhoneNumber, persons,
		),
		StudentEnrollment:      persons,
		Companions:             EnrollmentDecisionCompanions(NewStudentCompanionRepository(students.CarePlan), students.CarePlan.DeleteCompanionEdges),
		PickupSchedules:        timetableCompose.NewEnrollmentPickupSchedules(repos.StudentPickupSchedule),
		ArrivalSchedules:       timetableCompose.NewEnrollmentArrivalSchedules(repos.StudentArrivalSchedule),
		AccessLog:              EnrollmentDecisionAccessLog(repos.DataAccessLog),
		Adjustments:            EnrollmentDecisionAdjustments(studentAudits.EnrollmentOfferingAdjustment),
		Restorations:           EnrollmentDecisionRestorations(studentAudits.EnrollmentRestorationAudit),
		Payers:                 EnrollmentDecisionPayers(studentAudits.GuardianFinancialChange),
		StudentAudit:           EnrollmentDecisionStudentAudit(studentAuditService),
		StudentConsents:        EnrollmentDecisionStudentConsents(studentConsentService),
		CareWithdrawal:         careLifecycleService,
		Broadcasts:             EnrollmentDecisionBroadcasts(realtimeHub),
		ParentsURL:             EnrollmentDecisionParentsURL(parentsURL, frontendURL),
		Settings:               EnrollmentDecisionSettingValues(settingsService),
		LockTemplateRecurrence: recurrenceLock.LockRecurrenceWrites,
		Pickups: enrollmentCompose.WeeklyPickupHooks{
			LockStudents:  careplanCompose.LockWeeklyPickupStudents(pickupStudentLocks{students: persons}),
			ResyncExcusal: pickupAutoExcusal.ResyncFutureExceptions,
		},
		Logger: logger.With("service", "enrollment-decision"),
		Today:  today,
	}))
	requestReviewPolicy := NewParentRequestReviewPolicy(userContextService.Caller().ParentRequestReviews)
	parentRequestEvents := studentPeople.ParentRequestEvent
	careRequestService := NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		students.CarePlan,
		persons,
		arrivalScheduleService,
		pickupScheduleService,
		NewLoggedStudentPresence(db, logger),
		pickupAutoExcusal,
		students.CarePlan,
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
	offeringChangeDeps, err := offeringChangeOwners(
		students.CarePlan, nil, repos.Enrollment(), repos.Student, studentPeople.CareWithdrawal, settingsService,
		requestReviewPolicy, requestTokenReads.Permissions, pillEmitter, parentRequestEvents,
	)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	offeringChangeDeps.Planning = ManualPlanningReader{Query: PlanningQuery{DB: db}, CourseGroups: repos.Timetable}
	offeringChangeDeps.Bookings = careBookings
	offeringChangeDeps.Today = today
	offeringChangeDeps.Logger = logger.With("service", "offering-change-requests")
	offeringChanges, err := newOfferingChanges(offeringChangeDeps)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	newPickupAdjustmentsFor := func(offerings careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error) {
		return NewPickupAdjustments(
			students.CarePlan, pickupScheduleService, arrivalScheduleService, pickupBaselines, offerings,
			settingsService, studentAuditService, repos.Student, today,
		)
	}
	pickupAdjustments, err := newPickupAdjustmentsFor(offeringChanges)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	excusedRequestService, err := NewExcusedAbsenceRequestService(
		students.CarePlan, repos.Student, repos.Person, ParentRequestReviewScope(requestReviewPolicy, requestTokenReads.Permissions),
		pillEmitter, realtimeHub, parentRequestEvents, nil, nil, nil, logger.With("service", "excused-requests"), nil,
	)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	masterDataDecisions, err := NewMasterDataDecisions(
		students.CarePlan, persons, repos.Student, repos.Person, studentAuditService,
		ParentRequestWriteScope(requestReviewPolicy, requestTokenReads.Permissions),
		pillEmitter, realtimeHub, parentRequestEvents, nil, logger.With("service", "master-data-review"), nil,
	)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	parentRequestCoordinator, err := NewParentRequestCoordinator(
		masterDataDecisions, excusedRequestService, careRequestService,
		newOfferingChangeConflicts(offeringChanges), parentRequestEvents, requestTokenReads.Permissions,
	)
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	scheduleSubstitution, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: live.Deviations, Staff: NewPlanStaffDirectory(repos.Staff), Broadcaster: realtimeHub, Logger: logger.With("service", "schedule-substitution"),
		ActivityInstances: timetableCompose.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:     timetableCompose.NewTimetableInstanceStaffReads(repos.InstanceStaff),
	})
	if err != nil {
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, err
	}
	substitutionService := schoolStructure.NewSubstitutionModule(schoolStructure.SubstitutionDependencies{
		Groups: NewSchoolStructureSuiteRepositories(db).Group, Substitutions: NewEducationHandovers(contextRepos.Substitutions, NewTestGroupHandovers(db)),
		Persons: NewEducationPersonQuery(persons), Teachers: NewEducationCaregivers(repos.Teacher),
		Staff: NewEducationStaff(repos.Staff), Actors: SubstitutionActorResolver{Identity: userContextService.Caller()},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   NewEducationSubstitutionAudit(studentAudits.SubstitutionChange),
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
		NewManualPartialAbsenceDates(students.CarePlan),
		db.Pool(),
		students.CarePlan.LockExceptionDay,
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
		return StudentTestModule{}, ActiveTestModule{}, StudentTestPeople{}, nil, nil, nil, fmt.Errorf("compose OGS group live projection: %w", err)
	}
	return StudentTestModule{
			NewStudentPhotos: newStudentPhotos, NewPickupAdjustments: newPickupAdjustmentsFor,
			CareLifecycle: careLifecycleService, PartialAbsence: partialAbsenceService, CareRequests: careRequestService,
			OfferingChanges: offeringChanges, PickupAdjustments: pickupAdjustments, ExcusedRequests: excusedRequestService,
			MasterDataReview: masterDataDecisions, ParentRequests: parentRequestCoordinator,
		}, live, StudentTestPeople{PeopleDirectory: persons, StudentAudit: studentAuditService, StudentPhotos: studentPhotoService},
		auditCommand, enrollmentDecisionService, ogsGroupLiveService, nil
}

// StudentTestStatusDayOverviewPeople serves the absence overview's people
// reads from the live graph's users service.
func StudentTestStatusDayOverviewPeople(live ActiveTestModule) presenceservice.StatusDayOverviewPeople {
	return StatusDayOverviewPeople(live.Users)
}

// HistorySlots supplies the same owner projection to narrow student route fixtures.
func (m StudentTestModule) HistorySlots(records timetablePorts.AttendanceHistoryRecords) presenceservice.HistorySlotReader {
	return NewHistorySlots(records)
}
