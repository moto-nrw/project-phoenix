package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
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

// WorkSessionCleaner exposes the cleanup routine for stale work sessions.
type WorkSessionCleaner interface {
	CleanupOpenSessions(ctx context.Context) (int, error)
}

// BreakAutoEnder exposes the method to auto-end expired breaks.
type BreakAutoEnder interface {
	AutoEndExpiredBreaks(ctx context.Context) (int, error)
}

// EmailChangeTokenCleaner exposes the cleanup routine for email change tokens.
type EmailChangeTokenCleaner interface {
	CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error)
}

// OperatorInvitationCleaner exposes the cleanup routine for operator invitation tokens.
type OperatorInvitationCleaner interface {
	CleanupExpiredOperatorInvitations(ctx context.Context) (int, error)
}

// FeedbackCleaner exposes the cleanup routine for old feedback entries.
type FeedbackCleaner interface {
	DeleteEntriesOlderThan(ctx context.Context, days int) (int, error)
}

type UnregisteredTagScanCleaner interface {
	DeleteOlderThan(ctx context.Context, days int) (int, error)
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
	feedbackCleaner            FeedbackCleaner
	unregisteredTagScanCleaner UnregisteredTagScanCleaner
	materializer               scheduleSvc.MaterializationService
	timetableCleanup           scheduleSvc.TimetableCleanupService
	timeTrackingCleanup        active.TimeTrackingCleanupService
	studentChangeLogCleanup    usersSvc.StudentChangeLogCleanupService
	autoStart                  scheduleSvc.AutoStartService
	settings                   SettingsResolver
	db                         *bun.DB
	schoolRepo                 platform.SchoolRepository
	cleanupJobs                []CleanupJob
	tasks                      map[string]*ScheduledTask
	mu                         sync.RWMutex
	logger                     *slog.Logger
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

	// Overdue instance tracking (WP-B9). Re-fire guard so the same instance
	// does not emit `instance_overdue` every minute for the same planned
	// row. Cleared explicitly on day boundary; see checkAndRunOverdue.
	instanceRepo         scheduleModel.ActivityInstanceRepository
	instanceStudentRepo  scheduleModel.InstanceStudentRepository
	studentStatusDayRepo activeModel.StudentStatusDayRepository
	overdueBroadcaster   realtime.Broadcaster
	overdueEmitted       sync.Map // overdueKey{tenantID, instanceID} → time.Time
	overdueEmittedDay    timezone.Date
	overdueEmittedDayMu  sync.Mutex

	// Student lifecycle (parent-enrollment PR 2). Wired via SetStudentLifecycleRepo.
	// Nil → activate-students task does not register.
	studentLifecycleRepo  StudentLifecycleRepository
	studentLifecycleAudit StudentLifecycleAuditor

	// Outbox worker (parent-enrollment PR 5). Wired via SetOutboxWorker.
	// Nil → outbox task does not register.
	outboxWorker OutboxWorkerRunner

	// Rollover deadline resolver (phase rollover slice 1). Wired via
	// SetRolloverDeadlineRunner. Nil → task does not register.
	rolloverDeadlineRunner RolloverDeadlineRunner
}

// OutboxWorkerRunner is the narrow contract the scheduler needs from the
// platform outbox worker. Defined here so the scheduler doesn't import
// services/platform.
type OutboxWorkerRunner interface {
	RunOnce(ctx context.Context, batchSize int) (int, error)
	SetMaxAttempts(n int)
}

// SetOutboxWorker wires the outbox worker. Nil disables the outbox task.
func (s *Scheduler) SetOutboxWorker(w OutboxWorkerRunner) {
	s.outboxWorker = w
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

// NewScheduler creates a new scheduler
func NewScheduler(activeService active.Service, cleanupService active.CleanupService, authService AuthCleanup, invitationService InvitationCleaner, emailChangeCleaner EmailChangeTokenCleaner, operatorInvitationCleaner OperatorInvitationCleaner, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		activeService:     activeService,
		cleanupService:    cleanupService,
		authCleanup:       authService,
		invitationCleanup: invitationService,
		cleanupJobs:       buildCleanupJobs(authService, invitationService, emailChangeCleaner, operatorInvitationCleaner),
		tasks:             make(map[string]*ScheduledTask),
		done:              make(chan struct{}),
		logger:            logger,
	}
}

// getLogger returns the scheduler's logger, falling back to slog.Default() if nil.
func (s *Scheduler) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// SetWorkSessionCleaner sets the work session cleanup service (optional).
func (s *Scheduler) SetWorkSessionCleaner(wsc WorkSessionCleaner) {
	s.workSessionCleanup = wsc
}

// SetBreakAutoEnder sets the break auto-end service (optional).
func (s *Scheduler) SetBreakAutoEnder(bae BreakAutoEnder) {
	s.breakAutoEnder = bae
}

// SetFeedbackCleaner sets the feedback cleanup service (optional).
func (s *Scheduler) SetFeedbackCleaner(fc FeedbackCleaner) {
	s.feedbackCleaner = fc
}

func (s *Scheduler) SetUnregisteredTagScanCleaner(cleaner UnregisteredTagScanCleaner) {
	s.unregisteredTagScanCleaner = cleaner
}

// SetMaterializer wires the timetable materialization service. When set, the
// scheduler registers the weekly materialization task in Start(). A nil
// materializer is a valid configuration — the task simply does not register,
// matching the legacy "opt-in via dependency injection" pattern used by the
// feedback cleaner, work-session cleaner, and break auto-ender above.
func (s *Scheduler) SetMaterializer(m scheduleSvc.MaterializationService) {
	s.materializer = m
}

// SetTimetableCleanup wires the timetable GDPR cleanup service (WP-B14). When
// set, the scheduler registers the daily timetable-cleanup task in Start().
// Nil is a valid configuration — the task simply does not register, matching
// the opt-in shape of SetMaterializer.
func (s *Scheduler) SetTimetableCleanup(svc scheduleSvc.TimetableCleanupService) {
	s.timetableCleanup = svc
}

// SetTimeTrackingCleanup wires the time-tracking retention cleanup service
// (Tranche 0b). Same opt-in shape as SetTimetableCleanup — nil is fine, the
// task simply doesn't register in Start().
func (s *Scheduler) SetTimeTrackingCleanup(svc active.TimeTrackingCleanupService) {
	s.timeTrackingCleanup = svc
}

// SetStudentChangeLogCleanup wires the per-child change-history retention
// cleanup service (issue #1455). Same opt-in shape — nil is fine, the task
// simply doesn't register in Start().
func (s *Scheduler) SetStudentChangeLogCleanup(svc usersSvc.StudentChangeLogCleanupService) {
	s.studentChangeLogCleanup = svc
}

// SetAutoStartService wires the planned-instance auto-start service. When set,
// the scheduler registers a minute-polled task gated by timetable.enabled and
// timetable.auto_start_planned.
func (s *Scheduler) SetAutoStartService(svc scheduleSvc.AutoStartService) {
	s.autoStart = svc
}

// SetDB sets the database connection for tenant-aware operations.
func (s *Scheduler) SetDB(db *bun.DB) {
	s.db = db
}

// SetSchoolRepo sets the school repository for tenant iteration.
func (s *Scheduler) SetSchoolRepo(repo platform.SchoolRepository) {
	s.schoolRepo = repo
}

// SetSettingsService sets the settings resolver for per-tenant configuration.
// When set, the scheduler reads per-tenant settings instead of global env vars.
func (s *Scheduler) SetSettingsService(svc SettingsResolver) {
	s.settings = svc
}

// SetInstanceOverdueDeps wires the dependencies for the WP-B9 overdue
// instance tick. Both parameters are required; passing nil for either
// disables the tick entirely (no task registers, no SSE events fire). Same
// opt-in shape as SetMaterializer: a partial wiring is never a silent
// misconfiguration.
func (s *Scheduler) SetInstanceOverdueDeps(repo scheduleModel.ActivityInstanceRepository, broadcaster realtime.Broadcaster) {
	s.instanceRepo = repo
	s.overdueBroadcaster = broadcaster
}

// SetTimetableBridgeRepos wires the schedule-side repositories used by the
// daily session-end bridge (completeTimetableInstancesForEndedSessions).
// Independent of the overdue-tick wiring: it also sets instanceRepo so the
// bridge works even when SetInstanceOverdueDeps was never called. Without
// this wiring the bridge is a no-op.
func (s *Scheduler) SetTimetableBridgeRepos(instanceStudents scheduleModel.InstanceStudentRepository, instances scheduleModel.ActivityInstanceRepository) {
	s.instanceStudentRepo = instanceStudents
	s.instanceRepo = instances
}

// SetStudentStatusDayRepo wires the repository used by the nightly
// status-flag clear (archive to student_status_days + clear legacy flags).
func (s *Scheduler) SetStudentStatusDayRepo(repo activeModel.StudentStatusDayRepository) {
	s.studentStatusDayRepo = repo
}

// forEachTenant executes fn for each active tenant inside a WithTenantTx.
// If schoolRepo or db is not set, falls back to running fn with plain ctx (non-tenant-aware mode).
// Thin wrapper over tenant.ForEachActive; the scheduler-owned fallback to non-tenant-aware mode
// is preserved so existing tests that do not wire a school repo keep working.
func (s *Scheduler) forEachTenant(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if s.db == nil || s.schoolRepo == nil {
		s.getLogger().Warn("tenant iteration not configured, running without tenant context",
			slog.String("operation", opName))
		return fn(ctx)
	}

	return tenant.ForEachActive(ctx, s.db, s.schoolRepo, s.getLogger(), opName, func(txCtx context.Context, _ int64) error {
		return fn(txCtx)
	})
}

// forEachTenantSettings executes fn for each active tenant, passing tenant ID for settings resolution.
// Falls back to non-tenant-aware mode if schoolRepo/db is not set (tests, local dev without
// seeded schools). Shared with the CLI via tenant.ForEachActive.
func (s *Scheduler) forEachTenantSettings(ctx context.Context, opName string, fn func(ctx context.Context, tenantID int64) error) {
	if s.db == nil || s.schoolRepo == nil {
		s.getLogger().Warn("tenant iteration not configured, running without tenant context",
			slog.String("operation", opName))
		_ = fn(ctx, 0)
		return
	}

	if err := tenant.ForEachActive(ctx, s.db, s.schoolRepo, s.getLogger(), opName, fn); err != nil {
		s.getLogger().Error("failed to list active tenants",
			slog.String("operation", opName),
			slog.String("error", err.Error()),
		)
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.getLogger().Info("starting scheduler service")

	// Schedule daily data cleanup at 2 AM
	s.scheduleCleanupTask()

	// Schedule daily timetable GDPR cleanup (WP-B14). Shares the same toggle
	// and time as scheduleCleanupTask so admins configure one nightly window
	// for all retention jobs.
	s.scheduleTimetableCleanupTask()

	// Schedule daily time-tracking GDPR cleanup (Tranche 0b). Same toggle
	// (gdpr.data_cleanup_enabled) and same cleanup-time as the other
	// retention jobs so admins have a single nightly window to configure.
	s.scheduleTimeTrackingCleanupTask()

	// Schedule daily per-child change-history GDPR cleanup (issue #1455).
	// Same toggle + cleanup-time as the other retention jobs.
	s.scheduleStudentChangeLogCleanupTask()

	// Schedule daily session end at configurable time (default 6 PM)
	s.scheduleSessionEndTask()

	// Schedule token cleanup every hour
	s.scheduleTokenCleanupTask()

	// Schedule abandoned session cleanup
	s.scheduleSessionCleanupTask()

	// Schedule break auto-end task
	s.scheduleBreakAutoEndTask()

	// Schedule daily sick/excused status-flag clear task
	s.scheduleStatusFlagClearTask()

	// Schedule weekly timetable materialization (WP-B8)
	s.scheduleMaterializationTask()

	// Schedule minute-polled overdue instance tick (WP-B9)
	s.scheduleInstanceOverdueTask()

	// Schedule minute-polled automatic starts for planned instances.
	s.scheduleAutoStartTask()

	// Schedule per-tenant activate-students tick (parent-enrollment PR 2)
	s.scheduleActivateStudentsTask()

	// Schedule platform email outbox worker (parent-enrollment PR 5)
	s.scheduleOutboxWorkerTask()

	// Schedule per-tenant rollover deadline resolver (phase rollover)
	s.scheduleRolloverDeadlineTask()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.getLogger().Info("stopping scheduler service")
	close(s.done)
	s.wg.Wait()
	s.getLogger().Info("scheduler service stopped")
}

// scheduleCleanupTask schedules the daily cleanup task using minute-polling.
// Each minute, it checks each tenant's configured cleanup time and fires if matched.
func (s *Scheduler) scheduleCleanupTask() {
	// Env var can globally disable cleanup regardless of settings service
	if os.Getenv("CLEANUP_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("cleanup scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require explicit opt-in via env var
	if s.settings == nil && os.Getenv("CLEANUP_SCHEDULER_ENABLED") != "true" {
		s.getLogger().Info("cleanup scheduler is disabled")
		return
	}

	task := &ScheduledTask{
		Name:     "visit-cleanup",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runCleanupTaskPolling(task)
}

// runCleanupTaskPolling checks every minute if any tenant's cleanup time matches now.
func (s *Scheduler) runCleanupTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("cleanup task using minute-polling for per-tenant scheduling")

	// Immediate check on startup so we don't miss the current minute after a restart.
	s.checkAndRunCleanup(task)

	// Align to the next minute boundary so ticks land at HH:MM:00.
	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunCleanup(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunCleanup evaluates each tenant's cleanup settings and runs if time matches.
func (s *Scheduler) checkAndRunCleanup(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "cleanup-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyDataCleanupEnabled, "CLEANUP_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		cleanupTime := s.resolveStringSetting(tenantCtx, configModel.KeyDataCleanupTime, "CLEANUP_SCHEDULER_TIME", "02:00")
		if !timeMatchesNow(cleanupTime) {
			return nil
		}

		if wasRunToday(&s.lastDataCleanup, tenantID) {
			return nil
		}

		// Mark immediately to prevent double-fire from concurrent ticks
		markRunToday(&s.lastDataCleanup, tenantID)

		s.getLogger().Info("running data cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		if !s.executeCleanupForTenant(cleanupCtx, tenantID) {
			// Clear mark so cleanup retries on next matching minute
			s.lastDataCleanup.Delete(tenantID)
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

	// Cleanup old feedback entries based on data retention setting
	if s.feedbackCleaner != nil {
		retentionDays := s.resolveIntSetting(ctx, configModel.KeyFeedbackDataRetentionDays, "", 90)
		if deleted, err := s.feedbackCleaner.DeleteEntriesOlderThan(ctx, retentionDays); err != nil {
			s.getLogger().Error("feedback cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		} else if deleted > 0 {
			s.getLogger().Info("feedback cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_deleted", deleted),
				slog.Int("retention_days", retentionDays),
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

	return true
}

// executeCleanup runs the cleanup task for all tenants (backward-compatible wrapper).
// Used by existing tests and the legacy code path.
func (s *Scheduler) executeCleanup(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		s.getLogger().Warn("cleanup task already running, skipping")
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(24 * time.Hour)
		task.mu.Unlock()
	}()

	timeoutMinutes := s.resolveIntSetting(context.Background(), configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	_ = s.forEachTenant(ctx, "cleanup-visits", func(tenantCtx context.Context) error {
		s.executeCleanupForTenant(tenantCtx, 0)
		return nil
	})
}

// executeSessionEnd runs session end for all tenants (backward-compatible wrapper).
// Used by existing tests and the legacy code path.
func (s *Scheduler) executeSessionEnd(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		s.getLogger().Warn("session end task already running, skipping")
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(24 * time.Hour)
		task.mu.Unlock()
	}()

	timeoutMinutes := s.resolveIntSetting(context.Background(), configModel.KeySessionEndTimeoutMinutes, "SESSION_END_TIMEOUT_MINUTES", 10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	_ = s.forEachTenant(ctx, "session-end", func(tenantCtx context.Context) error {
		result, err := s.activeService.EndDailySessions(tenantCtx)
		if err != nil {
			s.getLogger().Error("session end failed", "error", err)
			return nil
		}
		timetableCompleted, err := s.completeTimetableInstancesForEndedSessions(tenantCtx, result)
		if err != nil {
			s.getLogger().Error("session end timetable sync failed",
				slog.String("error", err.Error()),
			)
			return err
		}
		if result.SessionsEnded > 0 {
			s.getLogger().Info("session end completed",
				slog.Int("sessions_ended", result.SessionsEnded),
				slog.Int("visits_ended", result.VisitsEnded),
				slog.Int("timetable_instances_completed", timetableCompleted),
			)
		}
		return nil
	})
}

// completeTimetableInstancesForEndedSessions closes the schedule-side rows
// linked to active.groups that the daily session-end job just closed.
//
// The active service owns active.groups/visits/supervisors; timetable
// instances live in schedule.*, so the scheduler bridges the two inside the
// tenant transaction created by ForEachActive. If this sync fails, callers
// return the error so the active close rolls back too instead of leaving the
// planner in a stale "active" state.
func (s *Scheduler) completeTimetableInstancesForEndedSessions(ctx context.Context, result *active.DailySessionCleanupResult) (int, error) {
	if result == nil || len(result.EndedActiveGroupIDs) == 0 {
		return 0, nil
	}
	if s.instanceStudentRepo == nil || s.instanceRepo == nil {
		return 0, nil
	}

	now := time.Now()

	if err := s.instanceStudentRepo.MarkExpectedAbsentByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now); err != nil {
		return 0, fmt.Errorf("mark expected timetable students absent: %w", err)
	}

	rows, err := s.instanceRepo.CompleteActiveByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now)
	if err != nil {
		return 0, fmt.Errorf("complete active timetable instances: %w", err)
	}
	return int(rows), nil
}

// executeSessionCleanup runs session cleanup for all tenants (backward-compatible wrapper).
// Used by existing tests. Parameters are kept for signature compatibility.
func (s *Scheduler) executeSessionCleanup(task *ScheduledTask, _ int, thresholdMinutes int) {
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
		task.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	threshold := time.Duration(thresholdMinutes) * time.Minute
	_ = s.forEachTenant(ctx, "session-cleanup", func(tenantCtx context.Context) error {
		count, err := s.activeService.CleanupAbandonedSessions(tenantCtx, threshold)
		if err != nil {
			return err
		}
		if count > 0 {
			s.getLogger().Info("session cleanup completed",
				slog.Int("abandoned_sessions", count),
			)
		}
		return nil
	})
}

// scheduleTokenCleanupTask schedules hourly token cleanup
func (s *Scheduler) scheduleTokenCleanupTask() {
	task := &ScheduledTask{
		Name:     "token-cleanup",
		Schedule: "1h", // Run every hour
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runTokenCleanupTask(task)
}

// runTokenCleanupTask runs the token cleanup task on schedule
func (s *Scheduler) runTokenCleanupTask(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in token cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("token cleanup task scheduled to run every hour")

	// Run immediately on startup
	s.executeTokenCleanup(task)

	// Then run every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeTokenCleanup(task)
		case <-s.done:
			return
		}
	}
}

// executeTokenCleanup executes the token cleanup task
func (s *Scheduler) executeTokenCleanup(task *ScheduledTask) {
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

	s.getLogger().Info("running scheduled token cleanup")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if err := s.RunCleanupJobs(); err != nil {
		s.getLogger().Error("token cleanup failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	s.getLogger().Info("token cleanup completed",
		slog.Duration("duration", duration.Round(time.Millisecond)))
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	if len(s.cleanupJobs) == 0 {
		s.getLogger().Info("no cleanup jobs registered, skipping token cleanup")
		return nil
	}

	ctx := context.Background()
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
			s.getLogger().Error("cleanup job failed",
				slog.String("job", job.Description),
				slog.Any("error", err),
			)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		s.getLogger().Info("cleanup job completed",
			slog.String("job", job.Description),
			slog.Int("records_deleted", count))
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
	if os.Getenv("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require non-false env var
	if s.settings == nil && os.Getenv("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled")
		return
	}

	task := &ScheduledTask{
		Name:     "session-end",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runSessionEndTaskPolling(task)
}

// runSessionEndTaskPolling checks every minute if any tenant's session end time matches now.
func (s *Scheduler) runSessionEndTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in session end task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("session end task using minute-polling for per-tenant scheduling")

	// Immediate check on startup so we don't miss the current minute after a restart.
	s.checkAndRunSessionEnd(task)

	// Align to the next minute boundary so ticks land at HH:MM:00.
	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunSessionEnd(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunSessionEnd evaluates each tenant's session end settings and runs if time matches.
func (s *Scheduler) checkAndRunSessionEnd(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
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

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeySessionEndTimeoutMinutes, "SESSION_END_TIMEOUT_MINUTES", 10)
		endCtx, endCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer endCancel()

		result, err := s.activeService.EndDailySessions(endCtx)
		if err != nil {
			s.getLogger().Error("session end failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return nil // don't fail other tenants
		}
		timetableCompleted, err := s.completeTimetableInstancesForEndedSessions(endCtx, result)
		if err != nil {
			s.getLogger().Error("session end timetable sync failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return err
		}

		s.getLogger().Info("session end completed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("sessions_ended", result.SessionsEnded),
			slog.Int("visits_ended", result.VisitsEnded),
			slog.Int("supervisors_ended", result.SupervisorsEnded),
			slog.Int("timetable_instances_completed", timetableCompleted),
		)

		markRunToday(&s.lastSessionEnd, tenantID)
		return nil
	})
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task.
// Uses a fixed 5-minute polling interval, resolves per-tenant settings on each tick.
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Quick check: if globally disabled via env and no settings service, skip
	if s.settings == nil && os.Getenv("SESSION_CLEANUP_ENABLED") == "false" {
		s.getLogger().Info("session cleanup is disabled")
		return
	}

	// Parse global defaults (used as fallback and for backward-compatible struct fields)
	s.sessionCleanupIntervalMinutes = 15
	if envInterval := os.Getenv("SESSION_CLEANUP_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			s.sessionCleanupIntervalMinutes = parsed
		}
	}
	s.sessionAbandonedThresholdMinutes = 60
	if envThreshold := os.Getenv("SESSION_ABANDONED_THRESHOLD_MINUTES"); envThreshold != "" {
		if parsed, err := strconv.Atoi(envThreshold); err == nil && parsed > 0 {
			s.sessionAbandonedThresholdMinutes = parsed
		}
	}

	task := &ScheduledTask{
		Name:     "session-cleanup",
		Schedule: "5m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runSessionCleanupTaskPolling(task)
}

// runSessionCleanupTaskPolling checks every 5 minutes if any tenant needs session cleanup.
func (s *Scheduler) runSessionCleanupTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in session cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("session cleanup using 5-minute polling for per-tenant scheduling")

	// Brief delay on startup to let services initialize
	time.Sleep(30 * time.Second)
	s.checkAndRunSessionCleanup(task)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunSessionCleanup(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunSessionCleanup evaluates each tenant's session cleanup settings.
func (s *Scheduler) checkAndRunSessionCleanup(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
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

	if os.Getenv("BREAK_AUTO_END_ENABLED") == "false" {
		s.getLogger().Info("break auto-end is disabled via env var")
		return
	}

	// Resolve interval from env var (global, not per-tenant)
	s.breakAutoEndIntervalSeconds = 60
	if val := os.Getenv("BREAK_AUTO_END_INTERVAL_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			s.breakAutoEndIntervalSeconds = parsed
		}
	}

	task := &ScheduledTask{
		Name:     "break-auto-end",
		Schedule: fmt.Sprintf("%ds-poll", s.breakAutoEndIntervalSeconds),
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runBreakAutoEndTaskPolling(task)
}

// runBreakAutoEndTaskPolling runs break auto-end check at the configured interval for all tenants.
func (s *Scheduler) runBreakAutoEndTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in break auto-end task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	interval := time.Duration(s.breakAutoEndIntervalSeconds) * time.Second
	s.getLogger().Info("break auto-end polling started",
		slog.Int("interval_seconds", s.breakAutoEndIntervalSeconds),
	)

	// Brief delay on startup
	time.Sleep(10 * time.Second)
	s.checkAndRunBreakAutoEnd(task)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunBreakAutoEnd(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunBreakAutoEnd runs break auto-end for all tenants.
func (s *Scheduler) checkAndRunBreakAutoEnd(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tenant-scoped: auto-end expired breaks
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

// --- Settings-aware helpers ---
//
// Fallback chain: tenant DB override → env var → registry default.
// The settings service's Resolve* returns the registry default when no tenant
// override exists, which would skip the env var. We use HasTenantOverride to
// distinguish "tenant explicitly set a value" from "returning registry default".

// resolveStringSetting resolves a setting via the settings service with env var fallback.
func (s *Scheduler) resolveStringSetting(ctx context.Context, key string, envVar string, defaultVal string) string {
	if s.settings != nil {
		if hasOverride, err := s.settings.HasTenantOverride(ctx, key); err != nil {
			s.getLogger().Warn("settings override check failed, falling back",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		} else if hasOverride {
			if val, err := s.settings.ResolveString(ctx, key); err == nil && val != "" {
				return val
			}
		}
	}
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	return defaultVal
}

// resolveBoolSetting resolves a boolean setting via the settings service with env var fallback.
func (s *Scheduler) resolveBoolSetting(ctx context.Context, key string, envVar string, defaultVal bool) bool {
	if s.settings != nil {
		if hasOverride, err := s.settings.HasTenantOverride(ctx, key); err != nil {
			s.getLogger().Warn("settings override check failed, falling back",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		} else if hasOverride {
			if val, err := s.settings.ResolveBool(ctx, key); err == nil {
				return val
			}
		}
	}
	if val := os.Getenv(envVar); val != "" {
		return val == "true"
	}
	return defaultVal
}

// resolveIntSetting resolves an integer setting via the settings service with env var fallback.
func (s *Scheduler) resolveIntSetting(ctx context.Context, key string, envVar string, defaultVal int) int {
	if s.settings != nil {
		if hasOverride, err := s.settings.HasTenantOverride(ctx, key); err != nil {
			s.getLogger().Warn("settings override check failed, falling back",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		} else if hasOverride {
			if val, err := s.settings.ResolveInt(ctx, key); err == nil && val > 0 {
				return val
			}
		}
	}
	if val := os.Getenv(envVar); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultVal
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
	val, ok := lastRunMap.Load(tenantID)
	if !ok {
		return false
	}
	lastRun, ok := val.(time.Time)
	if !ok {
		return false
	}
	now := time.Now()
	return lastRun.Year() == now.Year() && lastRun.YearDay() == now.YearDay()
}

// markRunToday records that a per-tenant job ran today.
func markRunToday(lastRunMap *sync.Map, tenantID int64) {
	lastRunMap.Store(tenantID, time.Now())
}

// scheduleStatusFlagClearTask schedules a daily task to clear sick / excused
// flags for tenants whose operations.sick_clear_mode or
// operations.excused_clear_mode is set to "end_of_day". The task fires at the
// tenant's configured operations.status_flag_clear_time.
func (s *Scheduler) scheduleStatusFlagClearTask() {
	// Env var kill switch to allow ops to disable this task without code changes.
	if os.Getenv("STATUS_FLAG_CLEAR_ENABLED") == "false" {
		s.getLogger().Info("status flag clear scheduler is disabled via env var")
		return
	}

	task := &ScheduledTask{
		Name:     "status-flag-clear",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runStatusFlagClearTaskPolling(task)
}

// runStatusFlagClearTaskPolling checks every minute if any tenant's status
// flag clear time matches now and clears the configured end_of_day flags.
func (s *Scheduler) runStatusFlagClearTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in status flag clear task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("status flag clear task using minute-polling for per-tenant scheduling")

	s.checkAndRunStatusFlagClear(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunStatusFlagClear(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunStatusFlagClear evaluates each tenant's clear_mode settings and
// clears flags when the configured status flag clear time matches now.
func (s *Scheduler) checkAndRunStatusFlagClear(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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

	task := &ScheduledTask{
		Name:     "timetable-materialization",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runMaterializationTaskPolling(task)
}

// runMaterializationTaskPolling ticks every minute and delegates to
// checkAndRunMaterialization. Minute alignment matches the other scheduler
// tasks so HH:MM:00 ticks land deterministically.
func (s *Scheduler) runMaterializationTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in materialization task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("timetable materialization using minute-polling for per-tenant scheduling")

	// Startup check — covers the case of the server booting on the scheduled
	// weekday after the minute has already passed.
	s.checkAndRunMaterialization(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunMaterialization(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunMaterialization iterates active tenants and fires materialization
// for each tenant whose configured weekday matches today's ISO weekday, gated
// on the timetable.materialization_enabled setting.
func (s *Scheduler) checkAndRunMaterialization(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "materialization-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyTimetableMaterializationEnabled, "", true)
		if !enabled {
			return nil
		}

		// Registry default is 5 (Friday, ISO 8601). The helper goes through
		// HasTenantOverride → ResolveInt → env → default, exactly matching
		// the documented fallback pattern.
		targetWeekday := s.resolveIntSetting(tenantCtx, configModel.KeyTimetableMaterializationWeekday, "", 5)
		if !isoWeekdayMatchesNow(targetWeekday) {
			return nil
		}

		if wasRunToday(&s.lastMaterialization, tenantID) {
			return nil
		}
		markRunToday(&s.lastMaterialization, tenantID)

		weeksAhead := s.resolveIntSetting(tenantCtx, configModel.KeyTimetableMaterializationWeeksAhead, "", 1)
		from, to := s.materializer.ResolveWindow(timezone.TodayDate(), weeksAhead)

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

// isoWeekdayMatchesNow returns true if wd (1=Mon…7=Sun) matches today's ISO
// weekday. Wall-clock local time is used because the per-tenant timezone is
// not part of WP-B8 — default materialization day is "Friday wherever the
// server is", good enough for a German-only deployment.
func isoWeekdayMatchesNow(wd int) bool {
	today := time.Now().Weekday()
	if today == time.Sunday {
		return wd == 7
	}
	return wd == int(today)
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

	task := &ScheduledTask{
		Name:     "timetable-auto-start",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runAutoStartTaskPolling(task)
}

func (s *Scheduler) runAutoStartTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in timetable auto-start task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("timetable auto-start tick using minute-polling")

	s.checkAndRunAutoStart(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunAutoStart(task)
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) checkAndRunAutoStart(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
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

// scheduleInstanceOverdueTask registers the tick when both dependencies are
// wired. No repo or no broadcaster → no task; matches SetMaterializer's
// opt-in pattern so partial wiring is never a silent misconfiguration.
func (s *Scheduler) scheduleInstanceOverdueTask() {
	if s.instanceRepo == nil || s.overdueBroadcaster == nil {
		s.getLogger().Info("instance overdue tick not configured (missing repo or broadcaster)")
		return
	}

	task := &ScheduledTask{
		Name:     "instance-overdue",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runInstanceOverdueTaskPolling(task)
}

// runInstanceOverdueTaskPolling mirrors the minute-polling loops used by
// cleanup / session-end / materialization. Startup check + minute alignment
// + 60 s ticker + done-signal exit.
func (s *Scheduler) runInstanceOverdueTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in instance overdue task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("instance overdue tick using minute-polling")

	s.checkAndRunOverdue(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunOverdue(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunOverdue rotates the day-cache when midnight has been crossed,
// then iterates active tenants and delegates the per-tenant work to
// runOverdueForTenant. Extracted into two methods so unit tests can invoke
// the inner loop directly with a synthetic tenant ctx without building the
// full school repo + settings service stack.
func (s *Scheduler) checkAndRunOverdue(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned {
			continue
		}
		instanceStart := combineDayAndTime(today, inst.StartTime)
		if now.Sub(instanceStart) < cutoff {
			continue
		}
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
	return time.Date(day.Year, day.Month, day.Day,
		tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.Local)
}

// --- Timetable GDPR cleanup (WP-B14) ---
//
// Runs daily, gated on the SAME KeyDataCleanupEnabled toggle as the visits
// cleanup. Admins configure one nightly window; both retention jobs honor it.
// Deletes activity_instances (CASCADEs to instance_staff + instance_students)
// and activity_exceptions older than the tenant's gdpr.timetable_retention_days.
// Per-tenant iteration via forEachTenantSettings; dedupe via lastTimetableCleanup.

// scheduleTimetableCleanupTask registers the daily timetable cleanup task
// when a TimetableCleanupService has been wired in. Nil service → no task.
func (s *Scheduler) scheduleTimetableCleanupTask() {
	if s.timetableCleanup == nil {
		s.getLogger().Info("timetable GDPR cleanup not configured (no TimetableCleanupService)")
		return
	}

	task := &ScheduledTask{
		Name:     "timetable-cleanup",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runTimetableCleanupTaskPolling(task)
}

// runTimetableCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimetableCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimetableCleanupTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in timetable cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("timetable cleanup task using minute-polling for per-tenant scheduling")

	s.checkAndRunTimetableCleanup(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunTimetableCleanup(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunTimetableCleanup evaluates each tenant's cleanup settings and
// runs timetable cleanup if the configured cleanup time matches now. Shares
// KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits cleanup task — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunTimetableCleanup(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "timetable-cleanup-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyDataCleanupEnabled, "CLEANUP_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		cleanupTime := s.resolveStringSetting(tenantCtx, configModel.KeyDataCleanupTime, "CLEANUP_SCHEDULER_TIME", "02:00")
		if !timeMatchesNow(cleanupTime) {
			return nil
		}

		if wasRunToday(&s.lastTimetableCleanup, tenantID) {
			return nil
		}

		// Mark immediately to prevent double-fire from concurrent ticks
		markRunToday(&s.lastTimetableCleanup, tenantID)

		s.getLogger().Info("running timetable GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, configModel.KeyDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.timetableCleanup.CleanupExpiredTimetableData(cleanupCtx)
		if err != nil {
			s.getLogger().Error("timetable cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			// Clear today-mark so retry on next matching minute succeeds.
			s.lastTimetableCleanup.Delete(tenantID)
			return nil
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

	task := &ScheduledTask{
		Name:     "time-tracking-cleanup",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runTimeTrackingCleanupTaskPolling(task)
}

// runTimeTrackingCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimeTrackingCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimeTrackingCleanupTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in time-tracking cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("time-tracking cleanup task using minute-polling for per-tenant scheduling")

	s.checkAndRunTimeTrackingCleanup(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunTimeTrackingCleanup(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunTimeTrackingCleanup evaluates each tenant's cleanup settings and
// runs time-tracking cleanup if the configured cleanup time matches now.
// Shares KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits and timetable cleanup tasks — one admin switch for all
// nightly retention.
func (s *Scheduler) checkAndRunTimeTrackingCleanup(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "time-tracking-cleanup-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyDataCleanupEnabled, "CLEANUP_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		cleanupTime := s.resolveStringSetting(tenantCtx, configModel.KeyDataCleanupTime, "CLEANUP_SCHEDULER_TIME", "02:00")
		if !timeMatchesNow(cleanupTime) {
			return nil
		}

		if wasRunToday(&s.lastTimeTrackingCleanup, tenantID) {
			return nil
		}

		// Mark immediately to prevent double-fire from concurrent ticks.
		markRunToday(&s.lastTimeTrackingCleanup, tenantID)

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
			// Clear today-mark so retry on next matching minute succeeds.
			s.lastTimeTrackingCleanup.Delete(tenantID)
			return nil
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

	task := &ScheduledTask{
		Name:     "student-change-log-cleanup",
		Schedule: "1m-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runStudentChangeLogCleanupTaskPolling(task)
}

// runStudentChangeLogCleanupTaskPolling ticks every minute and defers to
// checkAndRunStudentChangeLogCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runStudentChangeLogCleanupTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in student change-log cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("student change-log cleanup task using minute-polling for per-tenant scheduling")

	s.checkAndRunStudentChangeLogCleanup(task)

	if !s.waitUntilNextMinute() {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunStudentChangeLogCleanup(task)
		case <-s.done:
			return
		}
	}
}

// checkAndRunStudentChangeLogCleanup evaluates each tenant's cleanup settings
// and runs change-history cleanup if the configured cleanup time matches now.
// Shares the same data-cleanup toggle/time/timeout as the other retention
// jobs — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunStudentChangeLogCleanup(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	s.forEachTenantSettings(ctx, "student-change-log-cleanup-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, configModel.KeyDataCleanupEnabled, "CLEANUP_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		cleanupTime := s.resolveStringSetting(tenantCtx, configModel.KeyDataCleanupTime, "CLEANUP_SCHEDULER_TIME", "02:00")
		if !timeMatchesNow(cleanupTime) {
			return nil
		}

		if wasRunToday(&s.lastStudentChangeLogCleanup, tenantID) {
			return nil
		}

		// Mark immediately to prevent double-fire from concurrent ticks.
		markRunToday(&s.lastStudentChangeLogCleanup, tenantID)

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
			// Clear today-mark so retry on next matching minute succeeds.
			s.lastStudentChangeLogCleanup.Delete(tenantID)
			return err
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
