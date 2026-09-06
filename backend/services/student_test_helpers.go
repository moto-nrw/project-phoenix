package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/services/active"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/ogsgrouplive"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StudentTestModule struct {
	ActiveTestModule
	GradeTransitionTestModule
	PeopleDirectory    peopledirectory.Capability
	Audit              auditModels.Command
	Schools            platform.SchoolService
	CareLifecycle      users.CareLifecycleService
	StudentAudit       users.StudentAuditService
	PartialAbsence     schedule.PartialAbsenceService
	EnrollmentDecision enrollment.DecisionService
	CareRequests       schedule.CareScheduleRequestService
	OfferingChanges    enrollment.OfferingChangeRequestService
	PickupAdjustments  enrollment.PickupAdjustmentService
	ExcusedRequests    absence.ExcusedAbsenceRequestService
	MasterDataReview   users.MasterDataReviewService
	ParentRequests     *users.ParentRequestCoordinator
	FamilyProtection   *users.FamilyProtectionService
	OGSGroupLive       ogsgrouplive.Getter
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
	grade, err := NewGradeTransitionTestModule(db, clocks...)
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
	studentConsentService := users.NewStudentConsentService(repos.StudentConsentChange)
	users.WirePersonCareParticipation(usersService, careLifecycleService)
	schedule.WireCareParticipation(careDayService, careLifecycleService)
	approvedOfferings := enrollment.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudents{query: persons})
	pickupBaselines := schedule.NewPickupBaselineServiceWithSettings(repos.StudentPickupSchedule, approvedOfferings, repos.CareOffering, settingsService)
	pickupAutoExcusal := schedule.NewPickupAutoExcusalSyncer(repos.StudentPickupException, pickupBaselines, repos.InstanceStudent, db)
	rosterReconciler := schedule.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, logger, now)
	pillEmitter := parentmessaging.NewEmitter(
		db,
		repos.ParentMessageThread,
		repos.ParentMessage,
		settingsService,
		realtimeHub,
		logger.With("service", "parent-events"),
	)
	partialAbsenceService := schedule.NewPartialAbsenceService(
		repos.StudentPickupException,
		repos.StudentStatusDay,
		repos.ExcusedAbsenceRequest,
		repos.InstanceStudent,
		pickupAutoExcusal,
		db,
	)
	enrollmentDecisionService := enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		Requests:               repos.Enrollment(),
		Children:               repos.Enrollment(),
		Guardians:              repos.Enrollment(),
		LateInviteRepo:         repos.Enrollment(),
		ApprovedOfferings:      approvedOfferings,
		CareOfferingRepo:       repos.CareOffering,
		Phases:                 repos.Enrollment(),
		Schemas:                repos.Enrollment(),
		DataAccessLogRepo:      repos.DataAccessLog,
		OfferingAdjustmentRepo: repos.EnrollmentOfferingAdjustment,
		RestorationAuditRepo:   repos.EnrollmentRestorationAudit,
		SchoolRepo:             repos.School,
		PersonRepo:             repos.Person,
		StaffRepo:              repos.Staff,
		StudentRepo:            repos.Student,
		StudentGuardianRepo:    repos.StudentGuardian,
		GuardianFinancialAudit: repos.GuardianFinancialChange,
		GuardianProfileRepo:    repos.GuardianProfile,
		GuardianPhoneRepo:      repos.GuardianPhoneNumber,
		PickupScheduleRepo:     repos.StudentPickupSchedule,
		PickupBaselines:        pickupBaselines,
		ArrivalScheduleRepo:    repos.StudentArrivalSchedule,
		StudentEnrollmentRepo:  repos.StudentEnrollment,
		ActivityGroupRepo:      repos.ActivityGroup,
		ActivityScheduleRepo:   repos.ActivitySchedule,
		CalendarPeriodRepo:     repos.CalendarPeriod,
		TimeframeRepo:          repos.Timeframe,
		ActivityExceptionRepo:  repos.ActivityException,
		AccountRepo:            repos.Account,
		AccountTenantRepo:      repos.AccountTenant,
		AccountRoleRepo:        repos.AccountRole,
		RoleRepo:               repos.Role,
		OutboxEnqueuer:         emailOutboxService,
		StudentAudit:           studentAuditService,
		StudentConsents:        studentConsentService,
		CareWithdrawal:         careLifecycleService,
		Broadcaster:            realtimeHub,
		PickupGuardianNotifier: pillEmitter,
		FrontendURL:            frontendURL,
		ParentsURL:             parentsURL,
		Settings:               settingsService,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		InstanceRosters: rosterReconciler,
		ResyncPickupAutoExcusals: func(ctx context.Context, studentIDs []int64) error {
			return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
				for _, studentID := range studentIDs {
					if err := pickupAutoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
						return err
					}
				}
				return nil
			})
		},
		LockPickupStudents: func(ctx context.Context, studentIDs []int64) error {
			for _, studentID := range studentIDs {
				if err := schedule.LockCareStudent(ctx, db, studentID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
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
	studentService := users.NewStudentService(
		repos.Student,
		repos.PrivacyConsent,
		repos.StudentCompanion,
		studentAuditService,
	)
	studentDeletionService := users.NewStudentDeletionService(
		studentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		repos.DataDeletion,
		repos.StudentDeletionAudit,
		feedbackCounter,
		db,
	)
	pillEmitter.SetTenantRuntime(unit)
	pillEmitter.WithDecisionNotifications(delivery.Notifications, delivery.NotificationPreferences)
	users.WireStudentDocumentCleanup(studentDeletionService, repos.StudentDocument)
	users.WireStudentDeletionCareWithdrawals(studentDeletionService, repos.CareWithdrawal)
	users.WireCareWithdrawalDeletion(careLifecycleService, studentDeletionService)
	grade.GradeTransition.SetOfferingSourceResyncer(enrollmentDecisionService.(education.OfferingSourceResyncer))
	enrollmentDecisionApplier := enrollmentDecisionService.(enrollment.ChangeRequestDecisionApplier)
	directOfferingApplier := enrollmentDecisionService.(enrollment.DirectOfferingAdjustmentApplier)
	requestReviewPolicy := usercontext.NewParentRequestReviewPolicy(
		settingsService,
		userContextService,
		configModels.KeyParentRequestGroupLeaderReviewEnabled,
	)
	parentRequestEvents := users.NewParentRequestEventRecorder(repos.ParentRequestEvent)
	careRequestService := schedule.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		repos.CareScheduleChangeRequest,
		repos.Student,
		repos.Person,
		arrivalScheduleService,
		pickupScheduleService,
		repos.StudentPickupException,
		repos.Attendance,
		pickupAutoExcusal,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
	)
	offeringChangeRequestService := enrollment.NewOfferingChangeRequestServiceWithPolicy(enrollment.OfferingChangeRequestServiceConfig{
		ChangeRepo:             repos.OfferingChangeRequest,
		Children:               repos.Enrollment(),
		Requests:               repos.Enrollment(),
		Phases:                 repos.Enrollment(),
		CareOfferingRepo:       repos.CareOffering,
		ImpactRepo:             manualPlanningReader{db: db},
		StudentRepo:            repos.Student,
		PersonRepo:             repos.Person,
		CareWithdrawalRepo:     repos.CareWithdrawal,
		OfferingAdjustmentRepo: repos.EnrollmentOfferingAdjustment,
		UserContext:            userContextService,
		Applier:                enrollmentDecisionApplier,
		DirectApplier:          directOfferingApplier,
		Settings:               settingsService,
		Emitter:                pillEmitter,
		Logger:                 logger.With("service", "offering-change-requests"),
		Today:                  today,
		EventRecorder:          parentRequestEvents,
	}, requestReviewPolicy)
	pickupOfferingCoordinator := offeringChangeRequestService.(enrollment.DirectOfferingAdjustmentCoordinator)
	pickupAdjustmentService := enrollment.NewPickupAdjustmentService(enrollment.PickupAdjustmentServiceConfig{
		PickupSchedules:     pickupScheduleService,
		ArrivalSchedules:    arrivalScheduleService,
		PickupScheduleRepo:  repos.StudentPickupSchedule,
		ArrivalScheduleRepo: repos.StudentArrivalSchedule,
		PickupBaselines:     pickupBaselines,
		Offerings:           pickupOfferingCoordinator,
		Settings:            settingsService,
		Audit:               studentAuditService,
		Students:            repos.Student,
		DB:                  db,
		Today:               today,
	})
	excusedRequestService := absence.NewExcusedAbsenceRequestServiceWithPolicy(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "excused-requests"),
		db,
	)
	masterDataReviewService := users.NewMasterDataReviewServiceWithAuditAndPolicy(
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
	parentRequestCoordinator := users.NewParentRequestCoordinator(
		masterDataReviewService.(users.MasterDataBulkReviewPort),
		excusedRequestService,
	)
	parentRequestCoordinator.SetMasterDataConflictPort(masterDataReviewService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetExcusedConflictPort(excusedRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetCareConflictPort(careRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetOfferingConflictPort(offeringChangeRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetEventRecorder(parentRequestEvents)
	familyProtectionService := users.NewFamilyProtectionService(repos.FamilyProtection, repos.Student)
	substitutionService := education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: contextRepos.Substitutions, Persons: newEducationPersonQuery(persons),
		Teachers: repos.Teacher, Staff: repos.Staff, Actors: substitutionActorResolver{identity: userContextService},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   repos.SubstitutionChange, DB: db, Broadcaster: realtimeHub,
		Logger: logger.With("service", "substitution"),
		Schedule: newScheduleSubstitutionBridge(schedule.NewSubstitutionAdapter(schedule.SubstitutionAdapterDependencies{
			Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
			Staff: repos.Staff, Engine: instanceService, Broadcaster: realtimeHub,
			Logger: logger.With("service", "schedule-substitution"),
		})),
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
	studentStatusDayService := active.NewStudentStatusDayServiceWithPartialAbsences(
		repos.StudentStatusDay,
		repos.StudentPickupException,
		db,
		now,
	)
	ogsGroupLiveService := ogsgrouplive.NewService(ogsgrouplive.Dependencies{
		People:            usersService,
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       userContextService,
		Active:            activeService,
		Settings:          settingsService,
		Pickups:           pickupScheduleService,
		Arrivals:          arrivalScheduleService,
		Instances:         instanceService,
		CareDays:          careDayService,
		CareParticipation: careLifecycleService,
		ExcusedRequests:   excusedRequestService,
		StatusDays:        studentStatusDayService,
		Logger:            logger.With("service", "ogs-group-live"),
		Now:               now,
	})
	return StudentTestModule{
		ActiveTestModule: live, GradeTransitionTestModule: grade, PeopleDirectory: persons, Audit: auditCommand,
		Schools: platform.NewSchoolService(repos.School), CareLifecycle: careLifecycleService, StudentAudit: studentAuditService,
		PartialAbsence: partialAbsenceService, EnrollmentDecision: enrollmentDecisionService, CareRequests: careRequestService,
		OfferingChanges: offeringChangeRequestService, PickupAdjustments: pickupAdjustmentService, ExcusedRequests: excusedRequestService,
		MasterDataReview: masterDataReviewService, ParentRequests: parentRequestCoordinator, FamilyProtection: familyProtectionService, OGSGroupLive: ogsGroupLiveService,
	}, nil
}
