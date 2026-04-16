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
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/active"
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

// SettingsResolver resolves setting values per tenant. Implemented by config.SettingsService.
type SettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService      active.Service
	cleanupService     active.CleanupService
	authCleanup        AuthCleanup
	invitationCleanup  InvitationCleaner
	workSessionCleanup WorkSessionCleaner
	breakAutoEnder     BreakAutoEnder
	feedbackCleaner    FeedbackCleaner
	settings           SettingsResolver
	db                 *bun.DB
	schoolRepo         platform.SchoolRepository
	cleanupJobs        []CleanupJob
	tasks              map[string]*ScheduledTask
	mu                 sync.RWMutex
	logger             *slog.Logger
	// done signals goroutines to stop when closed (replaces stored context)
	done chan struct{}
	wg   sync.WaitGroup

	// Session cleanup configuration (parsed once during initialization)
	sessionCleanupIntervalMinutes    int
	sessionAbandonedThresholdMinutes int

	// Break auto-end configuration (parsed once during initialization)
	breakAutoEndIntervalSeconds int

	// Per-tenant tracking for minute-polling (keyed by tenant ID)
	lastSessionEnd      sync.Map // tenant_id → time.Time
	lastDataCleanup     sync.Map // tenant_id → time.Time
	lastSessionCleanup  sync.Map // tenant_id → time.Time
	lastStatusFlagClear sync.Map // tenant_id → time.Time
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

// forEachTenant executes fn for each active tenant inside a WithTenantTx.
// If schoolRepo or db is not set, falls back to running fn with plain ctx (non-tenant-aware mode).
func (s *Scheduler) forEachTenant(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if s.db == nil || s.schoolRepo == nil {
		s.getLogger().Warn("tenant iteration not configured, running without tenant context",
			slog.String("operation", opName))
		return fn(ctx)
	}

	// List active tenants using admin context
	var schools []platform.School
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, tx bun.Tx) error {
		var listErr error
		schools, listErr = s.schoolRepo.ListActive(txCtx)
		return listErr
	})
	if err != nil {
		return fmt.Errorf("scheduler: list active tenants for %s: %w", opName, err)
	}

	for _, school := range schools {
		tenantErr := tenant.WithTenantTx(ctx, s.db, school.ID, func(txCtx context.Context, tx bun.Tx) error {
			return fn(txCtx)
		})
		if tenantErr != nil {
			s.getLogger().Error("tenant operation failed, continuing to next tenant",
				slog.String("operation", opName),
				slog.Int64("tenant_id", school.ID),
				slog.Any("error", tenantErr))
			continue
		}
	}

	return nil
}

// forEachTenantSettings executes fn for each active tenant, passing tenant ID for settings resolution.
// Falls back to non-tenant-aware mode if schoolRepo/db is not set.
func (s *Scheduler) forEachTenantSettings(ctx context.Context, opName string, fn func(ctx context.Context, tenantID int64) error) {
	if s.db == nil || s.schoolRepo == nil {
		s.getLogger().Warn("tenant iteration not configured, running without tenant context",
			slog.String("operation", opName))
		_ = fn(ctx, 0)
		return
	}

	var schools []platform.School
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		var listErr error
		schools, listErr = s.schoolRepo.ListActive(txCtx)
		return listErr
	})
	if err != nil {
		s.getLogger().Error("failed to list active tenants",
			slog.String("operation", opName),
			slog.String("error", err.Error()),
		)
		return
	}

	for _, school := range schools {
		tenantErr := tenant.WithTenantTx(ctx, s.db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			return fn(txCtx, school.ID)
		})
		if tenantErr != nil {
			s.getLogger().Error("tenant operation failed, continuing to next tenant",
				slog.String("operation", opName),
				slog.Int64("tenant_id", school.ID),
				slog.Any("error", tenantErr),
			)
		}
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.getLogger().Info("starting scheduler service")

	// Schedule daily data cleanup at 2 AM
	s.scheduleCleanupTask()

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
		if result.SessionsEnded > 0 {
			s.getLogger().Info("session end completed",
				slog.Int("sessions_ended", result.SessionsEnded),
				slog.Int("visits_ended", result.VisitsEnded),
			)
		}
		return nil
	})
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

		s.getLogger().Info("session end completed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("sessions_ended", result.SessionsEnded),
			slog.Int("visits_ended", result.VisitsEnded),
			slog.Int("supervisors_ended", result.SupervisorsEnded),
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
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
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
// tenant's configured operations.student_daily_checkout_time (the natural
// end of the OGS day); when that setting is empty, no clear happens.
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

// runStatusFlagClearTaskPolling checks every minute if any tenant's checkout
// time matches now and clears the configured end_of_day flags.
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
// clears flags when the configured daily checkout time matches now.
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
		clearTime := s.resolveStringSetting(tenantCtx, configModel.KeyStudentDailyCheckoutTime, "", "")
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
	if s.db == nil {
		return 0, fmt.Errorf("scheduler db not configured")
	}
	query := fmt.Sprintf(
		`UPDATE users.students SET %s = FALSE, %s = NULL WHERE %s = TRUE`,
		flagColumn, sinceColumn, flagColumn,
	)
	res, err := s.db.NewRaw(query).Exec(ctx)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}
