package scheduler

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	pwaSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	staffMessagingSvc "github.com/moto-nrw/project-phoenix/services/staffmessaging"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// AuthCleanup exposes the cleanup routines required from the auth service.
type AuthCleanup interface {
	CleanupExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	CleanupExpiredRateLimits(ctx context.Context) (int, error)
}

// InvitationCleaner exposes the cleanup routine required from the invitation service.
type InvitationCleaner interface {
	CleanupExpiredInvitations(ctx context.Context) (int, error)
}

// CleanupJob represents a single cleanup task that can be executed.
type CleanupJob struct {
	Description string
	Run         func(context.Context) (int, error)
}

// WorkerTracer is injected by the Worker composition root. The scheduler owns
// job names and outcomes; the adapter owns correlation, logs, and metrics.
type WorkerTracer struct {
	StartJob func(context.Context, string) (context.Context, error)
	Logger   func(context.Context) *slog.Logger
	Failure  func(context.Context, string, string, error)
	Run      func(JobID, string, time.Duration)
}

var errWorkerTraceStart = errors.New("start worker trace")

// WorkSessionCleaner exposes the cleanup routine for stale work sessions.
type WorkSessionCleaner interface {
	CleanupOpenSessions(ctx context.Context) (int, error)
}

// BreakAutoEnder exposes the method to auto-end expired breaks.
type BreakAutoEnder interface {
	AutoEndExpiredBreaks(ctx context.Context) (int, error)
}

// AutoCheckouter exposes the method to close open work sessions at their
// planned shift end (#1798).
type AutoCheckouter interface {
	AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error)
}

// EmailChangeTokenCleaner exposes the cleanup routine for email change tokens.
type EmailChangeTokenCleaner interface {
	CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error)
}

// OperatorInvitationCleaner exposes the cleanup routine for operator invitation tokens.
type OperatorInvitationCleaner interface {
	CleanupExpiredOperatorInvitations(ctx context.Context) (int, error)
}

type CalendarFeedCleaner interface {
	CleanupExpiredFeedTombstones(ctx context.Context) (int, error)
}

// FeedbackCleaner exposes the cleanup routine for old feedback entries.
type FeedbackCleaner interface {
	DeleteExpired(ctx context.Context) (int, error)
}

type UnregisteredTagScanCleaner interface {
	DeleteOlderThan(ctx context.Context, days int) (int, error)
}

// StaffDocumentFileCleaner removes files whose staff-document metadata either
// never committed or belongs to an offboarded staff member.
type StaffDocumentFileCleaner interface {
	CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error)
}

// StudentDocumentFileCleaner removes objects whose child-document metadata
// either never committed or was cascaded away by a child deletion.
type StudentDocumentFileCleaner interface {
	CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error)
}

// FileStoreCleaner removes objects of the school file storage whose metadata
// either never committed or was cascaded away by a folder deletion (#2596).
type FileStoreCleaner interface {
	CleanupOrphanedFiles(ctx context.Context) (int, error)
}

// SettingsResolver resolves setting values per tenant. Implemented by config.SettingsService.
type SettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService              active.Service
	cleanupService             active.CleanupService
	authCleanup                AuthCleanup
	invitationCleanup          InvitationCleaner
	workSessionCleanup         WorkSessionCleaner
	breakAutoEnder             BreakAutoEnder
	autoCheckouter             AutoCheckouter
	feedbackCleaner            FeedbackCleaner
	unregisteredTagScanCleaner UnregisteredTagScanCleaner
	staffDocumentFileCleaner   StaffDocumentFileCleaner
	studentDocumentFileCleaner StudentDocumentFileCleaner
	fileStoreCleaner           FileStoreCleaner
	materializer               scheduleSvc.MaterializationService
	timetableCleanup           scheduleSvc.TimetableCleanupService
	calendarFeedCleanup        CalendarFeedCleaner
	timeTrackingCleanup        active.TimeTrackingCleanupService
	studentChangeLogCleanup    usersSvc.StudentChangeLogCleanupService
	pwaUsageCleanup            pwaSvc.UsageService
	staffMessageCleanup        staffMessagingSvc.CleanupService
	bookingConsistency         auditModel.BookingConsistencyRepository
	enrollmentRejectedCleanup  enrollmentSvc.RejectedEnrollmentCleaner
	autoStart                  scheduleSvc.AutoStartService
	autoEnd                    scheduleSvc.AutoEndService
	settings                   SettingsResolver
	db                         *bun.DB
	schoolRepo                 platform.SchoolRepository
	tenantRuntime              tenant.UnitOfWork
	tenantRuntimeConfigured    bool
	tenantRuntimeObserver      func(entryPoint, outcome string)
	unitOfWorkObserver         func(entryPoint, kind, result string, duration time.Duration, retries int)
	workerTracer               WorkerTracer
	minuteSnapshotMu           sync.Mutex
	minuteSnapshotLoad         *schedulerMinuteSnapshotLoad
	minuteSnapshotNow          func() time.Time
	materializationNow         func() time.Time
	minuteSnapshotLoader       func(context.Context) (*schedulerMinuteSnapshot, error)
	allTenantIDsLoader         func(context.Context) ([]int64, error)
	cleanupJobs                []CleanupJob
	registry                   *Registry
	tasks                      map[string]*ScheduledTask
	mu                         sync.RWMutex
	logger                     *slog.Logger
	getenv                     func(string) string
	lifecycleCtx               context.Context
	stopLifecycle              context.CancelFunc
	// done signals goroutines to stop when closed (replaces stored context)
	done chan struct{}
	wg   sync.WaitGroup

	// Session cleanup configuration (parsed once during initialization)
	sessionCleanupIntervalMinutes    int
	sessionAbandonedThresholdMinutes int

	// Break auto-end configuration (parsed once during initialization)
	breakAutoEndIntervalSeconds int

	// Per-tenant tracking for minute-polling (keyed by tenant ID)
	lastSessionEnd              sync.Map // tenant_id → time.Time
	lastDataCleanup             sync.Map // tenant_id → time.Time
	lastSessionCleanup          sync.Map // tenant_id → time.Time
	lastStatusFlagClear         sync.Map // tenant_id → time.Time
	lastMaterialization         sync.Map // tenant_id → time.Time
	lastTimetableCleanup        sync.Map // tenant_id → time.Time (WP-B14)
	lastTimeTrackingCleanup     sync.Map // tenant_id → time.Time (Tranche 0b)
	lastStudentChangeLogCleanup sync.Map // tenant_id → time.Time (issue #1455)
	lastPWAUsageCleanup         sync.Map // tenant_id → time.Time (issue #2189)
	lastStaffMessageCleanup     sync.Map // tenant_id → time.Time (issue #2598)

	// Overdue instance tracking (WP-B9). Re-fire guard so the same instance
	// does not emit `instance_overdue` every minute for the same planned
	// row. Cleared explicitly on day boundary; see checkAndRunOverdue.
	instanceRepo         scheduleModel.ActivityInstanceRepository
	instanceRoomRepo     facilitiesModel.RoomRepository
	instanceStudentRepo  scheduleModel.InstanceStudentRepository
	timetableBridge      TimetableBridgeCompleter
	studentStatusDayRepo activeModel.StudentStatusDayRepository
	overdueBroadcaster   realtime.Broadcaster
	overdueEmitted       sync.Map // overdueKey{tenantID, instanceID} → time.Time
	overdueEmittedDay    timezone.Date
	overdueEmittedDayMu  sync.Mutex

	// Student lifecycle (parent-enrollment PR 2).
	// Nil → activate-students task does not register.
	studentLifecycleRepo  StudentLifecycleRepository
	studentLifecycleAudit StudentLifecycleAuditor
	careExitEffector      CareExitEffector

	// Outbox worker (parent-enrollment PR 5).
	// Nil → outbox task does not register.
	outboxWorker OutboxWorkerRunner

	// Rollover deadline resolver (phase rollover slice 1).
	rolloverDeadlineRunner RolloverDeadlineRunner

	// Personal reminder notifications. Anything missing prevents Worker
	// construction. reminderNotified is the
	// once-per-day re-fire guard (now per person), rotated on civil-date
	// rollover like overdueEmitted. It is process-local, so production must run
	// only one scheduler instance until this state is shared.
	reminderNotifications ReminderNotificationDeps
	reminderNotified      sync.Map // reminderNotificationKey → time.Time
	reminderNotifiedDay   timezone.Date
	reminderNotifiedDayMu sync.Mutex

	// Guardian appointment reminders (#1671).
	// appointmentReminderScannedAt holds the upper bound of each tenant's last
	// successful scan. Failed scans deliberately leave their tenant's boundary
	// untouched so the next tick retries its bounded window.
	appointmentReminders         AppointmentReminderQueuer
	appointmentReminderScannedAt map[int64]time.Time
	appointmentReminderScanMu    sync.Mutex
}

// OutboxWorkerRunner is the narrow contract the scheduler needs from the
// platform outbox worker. Defined here so the scheduler doesn't import
// services/platform.
type OutboxWorkerRunner interface {
	RunOnce(ctx context.Context, batchSize int) (int, error)
	Backlog(ctx context.Context) (int, error)
	SetMaxAttempts(n int)
}

// overdueKey composites tenant + instance so the sync.Map key cannot collide
// across tenants. Using a struct literal avoids the ambiguity of a stringly-
// typed "tenant:instance" key if either id ever contained a colon.
type overdueKey struct {
	tenantID   int64
	instanceID int64
}

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	Name     string
	Schedule string // cron-like schedule or duration
	LastRun  time.Time
	NextRun  time.Time
	Running  bool
	mu       sync.Mutex
}

func newScheduler(deps WorkerDependencies) *Scheduler {
	lifecycleCtx, stopLifecycle := context.WithCancel(context.Background())
	scheduler := &Scheduler{
		tasks:                        make(map[string]*ScheduledTask),
		done:                         make(chan struct{}),
		logger:                       deps.Logger,
		getenv:                       deps.Getenv,
		lifecycleCtx:                 lifecycleCtx,
		stopLifecycle:                stopLifecycle,
		appointmentReminderScannedAt: make(map[int64]time.Time),
	}
	addCleanupDependencies(scheduler, deps)
	addScheduleDependencies(scheduler, deps)
	addRuntimeDependencies(scheduler, deps)
	return scheduler
}

func addCleanupDependencies(scheduler *Scheduler, deps WorkerDependencies) {
	scheduler.cleanupService = deps.ActiveCleanup
	scheduler.authCleanup = deps.AuthCleanup
	scheduler.invitationCleanup = deps.InvitationCleanup
	scheduler.workSessionCleanup = deps.WorkSessionCleanup
	scheduler.feedbackCleaner = deps.FeedbackCleaner
	scheduler.unregisteredTagScanCleaner = deps.UnregisteredScanCleaner
	scheduler.staffDocumentFileCleaner = deps.StaffDocumentCleaner
	scheduler.studentDocumentFileCleaner = deps.StudentDocumentCleaner
	scheduler.fileStoreCleaner = deps.FileStoreCleaner
	scheduler.timetableCleanup = deps.TimetableCleanup
	scheduler.calendarFeedCleanup = deps.CalendarFeedCleanup
	scheduler.timeTrackingCleanup = deps.TimeTrackingCleanup
	scheduler.studentChangeLogCleanup = deps.StudentChangeLogCleanup
	scheduler.pwaUsageCleanup = deps.PWAUsageCleanup
	scheduler.staffMessageCleanup = deps.StaffMessageCleanup
	scheduler.enrollmentRejectedCleanup = deps.EnrollmentRejectedCleanup
	scheduler.cleanupJobs = buildCleanupJobs(deps.AuthCleanup, deps.InvitationCleanup, deps.EmailChangeCleanup, deps.OperatorInvitationCleanup)
}

func addScheduleDependencies(scheduler *Scheduler, deps WorkerDependencies) {
	scheduler.activeService = deps.Active
	scheduler.breakAutoEnder = deps.BreakAutoEnder
	scheduler.autoCheckouter = deps.AutoCheckouter
	scheduler.materializer = deps.Materializer
	scheduler.bookingConsistency = deps.BookingConsistency
	scheduler.autoStart = deps.AutoStart
	scheduler.autoEnd = deps.AutoEnd
	scheduler.instanceRepo = deps.InstanceRepo
	scheduler.instanceRoomRepo = deps.InstanceRoomRepo
	scheduler.instanceStudentRepo = deps.InstanceStudentRepo
	scheduler.timetableBridge = deps.TimetableBridge
	scheduler.studentStatusDayRepo = deps.StudentStatusDayRepo
	scheduler.overdueBroadcaster = deps.OverdueBroadcaster
	scheduler.studentLifecycleRepo = deps.StudentLifecycleRepo
	scheduler.studentLifecycleAudit = deps.StudentLifecycleAudit
	scheduler.careExitEffector = deps.CareExitEffector
	scheduler.outboxWorker = deps.OutboxWorker
	scheduler.rolloverDeadlineRunner = deps.RolloverDeadlineRunner
	scheduler.reminderNotifications = deps.ReminderNotifications
	scheduler.appointmentReminders = deps.AppointmentReminders
}

func addRuntimeDependencies(scheduler *Scheduler, deps WorkerDependencies) {
	scheduler.settings = deps.Settings
	scheduler.db = deps.DB
	scheduler.schoolRepo = deps.SchoolRepo
	scheduler.tenantRuntime = tenantRuntimeValue(deps.TenantRuntime)
	scheduler.tenantRuntimeConfigured = deps.TenantRuntime != nil
	scheduler.tenantRuntimeObserver = deps.TenantRuntimeObserver
	scheduler.unitOfWorkObserver = deps.UnitOfWorkObserver
	scheduler.workerTracer = deps.Tracer
}

func tenantRuntimeValue(runtime *tenant.UnitOfWork) tenant.UnitOfWork {
	if runtime == nil {
		return tenant.UnitOfWork{}
	}
	return *runtime
}

// getLogger returns the scheduler's logger, falling back to slog.Default() if nil.
func (s *Scheduler) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *Scheduler) env(key string) string {
	if s.getenv == nil {
		return ""
	}
	return s.getenv(key)
}

func (s *Scheduler) startWorkerJob(ctx context.Context, operation string) (context.Context, error) {
	if s.workerTracer.StartJob == nil {
		return ctx, nil
	}
	return s.workerTracer.StartJob(ctx, operation)
}

func (s *Scheduler) traceWorkerFailure(ctx context.Context, operation, outcome string, err error) bool {
	if s.workerTracer.Failure != nil {
		s.workerTracer.Failure(ctx, operation, outcome, err)
		return true
	}
	return false
}

func (s *Scheduler) workerLogger(ctx context.Context) *slog.Logger {
	if s.workerTracer.Logger != nil {
		return s.workerTracer.Logger(ctx)
	}
	return s.getLogger()
}

func (s *Scheduler) observeWorkerRun(jobID JobID, outcome string, duration time.Duration) {
	if s.workerTracer.Run != nil {
		s.workerTracer.Run(jobID, outcome, duration)
	}
}

func (s *Scheduler) withUnitOfWork(ctx context.Context) context.Context {
	ctx = tenant.WithUnitOfWork(ctx, s.tenantRuntime)
	if s.unitOfWorkObserver == nil {
		return ctx
	}
	return tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		s.unitOfWorkObserver("worker", string(event.Kind), string(event.Result), event.Duration, event.Retries)
	})
}

func (s *Scheduler) observeTenantRuntime(outcome string) {
	if s.tenantRuntimeObserver != nil {
		s.tenantRuntimeObserver("worker", outcome)
	}
}

// TimetableBridgeCompleter finalizes attendance and completes the schedule-side
// instances of ended active.groups in one step. Implemented by
// schedule.TimetableBridgeService — the same implementation the force-start
// path uses, so both paths leave identical rows behind (#1747).
type TimetableBridgeCompleter interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

// forEachTenant executes fn for each active tenant inside its tenant runtime.
// Missing runtime wiring fails closed before fn can reach a repository.
// Active tenant IDs share the same minute snapshot as settings-aware jobs so
// concurrent polling goroutines do not repeat the platform.schools query.
func (s *Scheduler) forEachTenant(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if !s.tenantRuntimeConfigured || (s.minuteSnapshotLoader == nil && (s.db == nil || s.schoolRepo == nil)) {
		s.observeTenantRuntime("missing_tenant")
		return fmt.Errorf("tenant runtime is not configured for %s", opName)
	}

	minuteSnapshot, err := s.getMinuteSnapshot(ctx)
	if err != nil && (minuteSnapshot == nil || len(minuteSnapshot.tenantIDs) == 0) {
		s.observeTenantRuntime("transaction_failure")
		return fmt.Errorf("load active tenants for %s: %w", opName, err)
	}
	s.forEachKnownTenant(ctx, minuteSnapshot.tenantIDs, opName, func(txCtx context.Context, _ int64) error {
		return fn(txCtx)
	})
	return nil
}

// forEachTenantIncludingInactive runs a maintenance operation for every
// non-deleted tenant. It is reserved for recovery work that must continue
// after a school has been deactivated.
func (s *Scheduler) forEachTenantIncludingInactive(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if !s.tenantRuntimeConfigured || (s.allTenantIDsLoader == nil && (s.db == nil || s.schoolRepo == nil)) {
		s.observeTenantRuntime("missing_tenant")
		return fmt.Errorf("tenant runtime is not configured for %s", opName)
	}

	ctx = s.withUnitOfWork(ctx)
	var tenantIDs []int64
	if s.allTenantIDsLoader != nil {
		var err error
		tenantIDs, err = s.allTenantIDsLoader(ctx)
		if err != nil {
			s.observeTenantRuntime("transaction_failure")
			return fmt.Errorf("load tenants for %s: %w", opName, err)
		}
	} else {
		var schools []platform.School
		if err := tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
			var listErr error
			schools, listErr = s.schoolRepo.ListNonDeleted(txCtx)
			return listErr
		}); err != nil {
			s.observeTenantRuntime("transaction_failure")
			return fmt.Errorf("load tenants for %s: %w", opName, err)
		}
		tenantIDs = make([]int64, 0, len(schools))
		for _, school := range schools {
			tenantIDs = append(tenantIDs, school.ID)
		}
	}
	s.forEachKnownTenant(ctx, tenantIDs, opName, func(txCtx context.Context, _ int64) error {
		return fn(txCtx)
	})
	return nil
}

// forEachTenantSettings executes fn for each active tenant, passing tenant ID for settings resolution.
// Missing runtime wiring skips work rather than invoking fn as tenant zero.
// Production jobs share one cross-tenant settings snapshot per minute.
func (s *Scheduler) forEachTenantSettings(ctx context.Context, opName string, fn func(ctx context.Context, tenantID int64) error) []int64 {
	if !s.tenantRuntimeConfigured || (s.minuteSnapshotLoader == nil && (s.db == nil || s.schoolRepo == nil)) {
		s.observeTenantRuntime("missing_tenant")
		s.getLogger().Error("tenant runtime is not configured",
			slog.String("entry_point", "worker"),
			slog.String("operation", opName),
		)
		return nil
	}

	minuteSnapshot, err := s.getMinuteSnapshot(ctx)
	if err != nil {
		// Older unit-test fakes implement only the narrow per-key resolver.
		// Production SettingsService always implements the batch loader.
		if errors.Is(err, errSchedulerSettingsBatchUnsupported) && minuteSnapshot != nil {
			return s.forEachKnownTenant(ctx, minuteSnapshot.tenantIDs, opName, fn)
		}
		s.observeTenantRuntime("transaction_failure")
		s.getLogger().Error("scheduler settings snapshot unavailable",
			slog.String("operation", opName),
			slog.String("error", err.Error()),
		)
		return nil
	}

	return s.forEachKnownTenant(ctx, minuteSnapshot.tenantIDs, opName, func(txCtx context.Context, tenantID int64) error {
		snapshot := minuteSnapshot.settings[tenantID]
		if snapshot == nil {
			return fmt.Errorf("settings snapshot missing for tenant %d", tenantID)
		}
		return fn(config.WithSettingsSnapshot(txCtx, snapshot), tenantID)
	})
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	if s.registry == nil {
		panic("worker registry is required")
	}
	ids := s.registry.IDs()
	s.getLogger().Info("starting scheduler service",
		slog.Int("registered_job_count", len(ids)),
		slog.Any("registered_job_ids", ids),
	)
	s.registry.Start()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	started := time.Now()
	s.getLogger().Info("stopping scheduler service")
	if s.stopLifecycle != nil {
		s.stopLifecycle()
	}
	close(s.done)
	s.wg.Wait()
	s.getLogger().Info("scheduler service stopped",
		slog.Duration("shutdown_drain_time", time.Since(started)),
	)
}

// taskContext is cancelled when the scheduler stops, in addition to its
// task-specific deadline.
func (s *Scheduler) taskContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

func (s *Scheduler) lifecycleContext() context.Context {
	if s.lifecycleCtx != nil {
		return s.lifecycleCtx
	}
	return context.Background()
}

// registerTask records the task in the registry and starts its polling
// goroutine.
func (s *Scheduler) registerTask(name, schedule string, runner func(*ScheduledTask)) {
	task := &ScheduledTask{
		Name:     name,
		Schedule: schedule,
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go runner(task)
}

// runMinutePolling is the shared runner for tasks that check per-tenant
// settings once per minute. It checks immediately on startup so the current
// minute isn't missed after a restart, then aligns to the minute boundary so
// ticks land at HH:MM:00. panicName and startupMsg are passed verbatim so the
// per-task log output stays byte-identical (Loki dashboards match on them).
func (s *Scheduler) runMinutePolling(task *ScheduledTask, panicName, startupMsg string, check func(context.Context, *ScheduledTask)) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%s: %v", panicName, r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info(startupMsg)

	// Immediate check on startup so we don't miss the current minute after a restart.
	s.runJobCheck(task, check)

	// Align to the next minute boundary so ticks land at HH:MM:00.
	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runJobCheck(task, check)
		case <-s.done:
			return
		}
	}
}

// runIntervalPolling is the shared runner for tasks that tick at a fixed or
// settings-driven interval. The startup delay honors s.done so shutdown during
// boot stays responsive; interval() is re-resolved on every tick so admins can
// change the cadence without a restart. panicName, startupMsg, and
// startupAttrs are passed verbatim so log output stays byte-identical.
func (s *Scheduler) runIntervalPolling(task *ScheduledTask, panicName, startupMsg string, startupDelay time.Duration, interval func() time.Duration, check func(context.Context, *ScheduledTask), startupAttrs ...any) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%s: %v", panicName, r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info(startupMsg, startupAttrs...)

	select {
	case <-time.After(startupDelay):
	case <-s.done:
		return
	}
	s.runJobCheck(task, check)

	for {
		select {
		case <-time.After(interval()):
			s.runJobCheck(task, check)
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) runJobCheck(task *ScheduledTask, check func(context.Context, *ScheduledTask)) {
	ctx := s.lifecycleContext()
	if traced, err := s.startWorkerJob(ctx, task.Name); err == nil {
		ctx = traced
	} else {
		s.getLogger().Error("worker job trace setup failed",
			slog.String("job_id", task.Name),
			slog.String("error", err.Error()),
		)
	}
	started := time.Now()
	defer func() {
		duration := time.Since(started)
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("worker job panic: %v", recovered)
			s.observeWorkerRun(JobID(task.Name), "panic", duration)
			s.traceWorkerFailure(ctx, task.Name, "panic", err)
			s.workerLogger(ctx).ErrorContext(ctx, "worker job run failed",
				slog.String("job_id", task.Name),
				slog.Duration("duration", duration),
			)
			panic(recovered)
		}
		s.observeWorkerRun(JobID(task.Name), "completed", duration)
		s.workerLogger(ctx).DebugContext(ctx, "worker job run completed",
			slog.String("job_id", task.Name),
			slog.Duration("duration", duration),
		)
	}()
	check(ctx, task)
}

// scheduleCleanupTask schedules the daily cleanup task using minute-polling.
// Each minute, it checks each tenant's configured cleanup time and fires if matched.
func (s *Scheduler) scheduleCleanupTask() {
	// Env var can globally disable cleanup regardless of settings service
	if s.env("CLEANUP_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("cleanup scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require explicit opt-in via env var
	if s.settings == nil && s.env("CLEANUP_SCHEDULER_ENABLED") != "true" {
		s.getLogger().Info("cleanup scheduler is disabled")
		return
	}

	s.registerTask("visit-cleanup", "1m-poll", s.runCleanupTaskPolling)
}

// runCleanupTaskPolling checks every minute if any tenant's cleanup time matches now.
func (s *Scheduler) runCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in cleanup task",
		"cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunCleanup)
}

// checkAndRunDailyGDPRCleanup is the shared per-tenant gate for the nightly
// retention jobs (visits, timetable, time-tracking). All three share
// KeyDataCleanupEnabled + KeyDataCleanupTime — one admin switch and one
// nightly window for all retention. dayCache dedupes per tenant per day; the
// today-mark is set immediately to prevent double-fire from concurrent ticks
// and cleared again when runForTenant returns an error so the tenant retries
// on the next matching minute. Returning that error through forEachTenantSettings
// also rolls back the tenant transaction, keeping cleanup audit rows and their
// corresponding deletes atomic.
func (s *Scheduler) checkAndRunDailyGDPRCleanup(ctx context.Context, task *ScheduledTask, dayCache *sync.Map, opName string, runForTenant func(ctx context.Context, tenantID int64, cleanupTime string) error) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, opName, func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyDataCleanupEnabled, "CLEANUP_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		cleanupTime := s.resolveStringSetting(tenantCtx, configModel.KeyDataCleanupTime, "CLEANUP_SCHEDULER_TIME", "02:00")
		if !timeMatchesNow(cleanupTime) {
			return nil
		}

		if wasRunToday(dayCache, tenantID) {
			return nil
		}

		// Mark immediately to prevent double-fire from concurrent ticks
		markRunToday(dayCache, tenantID)

		if err := runForTenant(tenantCtx, tenantID, cleanupTime); err != nil {
			// Clear mark so cleanup retries on next matching minute
			dayCache.Delete(tenantID)
			return err
		}
		return nil
	})
}

// checkAndRunCleanup evaluates each tenant's cleanup settings and runs if time matches.
func (s *Scheduler) checkAndRunCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastDataCleanup, "cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running data cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		if !s.executeCleanupForTenant(cleanupCtx, tenantID) {
			return fmt.Errorf("data cleanup failed for tenant %d", tenantID)
		}
		return nil
	})
}

// executeCleanupForTenant runs the cleanup operations for a single tenant.
// Returns true if the primary cleanup (expired visits) succeeded.
func (s *Scheduler) executeCleanupForTenant(ctx context.Context, tenantID int64) bool {
	// Cleanup expired visits
	result, err := s.cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		s.getLogger().Error("cleanup failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}

	s.getLogger().Info("cleanup completed for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("students_processed", result.StudentsProcessed),
		slog.Int64("records_deleted", result.RecordsDeleted),
	)

	// Cleanup stale supervisors
	if supervisorResult, err := s.cleanupService.CleanupStaleSupervisors(ctx); err != nil {
		s.getLogger().Error("supervisor cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else if supervisorResult.RecordsClosed > 0 {
		s.getLogger().Info("supervisor cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_closed", supervisorResult.RecordsClosed),
		)
	}

	// Cleanup stale attendance records (open from previous days)
	if attendanceResult, err := s.cleanupService.CleanupStaleAttendance(ctx); err != nil {
		s.getLogger().Error("attendance cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else if attendanceResult != nil {
		if !attendanceResult.Success {
			s.getLogger().Warn("attendance cleanup partial failure",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_closed", attendanceResult.RecordsClosed),
				slog.Int("errors", len(attendanceResult.Errors)),
			)
		} else if attendanceResult.RecordsClosed > 0 {
			s.getLogger().Info("attendance cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_closed", attendanceResult.RecordsClosed),
				slog.Int("students_affected", attendanceResult.StudentsAffected),
			)
		}
	}

	// Cleanup open work sessions
	if s.workSessionCleanup != nil {
		if closedCount, err := s.workSessionCleanup.CleanupOpenSessions(ctx); err != nil {
			s.getLogger().Error("work session cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		} else if closedCount > 0 {
			s.getLogger().Info("work session cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_closed", closedCount),
			)
		}
	}

	// The Feedback owner resolves and enforces its retention setting. A failed
	// cleanup aborts the tenant transaction instead of being logged and ignored.
	if s.feedbackCleaner != nil {
		deleted, err := s.feedbackCleaner.DeleteExpired(ctx)
		if err != nil {
			s.getLogger().Error("feedback cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return false
		}
		if deleted > 0 {
			s.getLogger().Info("feedback cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_deleted", deleted),
			)
		}
	}

	if s.unregisteredTagScanCleaner != nil {
		if deleted, err := s.unregisteredTagScanCleaner.DeleteOlderThan(ctx, 90); err != nil {
			s.getLogger().Error("unregistered RFID scan cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		} else if deleted > 0 {
			s.getLogger().Info("unregistered RFID scan cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_deleted", deleted),
				slog.Int("retention_days", 90),
			)
		}
	}

	if s.enrollmentRejectedCleanup != nil {
		result, cleanupErr := s.enrollmentRejectedCleanup.CleanupRejectedEnrollments(ctx)
		if cleanupErr != nil {
			s.getLogger().Error("rejected enrollment cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", cleanupErr.Error()))
			return false
		}
		if result.DeletedRequests > 0 {
			s.getLogger().Info("rejected enrollment cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("requests_deleted", result.DeletedRequests),
				slog.Int64("late_invites_deleted", result.DeletedLateInvites),
				slog.Int64("outbox_rows_deleted", result.DeletedOutboxRows))
		}
	}

	return true
}

// completeTimetableInstancesForEndedSessions closes the schedule-side rows
// linked to active.groups that the daily session-end job just closed.
//
// The active service owns active.groups/visits/supervisors; timetable
// instances live in schedule.*, so the scheduler bridges the two inside the
// tenant transaction created by the scheduler runtime. If this sync fails, callers
// return the error so the active close rolls back too instead of leaving the
// planner in a stale "active" state.
func (s *Scheduler) completeTimetableInstancesForEndedSessions(ctx context.Context, result *active.DailySessionCleanupResult) (int, error) {
	if result == nil || len(result.EndedActiveGroupIDs) == 0 {
		return 0, nil
	}
	if s.instanceStudentRepo == nil || s.timetableBridge == nil {
		return 0, nil
	}

	now := time.Now()

	// The bulk visit close in EndDailySessions bypasses the per-visit
	// attendance syncer, so mirror the checkout into slot attendance here —
	// otherwise history/exports keep showing children as still checked in.
	if _, err := s.instanceStudentRepo.CloseOpenCheckoutsByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now); err != nil {
		return 0, fmt.Errorf("close open timetable checkouts: %w", err)
	}

	// The bridge finalizes attendance before it stamps the instances completed:
	// children the care plan does not place in the OGS that day are spared the
	// absent stamp (#1747), everybody else flips expected → absent. Shared with
	// the force-start path so no caller can complete an instance that still
	// carries expected rows.
	rows, err := s.timetableBridge.CompleteActiveByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now)
	if err != nil {
		return 0, fmt.Errorf("complete active timetable instances: %w", err)
	}
	return int(rows), nil
}

// scheduleTokenCleanupTask schedules hourly token cleanup
func (s *Scheduler) scheduleTokenCleanupTask() {
	s.registerTask("token-cleanup", "1h", s.runTokenCleanupTask)
}

// runTokenCleanupTask runs the token cleanup task on schedule
func (s *Scheduler) runTokenCleanupTask(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in token cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("token cleanup task scheduled to run every hour")

	// Run immediately on startup
	s.runJobCheck(task, s.executeTokenCleanup)

	// Then run every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runJobCheck(task, s.executeTokenCleanup)
		case <-s.done:
			return
		}
	}
}

// executeTokenCleanup executes the token cleanup task
func (s *Scheduler) executeTokenCleanup(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(time.Hour)
		task.mu.Unlock()
	}()

	_ = s.runCleanupJobs(ctx)
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	ctx, err := s.startWorkerJob(s.lifecycleContext(), "token-cleanup")
	if err != nil {
		return fmt.Errorf("%w: %v", errWorkerTraceStart, err)
	}
	return s.runCleanupJobs(ctx)
}

func (s *Scheduler) runCleanupJobs(ctx context.Context) error {
	started := time.Now()
	logger := s.workerLogger(ctx)
	logger.InfoContext(ctx, "running scheduled token cleanup")
	if len(s.cleanupJobs) == 0 {
		logger.InfoContext(ctx, "no cleanup jobs registered, skipping token cleanup")
		return nil
	}

	if !s.tenantRuntimeConfigured {
		if !s.traceWorkerFailure(ctx, "token-cleanup", "missing_tenant", tenant.ErrRuntimeRequired) {
			s.observeTenantRuntime("missing_tenant")
			logger.ErrorContext(ctx, "runtime operation failed",
				slog.String("entry_point", "worker"),
				slog.String("operation", "token-cleanup"),
				slog.String("outcome", "missing_tenant"),
			)
		}
		return tenant.ErrRuntimeRequired
	}
	ctx = s.withUnitOfWork(ctx)
	var firstErr error

	for _, job := range s.cleanupJobs {
		if job.Run == nil {
			continue
		}

		var count int
		var err error
		if s.db != nil {
			err = tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, tx bun.Tx) error {
				var runErr error
				count, runErr = job.Run(txCtx)
				return runErr
			})
		} else {
			count, err = job.Run(ctx)
		}
		if err != nil {
			if !s.traceWorkerFailure(ctx, job.Description, "transaction_failure", err) {
				logger.ErrorContext(ctx, "cleanup job failed", slog.String("job", job.Description))
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		logger.InfoContext(ctx, "cleanup job completed",
			slog.String("job", job.Description),
			slog.Int("records_deleted", count))
	}
	if firstErr == nil {
		logger.InfoContext(ctx, "token cleanup completed", slog.Duration("duration", time.Since(started).Round(time.Millisecond)))
	}

	return firstErr
}

// buildCleanupJobs constructs the set of cleanup jobs so other runners can reuse the same registry.
func buildCleanupJobs(authService AuthCleanup, invitationService InvitationCleaner, emailChangeCleaner EmailChangeTokenCleaner, operatorInvitationCleaner OperatorInvitationCleaner) []CleanupJob {
	var jobs []CleanupJob

	if authService != nil {
		jobs = append(jobs,
			CleanupJob{
				Description: "Auth token cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredTokens(ctx)
				},
			},
			CleanupJob{
				Description: "Password reset token cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredPasswordResetTokens(ctx)
				},
			},
			CleanupJob{
				Description: "Password reset rate limit cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredRateLimits(ctx)
				},
			},
		)
	}

	if invitationService != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Invitation cleanup",
			Run: func(ctx context.Context) (int, error) {
				return invitationService.CleanupExpiredInvitations(ctx)
			},
		})
	}

	if emailChangeCleaner != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Email change token cleanup",
			Run: func(ctx context.Context) (int, error) {
				return emailChangeCleaner.CleanupExpiredEmailChangeTokens(ctx)
			},
		})
	}

	if operatorInvitationCleaner != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Operator invitation token cleanup",
			Run: func(ctx context.Context) (int, error) {
				return operatorInvitationCleaner.CleanupExpiredOperatorInvitations(ctx)
			},
		})
	}

	return jobs
}

// scheduleSessionEndTask schedules the daily session end task using minute-polling.
func (s *Scheduler) scheduleSessionEndTask() {
	// Env var can globally disable session end regardless of settings service
	if s.env("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require non-false env var
	if s.settings == nil && s.env("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled")
		return
	}

	s.registerTask("session-end", "1m-poll", s.runSessionEndTaskPolling)
}

// runSessionEndTaskPolling checks every minute if any tenant's session end time matches now.
func (s *Scheduler) runSessionEndTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in session end task",
		"session end task using minute-polling for per-tenant scheduling",
		s.checkAndRunSessionEnd)
}

// checkAndRunSessionEnd evaluates each tenant's session end settings and runs if time matches.
func (s *Scheduler) checkAndRunSessionEnd(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "session-end-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeySessionEndEnabled, "SESSION_END_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		endTime := s.resolveStringSetting(tenantCtx, configModel.KeySessionEndTime, "SESSION_END_TIME", "18:00")
		if !timeMatchesNow(endTime) {
			return nil
		}

		if wasRunToday(&s.lastSessionEnd, tenantID) {
			return nil
		}

		s.getLogger().Info("running session end for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("session_end_time", endTime),
		)

		ok, err := s.executeSessionEndForTenant(tenantCtx, tenantID)
		if err != nil {
			return err
		}
		if !ok {
			return nil // failure already logged; retry on the next matching minute
		}

		markRunToday(&s.lastSessionEnd, tenantID)
		return nil
	})
}

// executeSessionEndForTenant ends the tenant's daily sessions and completes
// the linked timetable instances. Session-end errors are logged and swallowed
// (ok=false, nil error) so other tenants still run; timetable-sync errors are
// returned so the caller's tenant transaction rolls back. ok is true only on
// full success — callers use it to decide whether to mark today as done.
func (s *Scheduler) executeSessionEndForTenant(ctx context.Context, tenantID int64) (bool, error) {
	timeoutMinutes := s.resolveIntSetting(ctx, configModel.KeySessionEndTimeoutMinutes, "SESSION_END_TIMEOUT_MINUTES", 10)
	endCtx, endCancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer endCancel()

	result, err := s.activeService.EndDailySessions(endCtx)
	if err != nil {
		s.getLogger().Error("session end failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false, nil // don't fail other tenants
	}
	timetableCompleted, err := s.completeTimetableInstancesForEndedSessions(endCtx, result)
	if err != nil {
		s.getLogger().Error("session end timetable sync failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false, err
	}

	s.getLogger().Info("session end completed for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("sessions_ended", result.SessionsEnded),
		slog.Int("visits_ended", result.VisitsEnded),
		slog.Int("supervisors_ended", result.SupervisorsEnded),
		slog.Int("timetable_instances_completed", timetableCompleted),
	)
	return true, nil
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task.
// Uses a fixed 5-minute polling interval, resolves per-tenant settings on each tick.
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Quick check: if globally disabled via env and no settings service, skip
	if s.settings == nil && s.env("SESSION_CLEANUP_ENABLED") == "false" {
		s.getLogger().Info("session cleanup is disabled")
		return
	}

	// Parse global defaults (used as fallback and for backward-compatible struct fields)
	s.sessionCleanupIntervalMinutes = 15
	if envInterval := s.env("SESSION_CLEANUP_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			s.sessionCleanupIntervalMinutes = parsed
		}
	}
	s.sessionAbandonedThresholdMinutes = 60
	if envThreshold := s.env("SESSION_ABANDONED_THRESHOLD_MINUTES"); envThreshold != "" {
		if parsed, err := strconv.Atoi(envThreshold); err == nil && parsed > 0 {
			s.sessionAbandonedThresholdMinutes = parsed
		}
	}

	s.registerTask("session-cleanup", "5m-poll", s.runSessionCleanupTaskPolling)
}

// runSessionCleanupTaskPolling checks every 5 minutes if any tenant needs session cleanup.
func (s *Scheduler) runSessionCleanupTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in session cleanup task",
		"session cleanup using 5-minute polling for per-tenant scheduling",
		30*time.Second, func() time.Duration { return 5 * time.Minute }, s.checkAndRunSessionCleanup)
}

// checkAndRunSessionCleanup evaluates each tenant's session cleanup settings.
func (s *Scheduler) checkAndRunSessionCleanup(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "session-cleanup", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeySessionCleanupEnabled, "SESSION_CLEANUP_ENABLED", true)
		if !enabled {
			return nil
		}

		intervalMinutes := s.resolveIntSetting(tenantCtx, configModel.KeySessionCleanupIntervalMinutes, "SESSION_CLEANUP_INTERVAL_MINUTES", 15)
		thresholdMinutes := s.resolveIntSetting(tenantCtx, configModel.KeySessionAbandonedThresholdMin, "SESSION_ABANDONED_THRESHOLD_MINUTES", 60)

		// Check if enough time has passed since last run for this tenant
		if val, ok := s.lastSessionCleanup.Load(tenantID); ok {
			if lastRun, ok := val.(time.Time); ok {
				if time.Since(lastRun) < time.Duration(intervalMinutes)*time.Minute {
					return nil // not yet time for this tenant
				}
			}
		}

		threshold := time.Duration(thresholdMinutes) * time.Minute
		count, err := s.activeService.CleanupAbandonedSessions(tenantCtx, threshold)
		if err != nil {
			return err
		}

		if count > 0 {
			s.getLogger().Info("session cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("abandoned_sessions", count),
				slog.Int("threshold_minutes", thresholdMinutes),
			)
		}

		s.lastSessionCleanup.Store(tenantID, time.Now())
		return nil
	})
}

// scheduleBreakAutoEndTask schedules the break auto-end task.
// Runs at a fixed interval (minimum of configured values across tenants or default 60s).
func (s *Scheduler) scheduleBreakAutoEndTask() {
	if s.breakAutoEnder == nil {
		s.getLogger().Info("break auto-end not configured (no BreakAutoEnder service)")
		return
	}

	// Resolve interval from env var (global, not per-tenant)
	s.breakAutoEndIntervalSeconds = 60
	if val := s.env("BREAK_AUTO_END_INTERVAL_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			s.breakAutoEndIntervalSeconds = parsed
		}
	}

	s.registerTask("break-auto-end", fmt.Sprintf("%ds-poll", s.breakAutoEndIntervalSeconds), s.runBreakAutoEndTaskPolling)
}

// runBreakAutoEndTaskPolling runs break auto-end check at the configured interval for all tenants.
func (s *Scheduler) runBreakAutoEndTaskPolling(task *ScheduledTask) {
	interval := time.Duration(s.breakAutoEndIntervalSeconds) * time.Second
	s.runIntervalPolling(task, "panic in break auto-end task", "break auto-end polling started",
		10*time.Second, func() time.Duration { return interval }, s.checkAndRunBreakAutoEnd,
		slog.Int("interval_seconds", s.breakAutoEndIntervalSeconds))
}

// checkAndRunBreakAutoEnd runs break auto-end for all tenants.
func (s *Scheduler) checkAndRunBreakAutoEnd(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 30*time.Second)
	defer cancel()

	breakErr := s.forEachTenant(ctx, "break-auto-end", func(tenantCtx context.Context) error {
		count, err := s.breakAutoEnder.AutoEndExpiredBreaks(tenantCtx)
		if err != nil {
			return err
		}

		if count > 0 {
			s.getLogger().Info("break auto-end completed",
				slog.Int("breaks_ended", count))
		}
		return nil
	})
	if breakErr != nil {
		s.getLogger().Error("break auto-end failed", "error", breakErr)
	}
}

// scheduleAutoCheckoutTask schedules the auto-checkout-at-shift-end task
// (#1798). Runs on the same fixed 60-second poll as break auto-end; the
// feature itself is gated per tenant via tracking.auto_checkout_enabled
// (registry default false — pure opt-in).
func (s *Scheduler) scheduleAutoCheckoutTask() {
	if s.autoCheckouter == nil {
		s.getLogger().Info("auto-checkout not configured (no AutoCheckouter service)")
		return
	}

	s.registerTask("auto-checkout", "60s-poll", s.runAutoCheckoutTaskPolling)
}

// runAutoCheckoutTaskPolling runs the auto-checkout check every 60 seconds.
func (s *Scheduler) runAutoCheckoutTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in auto-checkout task", "auto-checkout polling started",
		10*time.Second, func() time.Duration { return 60 * time.Second }, s.checkAndRunAutoCheckout)
}

// checkAndRunAutoCheckout closes due sessions for every tenant that has the
// feature enabled.
func (s *Scheduler) checkAndRunAutoCheckout(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 30*time.Second)
	defer cancel()

	s.forEachTenantSettings(ctx, "auto-checkout", func(tenantCtx context.Context, tenantID int64) error {
		// Opt-in per tenant; no env var fallback (new feature, settings-only).
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyTrackingAutoCheckoutEnabled, "", false)
		if !enabled {
			return nil
		}

		// Zero is valid here: grace 0 means checkout exactly at shift end.
		graceMinutes := s.resolveNonNegativeIntSetting(tenantCtx, configModel.KeyTrackingAutoCheckoutGraceMinutes, "", 15)
		count, err := s.autoCheckouter.AutoCheckoutDueSessions(tenantCtx, time.Duration(graceMinutes)*time.Minute)
		if err != nil {
			return err
		}
		if count > 0 {
			s.getLogger().Info("auto-checkout completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_closed", count))
		}
		return nil
	})
}

// --- Settings-aware helpers ---
//
// Fallback chain: tenant DB override → env var → registry default.
// The settings service's Resolve* returns the registry default when no tenant
// override exists, which would skip the env var. We use HasTenantOverride to
// distinguish "tenant explicitly set a value" from "returning registry default".

// resolveStringSetting resolves a setting via the settings service with env var fallback.
func (s *Scheduler) resolveStringSetting(ctx context.Context, key string, envVar string, defaultVal string) string {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		fallback = val
	}
	return config.ResolveStringOrDefault(ctx, s.settings, key, fallback, s.getLogger())
}

// resolveBoolSetting resolves a boolean setting via the settings service with env var fallback.
func (s *Scheduler) resolveBoolSetting(ctx context.Context, key string, envVar string, defaultVal bool) bool {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		fallback = val == "true"
	}
	return config.ResolveBoolOrDefault(ctx, s.settings, key, fallback, s.getLogger())
}

// resolveIntSetting resolves an integer setting via the settings service with env var fallback.
func (s *Scheduler) resolveIntSetting(ctx context.Context, key string, envVar string, defaultVal int) int {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			fallback = parsed
		}
	}
	if val := config.ResolveIntOrDefault(ctx, s.settings, key, fallback, s.getLogger()); val > 0 {
		return val
	}
	return fallback
}

func (s *Scheduler) resolveRequiredPositiveIntSetting(ctx context.Context, key string, envVar string) (int, error) {
	if s.settings == nil {
		return 0, fmt.Errorf("settings resolver not configured")
	}
	hasOverride, err := s.settings.HasTenantOverride(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("check override for %s: %w", key, err)
	}
	if !hasOverride {
		if val := s.env(envVar); val != "" {
			parsed, err := strconv.Atoi(val)
			if err != nil || parsed <= 0 {
				return 0, fmt.Errorf("environment variable %s must be positive integer, got %q", envVar, val)
			}
			return parsed, nil
		}
	}
	val, err := s.settings.ResolveInt(ctx, key)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, fmt.Errorf("setting %s must be positive, got %d", key, val)
	}
	return val, nil
}

// resolveNonNegativeIntSetting is resolveIntSetting for settings where zero is
// a meaningful value (e.g. tracking.auto_checkout_grace_minutes = 0 means
// checkout exactly at the planned shift end).
func (s *Scheduler) resolveNonNegativeIntSetting(ctx context.Context, key string, envVar string, defaultVal int) int {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			fallback = parsed
		}
	}
	if val := config.ResolveIntOrDefault(ctx, s.settings, key, fallback, s.getLogger()); val >= 0 {
		return val
	}
	return fallback
}

// waitUntilNextMinute blocks until the start of the next wall-clock minute,
// so that subsequent 60-second ticks are aligned to HH:MM:00.
// Returns false if the scheduler is shutting down during the wait.
func (s *Scheduler) waitUntilNextMinute() bool {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute) //nolint:forbidigo // sub-day minute alignment, not calendar-date math
	delay := time.Until(nextMinute)
	select {
	case <-time.After(delay):
		return true
	case <-s.done:
		return false
	}
}

// timeMatchesNow checks if an HH:MM time string matches the current minute.
func timeMatchesNow(timeStr string) bool {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return false
	}
	now := time.Now()
	return now.Hour() == hour && now.Minute() == minute
}

// wasRunToday checks if a per-tenant job already ran today.
func wasRunToday(lastRunMap *sync.Map, tenantID int64) bool {
	return wasRunAt(lastRunMap, tenantID, time.Now())
}

func wasRunAt(lastRunMap *sync.Map, tenantID int64, now time.Time) bool {
	val, ok := lastRunMap.Load(tenantID)
	if !ok {
		return false
	}
	lastRun, ok := val.(time.Time)
	if !ok {
		return false
	}
	return lastRun.Year() == now.Year() && lastRun.YearDay() == now.YearDay()
}

// markRunToday records that a per-tenant job ran today.
func markRunToday(lastRunMap *sync.Map, tenantID int64) {
	markRunAt(lastRunMap, tenantID, time.Now())
}

func markRunAt(lastRunMap *sync.Map, tenantID int64, now time.Time) {
	lastRunMap.Store(tenantID, now)
}

// scheduleStatusFlagClearTask schedules a daily task to clear sick / excused
// flags for tenants whose operations.sick_clear_mode or
// operations.excused_clear_mode is set to "end_of_day". The task fires at the
// tenant's configured operations.status_flag_clear_time.
func (s *Scheduler) scheduleStatusFlagClearTask() {
	// Env var kill switch to allow ops to disable this task without code changes.
	if s.env("STATUS_FLAG_CLEAR_ENABLED") == "false" {
		s.getLogger().Info("status flag clear scheduler is disabled via env var")
		return
	}

	s.registerTask("status-flag-clear", "1m-poll", s.runStatusFlagClearTaskPolling)
}

// runStatusFlagClearTaskPolling checks every minute if any tenant's status
// flag clear time matches now and clears the configured end_of_day flags.
func (s *Scheduler) runStatusFlagClearTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in status flag clear task",
		"status flag clear task using minute-polling for per-tenant scheduling",
		s.checkAndRunStatusFlagClear)
}

// checkAndRunStatusFlagClear evaluates each tenant's clear_mode settings and
// clears flags when the configured status flag clear time matches now.
func (s *Scheduler) checkAndRunStatusFlagClear(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 30*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "status-flag-clear", func(tenantCtx context.Context, tenantID int64) error {
		clearTime := s.resolveStringSetting(tenantCtx, configModel.KeyStatusFlagClearTime, "", "18:00")
		if clearTime == "" || !timeMatchesNow(clearTime) {
			return nil
		}

		if wasRunToday(&s.lastStatusFlagClear, tenantID) {
			return nil
		}
		markRunToday(&s.lastStatusFlagClear, tenantID)

		sickMode := s.resolveStringSetting(tenantCtx, configModel.KeySickClearMode, "", configModel.ClearModeNextCheckin)
		excusedMode := s.resolveStringSetting(tenantCtx, configModel.KeyExcusedClearMode, "", configModel.ClearModeEndOfDay)

		if sickMode == configModel.ClearModeEndOfDay {
			if affected, err := s.clearStatusFlag(tenantCtx, "sick", "sick_since"); err != nil {
				s.getLogger().Error("end-of-day sick clear failed",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
				s.lastStatusFlagClear.Delete(tenantID)
			} else if affected > 0 {
				s.getLogger().Info("end-of-day sick clear completed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("students_cleared", affected),
				)
			}
		}

		if excusedMode == configModel.ClearModeEndOfDay {
			if affected, err := s.clearStatusFlag(tenantCtx, "excused", "excused_since"); err != nil {
				s.getLogger().Error("end-of-day excused clear failed",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
				s.lastStatusFlagClear.Delete(tenantID)
			} else if affected > 0 {
				s.getLogger().Info("end-of-day excused clear completed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("students_cleared", affected),
				)
			}
		}

		return nil
	})
}

// clearStatusFlag clears a boolean flag + its timestamp for every student in
// the current tenant transaction. Column names are trusted constants — the
// caller picks them from a fixed set.
func (s *Scheduler) clearStatusFlag(ctx context.Context, flagColumn, sinceColumn string) (int64, error) {
	if s.studentStatusDayRepo == nil {
		return 0, fmt.Errorf("scheduler student status day repo not configured")
	}
	status, err := statusForFlagColumn(flagColumn)
	if err != nil {
		return 0, err
	}
	return s.studentStatusDayRepo.ArchiveAndClearStatusFlag(
		ctx, flagColumn, sinceColumn, status,
		timezone.TodayDate(), time.Now(), activeModel.StudentStatusSourceEndOfDay,
	)
}

func statusForFlagColumn(flagColumn string) (string, error) {
	switch flagColumn {
	case "sick":
		return activeModel.StudentStatusDaySick, nil
	case "excused":
		return activeModel.StudentStatusDayExcused, nil
	default:
		return "", fmt.Errorf("unsupported status flag column %q", flagColumn)
	}
}

// --- Timetable materialization (WP-B8) ---
//
// Defaults-on: materialization runs unless `timetable.materialization_enabled`
// is overridden to false (operator-only setting). With the opt-out, the
// per-tenant check short-circuits and does nothing for that tenant. The job fires
// once per day-of-week (on the configured weekday) via a 60-second
// minute-polling loop, same shape as scheduleSessionEndTask.
//
// The actual business logic (period selection, A/B week filtering, exception
// application, instance/staff/student generation) lives entirely in
// services/schedule.MaterializationService — the scheduler only decides
// "when to call it" and "for which tenant".

// scheduleMaterializationTask registers the weekly timetable-materialization
// task when a Materializer has been wired in. No materializer → no task.
func (s *Scheduler) scheduleMaterializationTask() {
	if s.materializer == nil {
		s.getLogger().Info("timetable materialization not configured (no Materializer service)")
		return
	}

	s.registerTask("timetable-materialization", "1m-poll", s.runMaterializationTaskPolling)
}

// runMaterializationTaskPolling ticks every minute and delegates to
// checkAndRunMaterialization. Minute alignment matches the other scheduler
// tasks so HH:MM:00 ticks land deterministically.
func (s *Scheduler) runMaterializationTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in materialization task",
		"timetable materialization using minute-polling for per-tenant scheduling",
		s.checkAndRunMaterializationWithContext)
}

// checkAndRunMaterializationWithContext iterates active tenants and fires
// materialization for each tenant whose configured weekday matches today's ISO
// weekday, gated on the timetable.materialization_enabled setting.
func (s *Scheduler) checkAndRunMaterializationWithContext(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 30*time.Minute)
	defer cancel()

	now := s.materializationTime()
	s.forEachTenantSettings(ctx, "materialization-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyTimetableMaterializationEnabled, "", true)
		if !enabled {
			return nil
		}

		// Registry default is 5 (Friday, ISO 8601). The helper goes through
		// HasTenantOverride → ResolveInt → env → default, exactly matching
		// the documented fallback pattern.
		targetWeekday := s.resolveIntSetting(tenantCtx, configModel.KeyTimetableMaterializationWeekday, "", 5)
		if !isoWeekdayMatches(targetWeekday, now) {
			return nil
		}

		if wasRunAt(&s.lastMaterialization, tenantID, now) {
			return nil
		}
		markRunAt(&s.lastMaterialization, tenantID, now)

		weeksAhead := s.resolveIntSetting(tenantCtx, configModel.KeyTimetableMaterializationWeeksAhead, "", 1)
		from, to := s.materializer.ResolveWindow(timezone.DateFromTime(now), weeksAhead)

		s.getLogger().Info("running timetable materialization for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("target_weekday", targetWeekday),
			slog.Int("weeks_ahead", weeksAhead),
			slog.String("from", from.String()),
			slog.String("to", to.String()),
		)

		result, err := s.materializer.MaterializeForTenant(tenantCtx, from, to, scheduleSvc.MaterializationSourceScheduler)
		if err != nil {
			// Clear today-mark so a retry on the next scheduler day succeeds.
			// We do NOT clear immediately because that would cause every
			// subsequent minute on the same day to retry a known-failing run.
			s.getLogger().Error("timetable materialization failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return nil
		}

		if result.InstancesCreated > 0 || result.CandidatesRaced > 0 {
			s.getLogger().Info("timetable materialization completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("created", result.InstancesCreated),
				slog.Int("skipped_existing", result.CandidatesSkippedExisting),
				slog.Int("raced", result.CandidatesRaced),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

func isoWeekdayMatches(wd int, now time.Time) bool {
	today := timezone.DateFromTime(now).Weekday()
	if today == time.Sunday {
		return wd == 7
	}
	return wd == int(today)
}

func (s *Scheduler) materializationTime() time.Time {
	if s.materializationNow != nil {
		return s.materializationNow()
	}
	return time.Now()
}

// --- Timetable auto-start tick ---
//
// Purpose: when a tenant enables timetable.auto_start_planned, start planned
// activity instances whose configured time window is currently active. The
// service remains conservative: no assigned staff or any conflict warning means
// "do not start automatically"; staff can still start manually.

func (s *Scheduler) scheduleAutoStartTask() {
	if s.autoStart == nil {
		s.getLogger().Info("timetable auto-start tick not configured (missing service)")
		return
	}

	s.registerTask("timetable-auto-start", "1m-poll", s.runAutoStartTaskPolling)
}

func (s *Scheduler) runAutoStartTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable auto-start task",
		"timetable auto-start tick using minute-polling",
		s.checkAndRunAutoStart)
}

func (s *Scheduler) checkAndRunAutoStart(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "timetable-auto-start", func(tenantCtx context.Context, tenantID int64) error {
		if !s.resolveBoolSetting(tenantCtx, configModel.KeyTimetableEnabled, "", false) {
			return nil
		}
		if !s.resolveBoolSetting(tenantCtx, configModel.KeyTimetableAutoStartPlanned, "", false) {
			return nil
		}

		result, err := s.autoStart.RunForTenant(tenantCtx, time.Now())
		if err != nil {
			s.getLogger().Warn("timetable auto-start failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return nil
		}
		if result.Started > 0 || result.SkippedConflict > 0 || result.Failed > 0 {
			s.getLogger().Info("timetable auto-start completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("checked", result.Checked),
				slog.Int("started", result.Started),
				slog.Int("skipped_no_staff", result.SkippedNoStaff),
				slog.Int("skipped_conflict", result.SkippedConflict),
				slog.Int("skipped_moved", result.SkippedMoved),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// --- Timetable auto-end tick ---

func (s *Scheduler) scheduleAutoEndTask() {
	if s.autoEnd == nil {
		s.getLogger().Info("timetable auto-end tick not configured (missing service)")
		return
	}

	s.registerTask("timetable-auto-end", "1m-poll", s.runAutoEndTaskPolling)
}

func (s *Scheduler) runAutoEndTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable auto-end task",
		"timetable auto-end tick using minute-polling",
		s.checkAndRunAutoEnd)
}

func (s *Scheduler) checkAndRunAutoEnd(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "timetable-auto-end", func(tenantCtx context.Context, tenantID int64) error {
		return s.runAutoEndForTenant(tenantCtx, tenantID)
	})
}

func (s *Scheduler) runAutoEndForTenant(ctx context.Context, tenantID int64) error {
	if !s.resolveBoolSetting(ctx, configModel.KeyTimetableEnabled, "", true) ||
		!s.resolveBoolSetting(ctx, configModel.KeyTimetableAutoEndEnabled, "", false) {
		return nil
	}
	graceMinutes := s.resolveIntSetting(ctx, configModel.KeyTimetableAutoEndGraceMinutes, "", 0)
	result, err := s.autoEnd.RunForTenant(ctx, time.Now(), time.Duration(graceMinutes)*time.Minute)
	if err != nil {
		return fmt.Errorf("auto-end tenant %d: %w", tenantID, err)
	}
	if result.Failed > 0 {
		s.getLogger().Warn("timetable auto-end completed with failures",
			slog.Int64("tenant_id", tenantID),
			slog.Int("checked", result.Checked),
			slog.Int("completed", result.Completed),
			slog.Int("failed", result.Failed),
			slog.Int("grace_minutes", graceMinutes),
			slog.Int64("duration_ms", result.DurationMS),
		)
		return nil
	}
	if result.Completed > 0 || result.SkippedConcurrent > 0 {
		s.getLogger().Info("timetable auto-end completed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("checked", result.Checked),
			slog.Int("completed", result.Completed),
			slog.Int("skipped_concurrent", result.SkippedConcurrent),
			slog.Int("grace_minutes", graceMinutes),
			slog.Int64("duration_ms", result.DurationMS),
		)
	}
	return nil
}

// --- Instance overdue tick (WP-B9) ---
//
// Purpose: emit realtime.EventInstanceOverdue once per planned instance that
// has exceeded its start_time by the tenant-configured threshold. Drives the
// "Überfällig" badge in the staff "My Day" view.
//
// Cadence: same minute-polling shape as every other scheduler task. The tick
// is independent of auto-start: tenants with auto-start disabled still need
// overdue hints, and tenants with auto-start enabled still need hints for rows
// auto-start intentionally skipped.
//
// Re-fire guard: an in-memory sync.Map keyed by (tenant_id, instance_id).
// Cleared explicitly when the civil-date rolls over — do not rely on entries
// decaying via planned-filter misses, that leaks memory for cancelled or
// completed instances until restart. On a mid-day restart the cache is
// empty and each still-overdue planned instance fires once more; subscribers
// are idempotent, that cost is acceptable for zero disk state.

// scheduleInstanceOverdueTask registers the tick when all dependencies are
// wired. A partial setup registers no task, matching the other typed jobs'
// pattern so misconfiguration cannot emit overdue events for unresolved rooms.
func (s *Scheduler) scheduleInstanceOverdueTask() {
	if s.instanceRepo == nil || s.instanceRoomRepo == nil || s.overdueBroadcaster == nil {
		s.getLogger().Info("instance overdue tick not configured (missing instance repo, room repo, or broadcaster)")
		return
	}

	s.registerTask("instance-overdue", "1m-poll", s.runInstanceOverdueTaskPolling)
}

// runInstanceOverdueTaskPolling mirrors the minute-polling loops used by
// cleanup / session-end / materialization. Startup check + minute alignment
// + 60 s ticker + done-signal exit.
func (s *Scheduler) runInstanceOverdueTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in instance overdue task",
		"instance overdue tick using minute-polling",
		s.checkAndRunOverdue)
}

// checkAndRunOverdue rotates the day-cache when midnight has been crossed,
// then iterates active tenants and delegates the per-tenant work to
// runOverdueForTenant. Extracted into two methods so unit tests can invoke
// the inner loop directly with a synthetic tenant ctx without building the
// full school repo + settings service stack.
func (s *Scheduler) checkAndRunOverdue(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	s.rotateOverdueCacheIfNewDay(time.Now())

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "instance-overdue", func(tenantCtx context.Context, tenantID int64) error {
		threshold := s.resolveIntSetting(tenantCtx, configModel.KeyTimetableOverdueThresholdMinutes, "", 5)
		s.runOverdueForTenant(tenantCtx, tenantID, threshold, time.Now())
		return nil
	})
}

// runOverdueForTenant iterates today's instances for a single tenant and
// emits instance_overdue once per still-planned, past-threshold row. `now`
// is injected for deterministic tests. `threshold` < 1 is a no-op.
func (s *Scheduler) runOverdueForTenant(ctx context.Context, tenantID int64, threshold int, now time.Time) {
	if threshold < 1 {
		return
	}

	today := timezone.DateFromTime(now)
	instances, err := s.instanceRepo.FindByTenantAndDate(ctx, today)
	if err != nil {
		s.getLogger().Warn("overdue tick: load today's instances failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}

	cutoff := time.Duration(threshold) * time.Minute
	candidates := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	roomIDs := make(map[int64]struct{})

	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned {
			continue
		}
		instanceStart := combineDayAndTime(today, inst.StartTime)
		if now.Sub(instanceStart) < cutoff {
			continue
		}
		candidates = append(candidates, inst)
		roomIDs[inst.RoomID] = struct{}{}
	}

	if len(roomIDs) > 0 {
		if s.instanceRoomRepo == nil {
			s.getLogger().Warn("overdue tick: room repository is not configured",
				slog.Int64("tenant_id", tenantID),
			)
			return
		}
		ids := make([]int64, 0, len(roomIDs))
		for roomID := range roomIDs {
			ids = append(ids, roomID)
		}
		rooms, err := s.instanceRoomRepo.FindByIDs(ctx, ids)
		if err != nil {
			s.getLogger().Warn("overdue tick: load instance rooms failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return
		}
		resolvedRoomIDs := make(map[int64]struct{}, len(rooms))
		for _, room := range rooms {
			if room == nil {
				continue
			}
			resolvedRoomIDs[room.ID] = struct{}{}
		}
		for roomID := range roomIDs {
			if _, found := resolvedRoomIDs[roomID]; !found {
				s.getLogger().Warn("overdue tick: instance room could not be resolved",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("room_id", roomID),
				)
				return
			}
		}
	}

	for _, inst := range candidates {
		key := overdueKey{tenantID: tenantID, instanceID: inst.ID}
		if _, seen := s.overdueEmitted.Load(key); seen {
			continue
		}
		s.emitInstanceOverdue(ctx, tenantID, inst)
		s.overdueEmitted.Store(key, now)
	}
}

// emitInstanceOverdue builds the SSE envelope and fires it tenant-wide:
// a planned instance has no bridged active.group yet, so there is no group-
// scoped topic to route through. Admin dashboard subscribers pick it up.
func (s *Scheduler) emitInstanceOverdue(ctx context.Context, tenantID int64, inst *scheduleModel.ActivityInstance) {
	instanceIDStr := fmt.Sprintf("%d", inst.ID)
	instanceDate := inst.Date.String()
	instanceStart := inst.StartTime.Format("15:04:05")
	roomIDStr := fmt.Sprintf("%d", inst.RoomID)

	event := realtime.NewEvent(realtime.EventInstanceOverdue, "", realtime.EventData{
		InstanceID:        &instanceIDStr,
		InstanceDate:      &instanceDate,
		InstanceStartTime: &instanceStart,
		RoomID:            &roomIDStr,
	})
	if err := s.overdueBroadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("overdue tick: broadcast failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("instance_id", inst.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	reason := "instance_overdue"
	refreshEvent := realtime.NewEvent(realtime.EventActiveSupervisionChanged, "", realtime.EventData{
		InstanceID: &instanceIDStr,
		Reason:     &reason,
	})
	if err := s.overdueBroadcaster.BroadcastToTenant(tenantID, refreshEvent); err != nil {
		s.getLogger().Warn("overdue tick: active supervision broadcast failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("instance_id", inst.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	s.getLogger().Info("instance_overdue broadcast",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("instance_id", inst.ID),
		slog.String("start_time", instanceStart),
	)
	_ = ctx // reserved: if later the broadcaster needs ctx for cancellation.
}

// rotateOverdueCacheIfNewDay clears the re-fire guard on civil-date roll-
// over. Called at the top of every tick so a restart mid-day does not
// miss the rollover.
func (s *Scheduler) rotateOverdueCacheIfNewDay(now time.Time) {
	today := timezone.DateFromTime(now)
	s.overdueEmittedDayMu.Lock()
	defer s.overdueEmittedDayMu.Unlock()
	if s.overdueEmittedDay != today {
		s.overdueEmitted.Range(func(k, _ any) bool {
			s.overdueEmitted.Delete(k)
			return true
		})
		s.overdueEmittedDay = today
	}
}

// combineDayAndTime returns "day at time-of-day", reading the wall-clock
// hour/minute/second from `tod` (typically an ActivityInstance.StartTime
// which lives as a bare TIME). Stays in the server's local zone so the
// comparison with time.Now() is apples-to-apples.
func combineDayAndTime(day timezone.Date, tod time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(),
		tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.Local)
}

// --- Timetable GDPR cleanup (WP-B14) ---
//
// Runs daily, gated on the SAME KeyDataCleanupEnabled toggle as the visits
// cleanup. Admins configure one nightly window; both retention jobs honor it.
// Deletes activity_instances (CASCADEs to instance_staff + instance_students)
// and activity_exceptions older than the tenant's gdpr.timetable_retention_days.
// Per-tenant iteration via forEachTenantSettings; dedupe via lastTimetableCleanup.

// scheduleTimetableCleanupTask registers the shared daily timetable/calendar
// cleanup task when either cleanup service has been wired in.
func (s *Scheduler) scheduleTimetableCleanupTask() {
	if s.timetableCleanup == nil && s.calendarFeedCleanup == nil {
		s.getLogger().Info("timetable cleanup not configured")
		return
	}

	s.registerTask("timetable-cleanup", "1m-poll", s.runTimetableCleanupTaskPolling)
}

// runTimetableCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimetableCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimetableCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable cleanup task",
		"timetable cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunTimetableCleanup)
}

// checkAndRunTimetableCleanup evaluates each tenant's cleanup settings and
// runs timetable cleanup if the configured cleanup time matches now. Shares
// KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits cleanup task — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunTimetableCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastTimetableCleanup, "timetable-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running timetable GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		if s.timetableCleanup != nil {
			result, err := s.timetableCleanup.CleanupExpiredTimetableData(cleanupCtx)
			if err != nil {
				s.getLogger().Error("timetable cleanup failed for tenant",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("timetable cleanup for tenant %d: %w", tenantID, err)
			}

			if result.InstancesDeleted > 0 || result.ExceptionsDeleted > 0 {
				s.getLogger().Info("timetable cleanup completed for tenant",
					slog.Int64("tenant_id", tenantID),
					slog.Int("instances_deleted", result.InstancesDeleted),
					slog.Int("exceptions_deleted", result.ExceptionsDeleted),
					slog.Int("students_affected", result.StudentsAffected),
					slog.Int("retention_days", result.RetentionDays),
					slog.Int64("duration_ms", result.DurationMS),
				)
			}
		}

		if s.calendarFeedCleanup != nil {
			deleted, err := s.calendarFeedCleanup.CleanupExpiredFeedTombstones(cleanupCtx)
			if err != nil {
				return fmt.Errorf("calendar feed cleanup for tenant %d: %w", tenantID, err)
			}
			if deleted > 0 {
				s.getLogger().Info("calendar feed cleanup completed for tenant",
					slog.Int64("tenant_id", tenantID),
					slog.Int("tombstones_deleted", deleted),
				)
			}
		}
		return nil
	})
}

// --- Time-tracking GDPR cleanup (Tranche 0b) -----------------------------
//
// Daily cleanup of work_sessions + work_session_breaks + audit.work_session_edits
// + staff_absences older than the tenant's gdpr.time_tracking_retention_days.
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastTimeTrackingCleanup. Mirrors the timetable cleanup task one-to-one.

// scheduleTimeTrackingCleanupTask registers the daily time-tracking cleanup
// when a TimeTrackingCleanupService has been wired in. Nil → no task.
func (s *Scheduler) scheduleTimeTrackingCleanupTask() {
	if s.timeTrackingCleanup == nil {
		s.getLogger().Info("time-tracking GDPR cleanup not configured (no TimeTrackingCleanupService)")
		return
	}

	s.registerTask("time-tracking-cleanup", "1m-poll", s.runTimeTrackingCleanupTaskPolling)
}

// runTimeTrackingCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimeTrackingCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimeTrackingCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in time-tracking cleanup task",
		"time-tracking cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunTimeTrackingCleanup)
}

// checkAndRunTimeTrackingCleanup evaluates each tenant's cleanup settings and
// runs time-tracking cleanup if the configured cleanup time matches now.
// Shares KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits and timetable cleanup tasks — one admin switch for all
// nightly retention.
func (s *Scheduler) checkAndRunTimeTrackingCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastTimeTrackingCleanup, "time-tracking-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running time-tracking GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.timeTrackingCleanup.CleanupExpiredTimeTrackingData(cleanupCtx)
		if err != nil {
			s.getLogger().Error("time-tracking cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("time-tracking cleanup for tenant %d: %w", tenantID, err)
		}

		if result.SessionsDeleted > 0 || result.AbsencesDeleted > 0 {
			s.getLogger().Info("time-tracking cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_deleted", result.SessionsDeleted),
				slog.Int("absences_deleted", result.AbsencesDeleted),
				slog.Int("staff_affected", result.StaffAffected),
				slog.Int("retention_days", result.RetentionDays),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// --- Per-child change-history GDPR cleanup (issue #1455) ---
//
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastStudentChangeLogCleanup. Mirrors the time-tracking cleanup task.

// scheduleStudentChangeLogCleanupTask registers the daily change-history
// cleanup when a StudentChangeLogCleanupService has been wired in. Nil → no
// task.
func (s *Scheduler) scheduleStudentChangeLogCleanupTask() {
	if s.studentChangeLogCleanup == nil {
		s.getLogger().Info("student change-log GDPR cleanup not configured (no StudentChangeLogCleanupService)")
		return
	}

	s.registerTask("student-change-log-cleanup", "1m-poll", s.runStudentChangeLogCleanupTaskPolling)
}

// runStudentChangeLogCleanupTaskPolling ticks every minute and defers to
// checkAndRunStudentChangeLogCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runStudentChangeLogCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in student change-log cleanup task",
		"student change-log cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunStudentChangeLogCleanup)
}

// checkAndRunStudentChangeLogCleanup evaluates each tenant's cleanup settings
// and runs change-history cleanup if the configured cleanup time matches now.
// Shares the same data-cleanup toggle/time/timeout as the other retention
// jobs — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunStudentChangeLogCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastStudentChangeLogCleanup, "student-change-log-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running student change-log GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.studentChangeLogCleanup.CleanupExpiredChangeLog(cleanupCtx)
		if err != nil {
			s.getLogger().Error("student change-log cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("student change-log cleanup for tenant %d: %w", tenantID, err)
		}

		if result.EditsDeleted > 0 {
			s.getLogger().Info("student change-log cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("edits_deleted", result.EditsDeleted),
				slog.Int("students_affected", result.StudentsAffected),
				slog.Int("retention_days", result.RetentionDays),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// --- PWA standalone-usage GDPR cleanup (issue #2189) ---
//
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastPWAUsageCleanup. Mirrors the student change-log cleanup task.

// schedulePWAUsageCleanupTask registers the daily PWA usage cleanup when a
// pwa.UsageService has been wired in. Nil → no task.
func (s *Scheduler) schedulePWAUsageCleanupTask() {
	if s.pwaUsageCleanup == nil {
		s.getLogger().Info("pwa usage GDPR cleanup not configured (no pwa.UsageService)")
		return
	}

	s.registerTask("pwa-usage-cleanup", "1m-poll", s.runPWAUsageCleanupTaskPolling)
}

// runPWAUsageCleanupTaskPolling ticks every minute and defers to
// checkAndRunPWAUsageCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runPWAUsageCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in pwa usage cleanup task",
		"pwa usage cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunPWAUsageCleanup)
}

// checkAndRunPWAUsageCleanup evaluates each tenant's cleanup settings and
// sweeps stale standalone-usage rows when the configured cleanup time
// matches now. Shares the same data-cleanup toggle/time/timeout as the
// other retention jobs.
func (s *Scheduler) checkAndRunPWAUsageCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastPWAUsageCleanup, "pwa-usage-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.pwaUsageCleanup.CleanupExpiredUsage(cleanupCtx)
		if err != nil {
			s.getLogger().Error("pwa usage cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("pwa usage cleanup for tenant %d: %w", tenantID, err)
		}

		if result.RowsDeleted > 0 {
			s.getLogger().Info("pwa usage cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("cleanup_time", cleanupTime),
				slog.Int("rows_deleted", result.RowsDeleted),
				slog.Int("retention_days", result.RetentionDays),
			)
		}
		return nil
	})
}

// scheduleStaffMessageCleanupTask registers the daily retention sweep for the
// OGS-internal colleague chat (#2598) when a cleanup service is wired in.
// Nil → no task.
func (s *Scheduler) scheduleStaffMessageCleanupTask() {
	if s.staffMessageCleanup == nil {
		s.getLogger().Info("staff message GDPR cleanup not configured (no staffmessaging.CleanupService)")
		return
	}

	s.registerTask("staff-message-cleanup", "1m-poll", s.runStaffMessageCleanupTaskPolling)
}

// runStaffMessageCleanupTaskPolling ticks every minute and defers to
// checkAndRunStaffMessageCleanup.
func (s *Scheduler) runStaffMessageCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in staff message cleanup task",
		"staff message cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunStaffMessageCleanup)
}

// checkAndRunStaffMessageCleanup sweeps expired internal staff messages per
// tenant, sharing the same data-cleanup toggle/time/timeout as the other
// retention jobs.
func (s *Scheduler) checkAndRunStaffMessageCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastStaffMessageCleanup, "staff-message-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		timeoutMinutes, err := s.resolveRequiredPositiveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES")
		if err != nil {
			s.getLogger().Error("staff message cleanup timeout setting failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("key", configModel.KeyDataCleanupTimeoutMinutes),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("staff message cleanup timeout for tenant %d: %w", tenantID, err)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.staffMessageCleanup.CleanupExpiredMessages(cleanupCtx)
		if err != nil {
			s.getLogger().Error("staff message cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("staff message cleanup for tenant %d: %w", tenantID, err)
		}

		if result.MessagesDeleted > 0 || result.ThreadsDeleted > 0 {
			s.getLogger().Info("staff message cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("cleanup_time", cleanupTime),
				slog.Int("messages_deleted", result.MessagesDeleted),
				slog.Int("threads_deleted", result.ThreadsDeleted),
				slog.Int("retention_days", result.RetentionDays),
			)
		}
		return nil
	})
}
