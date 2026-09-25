package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StudentTestModule struct {
	ActiveTestModule
	GradeTransitionTestModule
	PeopleDirectory    peopledirectory.Capability
	Audit              auditModels.Command
	Schools            organizationtenancy.Capability
	CareLifecycle      careplan.CareLifecycle
	StudentAudit       users.StudentAuditService
	PartialAbsence     careplan.PartialAbsenceService
	EnrollmentDecision enrollment.DecisionService
	CareRequests       carerequests.Service
	OfferingChanges    careplan.OfferingChangeCapability
	PickupAdjustments  careplan.PickupAdjustments
	ExcusedRequests    careplan.ExcusedAbsenceRequests
	MasterDataReview   users.MasterDataReviewService
	ParentRequests     *users.ParentRequestCoordinator
	OGSGroupLive       grouplive.Query
	StudentPhotos      users.StudentPhotoService
	// NewStudentPhotos rebinds the photo lifecycle to the caller's broadcaster
	// and file cleanup. Adapter tests assert on both, and the stored files are
	// an api-layer concern this graph cannot supply.
	NewStudentPhotos func(PhotoBroadcaster, users.PhotoUnlinker) users.StudentPhotoService
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

func NewStudentTestModule(db *bun.DB, unit tenant.UnitOfWork, feedbackCounter users.FeedbackEntryCounter, clocks ...func() time.Time) (StudentTestModule, error) {
	auditCommand, err := auditService.NewCommand(repositories.NewTestAuditStore(db), func(auditService.AppendObservation) {})
	if err != nil {
		return StudentTestModule{}, err
	}
	repos, err := repositories.NewStudentTestRepositories(db, auditCommand)
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
	var offeringResync education.OfferingSourceResyncer
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
	contextRepos, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return StudentTestModule{}, err
	}
	logger := slog.Default()
	now := optionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	settingsService := live.Settings
	usersService := live.Users
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
	frontendURL := currentFactoryConfig().FrontendURL
	parentsURL := currentFactoryConfig().ParentsURL
	studentConsentService := repositories.NewStudentConsents(db)
	// The stored files live in the API layer, so a services-only graph binds
	// no unlinker: the runtime skips the cleanup instead of guessing a path.
	newStudentPhotos := func(broadcaster PhotoBroadcaster, unlinker users.PhotoUnlinker) users.StudentPhotoService {
		return NewStudentPhotos(persons, guardian.PhotoRuntime, StudentPhotoRuntimeDependencies{
			Settings: settingsService, Broadcaster: broadcaster, Unlinker: unlinker,
			Consents: studentConsentService, Logger: logger,
		})
	}
	studentPhotoService := newStudentPhotos(realtimeHub, nil)
	users.WirePersonCareParticipation(usersService, careParticipationResolver(careLifecycleService))
	careplanCompose.WireCareParticipation(careDayService, careLifecycleService)
	approvedOfferings := enrollmentCompose.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudents{query: persons})
	pickupBaselines, err := careplanCompose.NewPickupBaselines(repos.CarePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db, Records: repos.CarePlan, Baselines: pickupBaselines, Blocks: newStudentPresence(db, logger), Preview: newPickupExcusalTimetable(repos.Timetable, db),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	rosterReconciler := timetableCompose.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, logger, now)
	recurrenceLock, err := repositories.NewTimetableRecurrenceLock(db)
	if err != nil {
		return StudentTestModule{}, err
	}
	pillEmitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		DB:          db,
		Runtime:     unit,
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		Settings:    settingsService,
		Broadcaster: realtimeHub,
		Logger:      logger.With("service", "parent-events"),
	})
	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db, repos.CarePlan, newStudentPresence(db, logger), pickupAutoExcusal)
	if err != nil {
		return StudentTestModule{}, err
	}
	guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return StudentTestModule{}, err
	}
	offeringLinks, err := NewCareOfferingCatalogTestModule(db, unit, CareOfferingCatalogTestOptions{
		Settings: settingsService, LockRecurrence: recurrenceLock.LockRecurrenceWrites,
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	resyncPickupAutoExcusals := func(ctx context.Context, studentIDs []int64) error {
		return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			for _, studentID := range studentIDs {
				if err := pickupAutoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
					return err
				}
			}
			return nil
		})
	}
	careBookings, err := newBookingMaterialization(bookingMaterializationInputs{
		Catalog: offeringLinks.Catalog, Timetable: repos.Timetable, Rosters: rosterReconciler, Students: persons,
		Periods: repos.SchoolCalendar(), Enrollment: repos.Enrollment(), Approved: approvedOfferings,
		Settings: settingsService, Bookings: repos.CarePlan, Withdrawals: careLifecycleService,
		Adjustments: repos.EnrollmentOfferingAdjustment, Persons: repos.Person, Accounts: guardianAccess,
		Pickup: pickupBaselines, PickupRows: repos.CarePlan,
		LockRecurrence:           recurrenceLock.LockRecurrenceWrites,
		ResyncPickupAutoExcusals: resyncPickupAutoExcusals,
		Broadcaster:              realtimeHub,
		GuardianNotifier:         pillEmitter,
		Today:                    today,
		Logger:                   logger.With("service", "care-plan-bookings"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	enrollmentDecisionService := enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		Requests:                  repos.Enrollment(),
		Children:                  repos.Enrollment(),
		Guardians:                 repos.Enrollment(),
		LateInviteRepo:            repos.Enrollment(),
		CareOfferingRepo:          enrollment.NewCareOfferingRepository(repos.CarePlan),
		Phases:                    repos.Enrollment(),
		Schemas:                   repos.Enrollment(),
		DataAccessLogRepo:         repos.DataAccessLog,
		OfferingAdjustmentRepo:    repos.EnrollmentOfferingAdjustment,
		RestorationAuditRepo:      repos.EnrollmentRestorationAudit,
		Notifications:             newEnrollmentNotifications(repos.Enrollment(), settingsService, outboxEnqueuer{outbox: emailOutboxService}, enrollmentSchoolDirectory{schools: repos.School}),
		PersonRepo:                repos.Person,
		StaffRepo:                 repos.Staff,
		StudentRepo:               repos.Student,
		StudentGuardianRepo:       repos.StudentGuardian,
		GuardianFinancialAudit:    repos.GuardianFinancialChange,
		GuardianProfileRepo:       repos.GuardianProfile,
		GuardianPhoneRepo:         repos.GuardianPhoneNumber,
		PickupScheduleRepo:        repos.StudentPickupSchedule,
		ArrivalScheduleRepo:       repos.StudentArrivalSchedule,
		CareBookings:              careBookings,
		GuardianAccess:            guardianAccess,
		StudentEnrollment:         persons,
		DepartureCompanions:       repositories.NewStudentCompanionRepository(repos.CarePlan),
		DeleteDepartureCompanions: repos.CarePlan.DeleteCompanionEdges,
		OutboxEnqueuer:            outboxEnqueuer{outbox: emailOutboxService},
		StudentAudit:              studentAuditService,
		StudentConsents:           studentConsentService,
		CareWithdrawal:            careLifecycleService,
		Broadcaster:               realtimeHub,
		FrontendURL:               frontendURL,
		ParentsURL:                parentsURL,
		Settings:                  settingsService,
		LockTemplateRecurrence:    recurrenceLock.LockRecurrenceWrites,
		ResyncPickupAutoExcusals:  resyncPickupAutoExcusals,
		LockPickupStudents: func(ctx context.Context, studentIDs []int64) error {
			for _, studentID := range studentIDs {
				if err := persons.LockStudent(ctx, studentID); err != nil {
					if errors.Is(err, peopledirectory.ErrStudentNotFound) {
						continue
					}
					return err
				}
			}
			return nil
		},
		Logger: logger.With("service", "enrollment-decision"),
		Today:  today,
	})
	offeringResync = careBookings
	requestReviewPolicy := NewParentRequestReviewPolicy(userContextService.Caller().ParentRequestReviews)
	parentRequestEvents := users.NewParentRequestEventRecorder(repos.ParentRequestEvent)
	careRequestService := NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		repos.CarePlan,
		persons,
		arrivalScheduleService,
		pickupScheduleService,
		newStudentPresence(db, logger),
		pickupAutoExcusal,
		repos.CarePlan,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
		WithCareRequestToday(today),
	)
	offeringChanges, err := newOfferingChanges(offeringChangeInputs{
		CarePlan: repos.CarePlan, Enrollment: repos.Enrollment(), Students: repos.Student,
		Withdrawals: repos.CareWithdrawal, Settings: settingsService,
		Planning: manualPlanningReader{db: db, courseGroups: repos.Timetable},
		Bookings: careBookings, Reviews: requestReviewPolicy, Emitter: pillEmitter,
		Events: parentRequestEvents, Today: today, Logger: logger.With("service", "offering-change-requests"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	newPickupAdjustmentsFor := func(offerings careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error) {
		return newPickupAdjustments(pickupAdjustmentInputs{
			CarePlan: repos.CarePlan, PickupSchedules: pickupScheduleService, ArrivalSchedules: arrivalScheduleService,
			Baselines: pickupBaselines, Offerings: offerings, Settings: settingsService,
			Audit: studentAuditService, Students: repos.Student, Today: today,
		})
	}
	pickupAdjustments, err := newPickupAdjustmentsFor(offeringChanges)
	if err != nil {
		return StudentTestModule{}, err
	}
	excusedRequestService, err := newExcusedAbsenceRequests(excusedRequestWiring{
		carePlan: repos.CarePlan, students: repos.Student, persons: repos.Person,
		scope:   parentRequestReviewScope(requestReviewPolicy),
		emitter: pillEmitter, broadcaster: realtimeHub, events: parentRequestEvents,
		logger: logger.With("service", "excused-requests"),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	masterDataReviewService := users.NewMasterDataReviewServiceWithAuditAndPolicy(authjwt.PermissionsFromCtx,
		repos.StudentDataChangeRequest,
		repos.Student,
		repos.Person,
		userContextService,
		pillEmitter,
		studentAuditService,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "master-data-review"),
		realtimeHub,
	)
	excusedCoordinatorPort := excusedRequestCoordinatorPort{requests: excusedRequestService}
	parentRequestCoordinator := users.NewParentRequestCoordinator(authjwt.PermissionsFromCtx,
		masterDataReviewService.(users.MasterDataBulkReviewPort),
		excusedCoordinatorPort,
	)
	parentRequestCoordinator.SetMasterDataConflictPort(masterDataReviewService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetExcusedConflictPort(excusedCoordinatorPort)
	parentRequestCoordinator.SetCareConflictPort(careRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetOfferingConflictPort(offeringChangeConflictPort{changes: offeringChanges})
	parentRequestCoordinator.SetEventRecorder(parentRequestEvents)
	scheduleSubstitution, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: live.Deviations, Staff: repos.Staff, Broadcaster: realtimeHub, Logger: logger.With("service", "schedule-substitution"),
		ActivityInstances: repositories.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:     repositories.NewTimetableInstanceStaffReads(repos.InstanceStaff),
	})
	if err != nil {
		return StudentTestModule{}, err
	}
	substitutionService := education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: contextRepos.Substitutions, Persons: newEducationPersonQuery(persons),
		Teachers: repos.Teacher, Staff: repos.Staff, Actors: substitutionActorResolver{identity: userContextService.Caller()},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   repos.SubstitutionChange, DB: db, Broadcaster: realtimeHub,
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
		db,
		repos.CarePlan.LockExceptionDay,
		now,
	)
	ogsGroupLiveService, err := grouplivelegacy.New(grouplivelegacy.Sources{
		Presence:          newStudentPresence(db, logger),
		People:            usersService,
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       groupLiveCaller{userContextService},
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
		MasterDataReview: masterDataReviewService, ParentRequests: parentRequestCoordinator, OGSGroupLive: ogsGroupLiveService,
	}, nil
}

// StatusDayOverviewPeople serves the absence overview's people reads from the
// module's users service.
func (m StudentTestModule) StatusDayOverviewPeople() presenceservice.StatusDayOverviewPeople {
	return StatusDayOverviewPeople(m.Users)
}

// HistorySlots supplies the same owner projection to narrow student route fixtures.
func (m StudentTestModule) HistorySlots(records timetableCompose.AttendanceHistoryRecords) presenceservice.HistorySlotReader {
	return NewHistorySlots(records)
}
