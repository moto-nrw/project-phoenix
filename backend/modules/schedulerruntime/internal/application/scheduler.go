package application

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CleanupJob represents a single cleanup task that can be executed.
type CleanupJob struct {
	Description string
	Run         func(context.Context) (int, error)
}

var errWorkerTraceStart = errors.New("start worker trace")

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService              ports.SessionMaintenance
	cleanupService             ports.PresenceCleanup
	authCleanup                ports.AuthCleanup
	invitationCleanup          ports.InvitationCleaner
	workSessionCleanup         ports.WorkSessionCleaner
	breakAutoEnder             ports.BreakAutoEnder
	autoCheckouter             ports.AutoCheckouter
	feedbackCleaner            ports.FeedbackCleaner
	retention                  RetentionRun
	cleanupRuns                ports.CleanupRunLedger
	cleanupClock               func() time.Time
	cleanupFailures            sync.Map // cleanupAttemptKey → time.Time of the last failed attempt
	staffDocumentFileCleaner   ports.StaffDocumentFileCleaner
	studentDocumentFileCleaner ports.StudentDocumentFileCleaner
	fileStoreCleaner           ports.FileStoreCleaner
	materializer               ports.TimetableMaterializer
	timetableCleanup           ports.TimetableCleanup
	calendarFeedCleanup        ports.CalendarFeedCleaner
	timeTrackingCleanup        ports.TimeTrackingCleanupService
	studentChangeLogCleanup    ports.StudentChangeLogCleanup
	pwaUsageCleanup            ports.PWAUsageCleanup
	staffMessageCleanup        ports.StaffMessageCleanup
	bookingConsistency         ports.BookingConsistencyAudit
	enrollmentRejectedCleanup  ports.RejectedEnrollmentCleaner
	autoStart                  ports.InstanceAutoStart
	autoEnd                    ports.InstanceAutoEnd
	settings                   ports.SettingsResolver
	schoolRepo                 ports.TenantDirectory
	tenantRuntime              ports.TenantRuntime
	tenantRuntimeObserver      func(entryPoint, outcome string)
	unitOfWorkObserver         func(entryPoint, kind, result string, duration time.Duration, retries int)
	workerTracer               ports.WorkerTracer
	tenantBatchCursors         sync.Map // job ID → last successful tenant ID
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
	schedulerLifecycle
	// leadership gates every job run on the Worker lease (#2726). It keeps
	// renewing while Stop drains the jobs and releases the lease afterwards.
	jobsStarted atomic.Bool
	// done signals goroutines to stop when closed (replaces stored context)
	wg sync.WaitGroup

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
	instanceRepo         ports.DayInstanceReader
	instanceRoomRepo     ports.ExistingRoomReader
	instanceStudentRepo  ports.InstanceCheckoutCloser
	timetableBridge      ports.TimetableBridgeCompleter
	studentStatusDayRepo ports.StudentStatusFlagArchiver
	overdueBroadcaster   ports.InstanceOverdueBroadcaster
	overdueEmissionState
	now func() time.Time

	// Student lifecycle (parent-enrollment PR 2).
	// Nil → activate-students task does not register.
	studentLifecycleRepo  ports.StudentLifecycleRepository
	studentLifecycleAudit ports.StudentLifecycleAuditor
	careExitEffector      ports.CareExitEffector

	// Outbox worker (parent-enrollment PR 5).
	// Nil → outbox task does not register.
	outboxWorker           ports.OutboxWorkerRunner
	settleExpiredTransfers func(context.Context, int) (int, error)

	// Rollover deadline resolver (phase rollover slice 1).
	rolloverDeadlineRunner ports.RolloverDeadlineRunner

	// Personal reminder notifications (Reminder Delivery). Nil → the tick
	// does not register.
	reminderNotifications ports.ReminderNotificationTick

	// Guardian appointment reminders (#1671).
	// appointmentReminderScannedAt holds the upper bound of each tenant's last
	// successful scan. Failed scans deliberately leave their tenant's boundary
	// untouched so the next tick retries its bounded window.
	appointmentReminders         ports.AppointmentReminderCommand
	appointmentReminderScannedAt map[int64]time.Time
	appointmentReminderScanMu    sync.Mutex

	// Scheduled parent-announcement reminders (#3162). Same bookkeeping as the
	// appointment reminders: one successful-scan boundary per tenant.
	announcementReminders         ports.ParentAnnouncementReminderCommand
	announcementReminderScannedAt map[int64]time.Time
	announcementReminderScanMu    sync.Mutex
}

// overdueEmissionState is the instance-local re-fire guard, not worker configuration.
type overdueEmissionState struct {
	overdueEmitted      sync.Map
	overdueEmittedDay   calendar.Date
	overdueEmittedDayMu sync.Mutex
}

// overdueKey composites tenant + instance so the sync.Map key cannot collide
// across tenants. Using a struct literal avoids the ambiguity of a stringly-
// typed "tenant:instance" key if either id ever contained a colon.
type overdueKey struct {
	tenantID   int64
	instanceID int64
}

type schedulerLifecycle struct {
	lifecycleCtx  context.Context
	stopLifecycle context.CancelFunc
	leadership    *leadership
	done          chan struct{}
}

func newSchedulerLifecycle(deps ports.WorkerDependencies) schedulerLifecycle {
	ctx, stop := context.WithCancel(context.Background())
	return schedulerLifecycle{lifecycleCtx: ctx, stopLifecycle: stop, leadership: schedulerLeadership(deps.Lease, deps.Logger, ctx), done: make(chan struct{})}
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

// newScheduler builds the Worker from its complete typed dependencies in one
// literal; no dependency is wired after construction (#2749).
func newScheduler(deps ports.WorkerDependencies) *Scheduler {
	return &Scheduler{
		tasks:                         make(map[string]*ScheduledTask),
		schedulerLifecycle:            newSchedulerLifecycle(deps),
		logger:                        deps.Logger,
		getenv:                        deps.Getenv,
		now:                           deps.Now,
		appointmentReminderScannedAt:  make(map[int64]time.Time),
		announcementReminders:         deps.AppointmentReminders,
		announcementReminderScannedAt: make(map[int64]time.Time),
		cleanupRuns:                   deps.CleanupRuns,

		cleanupService:             deps.ActiveCleanup,
		authCleanup:                deps.AuthCleanup,
		invitationCleanup:          deps.InvitationCleanup,
		workSessionCleanup:         deps.WorkSessionCleanup,
		feedbackCleaner:            deps.FeedbackCleaner,
		staffDocumentFileCleaner:   deps.StaffDocumentCleaner,
		studentDocumentFileCleaner: deps.StudentDocumentCleaner,
		fileStoreCleaner:           deps.FileStoreCleaner,
		timetableCleanup:           deps.TimetableCleanup,
		calendarFeedCleanup:        deps.CalendarFeedCleanup,
		timeTrackingCleanup:        deps.TimeTrackingCleanup,
		studentChangeLogCleanup:    deps.StudentChangeLogCleanup,
		pwaUsageCleanup:            deps.PWAUsageCleanup,
		staffMessageCleanup:        deps.StaffMessageCleanup,
		enrollmentRejectedCleanup:  deps.EnrollmentRejectedCleanup,
		cleanupJobs:                schedulerCleanupJobs(deps),

		activeService:          deps.Active,
		breakAutoEnder:         deps.BreakAutoEnder,
		autoCheckouter:         deps.AutoCheckouter,
		materializer:           deps.Materializer,
		bookingConsistency:     deps.BookingConsistency,
		autoStart:              deps.AutoStart,
		autoEnd:                deps.AutoEnd,
		instanceRepo:           deps.InstanceRepo,
		instanceRoomRepo:       deps.InstanceRoomRepo,
		instanceStudentRepo:    deps.InstanceStudentRepo,
		timetableBridge:        deps.TimetableBridge,
		studentStatusDayRepo:   deps.StudentStatusDayRepo,
		overdueBroadcaster:     deps.OverdueBroadcaster,
		studentLifecycleRepo:   deps.StudentLifecycleRepo,
		studentLifecycleAudit:  deps.StudentLifecycleAudit,
		careExitEffector:       deps.CareExitEffector,
		outboxWorker:           deps.OutboxWorker,
		settleExpiredTransfers: deps.SettleExpiredTransfers,
		rolloverDeadlineRunner: deps.RolloverDeadlineRunner,
		reminderNotifications:  deps.ReminderNotifications,
		appointmentReminders:   deps.AppointmentReminders,

		settings:              deps.Settings,
		schoolRepo:            deps.SchoolRepo,
		tenantRuntime:         deps.TenantRuntime,
		tenantRuntimeObserver: deps.TenantRuntimeObserver,
		unitOfWorkObserver:    deps.UnitOfWorkObserver,
		workerTracer:          deps.Tracer,
	}
}

func schedulerCleanupJobs(deps ports.WorkerDependencies) []CleanupJob {
	jobs := buildCleanupJobs(deps.AuthCleanup, deps.InvitationCleanup, deps.EmailChangeCleanup, deps.OperatorInvitationCleanup)
	jobs = append(jobs, operatorRetentionJobs(deps.OperatorRetention)...)
	return append(jobs, accountRetentionJobs(deps.AccountRetention)...)
}

// getLogger returns the scheduler's logger, falling back to slog.Default() if nil.
func (s *Scheduler) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func (s *Scheduler) env(key string) string {
	if s.getenv == nil {
		return ""
	}
	return s.getenv(key)
}

// Start begins the scheduler. With a lease it first tries to lead once, so a
// sole Worker runs its startup checks; a standby starts its jobs too, and
// each run waits for the lease.
func (s *Scheduler) Start() {
	if s.registry == nil {
		panic("worker registry is required")
	}
	ids := s.registry.IDs()
	s.getLogger().Info("starting scheduler service",
		slog.Int("registered_job_count", len(ids)),
		slog.Any("registered_job_ids", ids),
	)
	if s.leadership != nil {
		s.leadership.start()
	}
	s.registry.Start()
	s.jobsStarted.Store(true)
}

// Stop gracefully stops the scheduler: running jobs are cancelled and
// drained while the lease stays renewed, then the lease is released.
func (s *Scheduler) Stop() {
	started := time.Now()
	s.getLogger().Info("stopping scheduler service")
	s.jobsStarted.Store(false)
	if s.stopLifecycle != nil {
		s.stopLifecycle()
	}
	close(s.done)
	s.wg.Wait()
	drain := time.Since(started)
	if s.leadership != nil {
		s.leadership.stop()
		s.leadership.drained(drain)
	}
	s.getLogger().Info("scheduler service stopped",
		slog.Duration("shutdown_drain_time", drain),
	)
}

// Ready reports whether this Worker runs jobs: it started them and holds
// the lease. A standby is alive but not ready.
func (s *Scheduler) Ready() bool {
	if s.leadership == nil || !s.jobsStarted.Load() {
		return false
	}
	_, _, held := s.leadership.current()
	return held
}

// jobContext returns the context a job run executes under. With a lease it
// is the running term's context, ended with the term, and every transaction
// opened under it asserts the term before it commits. ok is false while
// this Worker does not lead.
func (s *Scheduler) jobContext() (context.Context, bool) {
	if s.leadership == nil {
		return s.lifecycleContext(), true
	}
	term, termCtx, held := s.leadership.current()
	if !held {
		s.leadership.suppress("standby")
		return nil, false
	}
	if s.tenantRuntime == nil {
		return termCtx, true
	}
	return s.tenantRuntime.WithCommitGuard(termCtx, s.leadership.fence(term)), true
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

	s.wg.Go(func() { runner(task) })
}

// waitUntilNextMinute keeps the wall-clock alignment while checking again on
// takeover. It returns the next acquisition signal for the aligned tick loop.
func (s *Scheduler) waitUntilNextMinute(acquired <-chan struct{}, onAcquire func()) (<-chan struct{}, bool) {
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute) //nolint:forbidigo // sub-day minute alignment, not calendar-date math
	timer := time.NewTimer(time.Until(nextMinute))
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return acquired, true
		case <-acquired:
			acquired = s.leadership.nextAcquisition()
			onAcquire()
		case <-s.done:
			return acquired, false
		}
	}
}

// statusFlagClearHour and statusFlagClearMinute are the fixed end of the day
// for "Am Ende des Tages" (#3729). The time used to be a school setting; it is
// 18:00 for every school now. Midnight would not work: the clear archives the
// flag onto calendar.TodayDate(), which is already the next day at 00:00.
const (
	statusFlagClearHour   = 18
	statusFlagClearMinute = 0
)

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

// --- Timetable auto-start tick ---
//
// Purpose: when a tenant enables timetable.auto_start_planned, start planned
// activity instances whose configured time window is currently active. The
// service remains conservative: no assigned staff or any conflict warning means
// "do not start automatically"; staff can still start manually.

// --- Timetable auto-end tick ---

// --- Instance overdue tick (WP-B9) ---

//
// Purpose: emit instance_overdue (realtimeevents) once per planned instance that
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

// --- Timetable GDPR cleanup (WP-B14) ---
//
// Runs daily, gated on the SAME KeyDataCleanupEnabled toggle as the visits
// cleanup. Admins configure one nightly window; both retention jobs honor it.
// Deletes activity_instances (CASCADEs to instance_staff + instance_students)
// and activity_exceptions older than the tenant's gdpr.timetable_retention_days.
// Per-tenant iteration via forEachTenantSettings; dedupe via lastTimetableCleanup.

// --- Time-tracking GDPR cleanup (Tranche 0b) -----------------------------
//
// Daily cleanup of work_sessions + work_session_breaks + audit.work_session_edits
// + staff_absences older than the tenant's gdpr.time_tracking_retention_days.
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastTimeTrackingCleanup. Mirrors the timetable cleanup task one-to-one.

// --- Per-child change-history GDPR cleanup (issue #1455) ---
//
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastStudentChangeLogCleanup. Mirrors the time-tracking cleanup task.

// --- PWA standalone-usage GDPR cleanup (issue #2189) ---
//
// Per-tenant iteration via forEachTenantSettings; dedupe via
// lastPWAUsageCleanup. Mirrors the student change-log cleanup task.

func schedulerLeadership(lease ports.WorkerLease, logger *slog.Logger, ctx context.Context) *leadership {
	if isNilDependency(lease.Store) {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return newLeadership(lease, logger, ctx)
}
