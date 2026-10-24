package application

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/adapters/jobruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// mustTenantRuntime binds a composed tenant runtime through the Worker's
// production adapter.
func mustTenantRuntime(runtime jobruntime.Runtime, err error) ports.TenantRuntime {
	if err != nil {
		panic(err)
	}
	return runtime
}

// unitTenantRuntime is the database-less runtime of the test support.
func unitTenantRuntime() ports.TenantRuntime {
	return mustTenantRuntime(jobruntime.New(testpkg.PassthroughTenantRuntime()))
}

// scriptedTenantRuntime is the database-less runtime with a scripted tenant
// transaction and retry decision.
func scriptedTenantRuntime(withinTenant func(context.Context, int64, func(context.Context, any) error) error, retryable func(error) bool) ports.TenantRuntime {
	return mustTenantRuntime(jobruntime.New(testpkg.ScriptedTenantRuntime(withinTenant, retryable)))
}

// dbTenantRuntime is the production tenant runtime against the test database.
func dbTenantRuntime(t *testing.T, db *bun.DB) ports.TenantRuntime {
	t.Helper()
	return mustTenantRuntime(jobruntime.New(testpkg.TenantRuntime(t, db)))
}

const schedulerUnitTenantID int64 = 1

func testEnv(values ...string) func(string) string {
	if len(values)%2 != 0 {
		panic("testEnv requires key/value pairs")
	}
	env := make(map[string]string, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		env[values[i]] = values[i+1]
	}
	return func(key string) string { return env[key] }
}

func TestStopCancelsRunningTaskContexts(t *testing.T) {
	t.Parallel()

	scheduler := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
	ctx, cancel := scheduler.taskContext(scheduler.lifecycleContext(), time.Hour)
	defer cancel()

	stopped := make(chan struct{})
	scheduler.wg.Go(func() {

		<-ctx.Done()
		close(stopped)
	})

	scheduler.Stop()

	require.ErrorIs(t, ctx.Err(), context.Canceled)
	select {
	case <-stopped:
	default:
		t.Fatal("scheduler stopped before its task context was cancelled")
	}
}

func newUnitScheduler(
	activeService ports.SessionMaintenance,
	cleanupService ports.PresenceCleanup,
	authService ports.AuthCleanup,
	invitationService ports.InvitationCleaner,
	emailChangeCleaner ports.EmailChangeTokenCleaner,
	operatorInvitationCleaner ports.OperatorInvitationCleaner,
	logger *slog.Logger,
) *Scheduler {
	scheduler := newScheduler(ports.WorkerDependencies{
		Logger:                    logger,
		TenantRuntime:             unitTenantRuntime(),
		Active:                    activeService,
		ActiveCleanup:             cleanupService,
		AuthCleanup:               authService,
		InvitationCleanup:         invitationService,
		EmailChangeCleanup:        emailChangeCleaner,
		OperatorInvitationCleanup: operatorInvitationCleaner,
		FeedbackCleaner:           &fakeFeedbackCleaner{},
	})
	jobs := scheduler.jobDefinitions()
	required := make([]ports.JobID, 0, len(jobs))
	for _, job := range jobs {
		required = append(required, job.ID())
	}
	registry, err := NewRegistry(required, jobs...)
	if err != nil {
		panic(err)
	}
	scheduler.registry = registry
	scheduler.minuteSnapshotLoader = func(context.Context) (*schedulerMinuteSnapshot, error) {
		return &schedulerMinuteSnapshot{tenantIDs: []int64{schedulerUnitTenantID}}, errSchedulerSettingsBatchUnsupported
	}
	scheduler.allTenantIDsLoader = func(context.Context) ([]int64, error) {
		return []int64{schedulerUnitTenantID}, nil
	}
	return scheduler
}

// unitSchedulerDependencies are the construction inputs of a partial worker fixture.
type unitSchedulerDependencies struct {
	activeService                 ports.SessionMaintenance
	allTenantIDsLoader            func(context.Context) ([]int64, error)
	announcementReminderScannedAt map[int64]time.Time
	announcementReminders         ports.ParentAnnouncementReminderCommand
	appointmentReminderScannedAt  map[int64]time.Time
	appointmentReminders          ports.AppointmentReminderCommand
	authCleanup                   ports.AuthCleanup
	autoCheckouter                ports.AutoCheckouter
	autoEnd                       ports.InstanceAutoEnd
	autoStart                     ports.InstanceAutoStart
	bookingConsistency            ports.BookingConsistencyAudit
	mealOrders                    ports.MealOrderSettlement
	breakAutoEnder                ports.BreakAutoEnder
	calendarFeedCleanup           ports.CalendarFeedCleaner
	careExitEffector              ports.CareExitEffector
	cleanupClock                  func() time.Time
	cleanupJobs                   []CleanupJob
	cleanupRuns                   ports.CleanupRunLedger
	cleanupService                ports.PresenceCleanup
	done                          chan struct{}
	enrollmentRejectedCleanup     ports.RejectedEnrollmentCleaner
	feedbackCleaner               ports.FeedbackCleaner
	fileStoreCleaner              ports.FileStoreCleaner
	getenv                        func(string) string
	instanceRepo                  ports.DayInstanceReader
	instanceRoomRepo              ports.ExistingRoomReader
	instanceStudentRepo           ports.InstanceCheckoutCloser
	invitationCleanup             ports.InvitationCleaner
	lifecycleCtx                  context.Context
	logger                        *slog.Logger
	materializationNow            func() time.Time
	materializer                  ports.TimetableMaterializer
	minuteSnapshotLoader          func(context.Context) (*schedulerMinuteSnapshot, error)
	minuteSnapshotNow             func() time.Time
	now                           func() time.Time
	outboxWorker                  ports.OutboxWorkerRunner
	overdueBroadcaster            ports.InstanceOverdueBroadcaster
	pwaUsageCleanup               ports.PWAUsageCleanup
	registry                      *Registry
	reminderNotifications         ports.ReminderNotificationTick
	retention                     RetentionRun
	rolloverDeadlineRunner        ports.RolloverDeadlineRunner
	schoolRepo                    ports.TenantDirectory
	settings                      ports.SettingsResolver
	staffDocumentFileCleaner      ports.StaffDocumentFileCleaner
	staffMessageCleanup           ports.StaffMessageCleanup
	stopLifecycle                 context.CancelFunc
	studentChangeLogCleanup       ports.StudentChangeLogCleanup
	studentDocumentFileCleaner    ports.StudentDocumentFileCleaner
	studentLifecycleAudit         ports.StudentLifecycleAuditor
	studentLifecycleRepo          ports.StudentLifecycleRepository
	studentStatusDayRepo          ports.StudentStatusFlagArchiver
	tasks                         map[string]*ScheduledTask
	tenantRuntime                 ports.TenantRuntime
	tenantRuntimeObserver         func(entryPoint, outcome string)
	timeTrackingCleanup           ports.TimeTrackingCleanupService
	timetableBridge               ports.TimetableBridgeCompleter
	timetableCleanup              ports.TimetableCleanup
	unitOfWorkObserver            func(entryPoint, kind, result string, duration time.Duration, retries int)
	workSessionCleanup            ports.WorkSessionCleaner
	workerTracer                  ports.WorkerTracer
}

func unitScheduler(input *unitSchedulerDependencies) *Scheduler {
	configured := newUnitScheduler(nil, nil, nil, nil, nil, nil, input.logger)
	runtime := input.tenantRuntime
	if runtime == nil {
		runtime = configured.tenantRuntime
	}
	snapshotLoader := input.minuteSnapshotLoader
	if snapshotLoader == nil && input.schoolRepo == nil {
		snapshotLoader = configured.minuteSnapshotLoader
	}
	tenantLoader := input.allTenantIDsLoader
	if tenantLoader == nil && input.schoolRepo == nil {
		tenantLoader = configured.allTenantIDsLoader
	}
	registry := input.registry
	if registry == nil {
		registry = configured.registry
	}
	feedbackCleaner := input.feedbackCleaner
	if feedbackCleaner == nil {
		feedbackCleaner = configured.feedbackCleaner
	}
	return &Scheduler{
		activeService:                 input.activeService,
		allTenantIDsLoader:            tenantLoader,
		announcementReminderScannedAt: input.announcementReminderScannedAt,
		announcementReminders:         input.announcementReminders,
		appointmentReminderScannedAt:  input.appointmentReminderScannedAt,
		appointmentReminders:          input.appointmentReminders,
		authCleanup:                   input.authCleanup,
		autoCheckouter:                input.autoCheckouter,
		autoEnd:                       input.autoEnd,
		autoStart:                     input.autoStart,
		bookingConsistency:            input.bookingConsistency,
		mealOrders:                    input.mealOrders,
		breakAutoEnder:                input.breakAutoEnder,
		calendarFeedCleanup:           input.calendarFeedCleanup,
		careExitEffector:              input.careExitEffector,
		cleanupClock:                  input.cleanupClock,
		cleanupJobs:                   input.cleanupJobs,
		cleanupRuns:                   input.cleanupRuns,
		cleanupService:                input.cleanupService,
		schedulerLifecycle:            schedulerLifecycle{done: input.done, lifecycleCtx: input.lifecycleCtx, stopLifecycle: input.stopLifecycle},
		enrollmentRejectedCleanup:     input.enrollmentRejectedCleanup,
		feedbackCleaner:               feedbackCleaner,
		fileStoreCleaner:              input.fileStoreCleaner,
		getenv:                        input.getenv,
		instanceRepo:                  input.instanceRepo,
		instanceRoomRepo:              input.instanceRoomRepo,
		instanceStudentRepo:           input.instanceStudentRepo,
		invitationCleanup:             input.invitationCleanup,
		logger:                        input.logger,
		materializationNow:            input.materializationNow,
		materializer:                  input.materializer,
		minuteSnapshotLoader:          snapshotLoader,
		cleanupMinuteSnapshotLoader:   snapshotLoader,
		minuteSnapshotNow:             input.minuteSnapshotNow,
		now:                           input.now,
		outboxWorker:                  input.outboxWorker,
		overdueBroadcaster:            input.overdueBroadcaster,
		pwaUsageCleanup:               input.pwaUsageCleanup,
		registry:                      registry,
		reminderNotifications:         input.reminderNotifications,
		retention:                     input.retention,
		rolloverDeadlineRunner:        input.rolloverDeadlineRunner,
		schoolRepo:                    input.schoolRepo,
		settings:                      input.settings,
		staffDocumentFileCleaner:      input.staffDocumentFileCleaner,
		staffMessageCleanup:           input.staffMessageCleanup,
		studentChangeLogCleanup:       input.studentChangeLogCleanup,
		studentDocumentFileCleaner:    input.studentDocumentFileCleaner,
		studentLifecycleAudit:         input.studentLifecycleAudit,
		studentLifecycleRepo:          input.studentLifecycleRepo,
		studentStatusDayRepo:          input.studentStatusDayRepo,
		tasks:                         input.tasks,
		tenantRuntime:                 runtime,
		tenantRuntimeObserver:         input.tenantRuntimeObserver,
		timeTrackingCleanup:           input.timeTrackingCleanup,
		timetableBridge:               input.timetableBridge,
		timetableCleanup:              input.timetableCleanup,
		unitOfWorkObserver:            input.unitOfWorkObserver,
		workSessionCleanup:            input.workSessionCleanup,
		workerTracer:                  input.workerTracer,
	}
}
