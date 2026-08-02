package scheduler

import (
	"context"
	"log/slog"
	"time"
)

const staffDocumentFileCleanupInterval = 5 * time.Minute

// scheduleStaffDocumentFileCleanupTask registers recovery for files left by a
// process crash between the private-file write and metadata commit.
func (s *Scheduler) scheduleStaffDocumentFileCleanupTask() {
	if s.staffDocumentFileCleaner == nil {
		s.getLogger().Info("staff document file cleanup not configured")
		return
	}

	s.registerTask("staff-document-file-cleanup", "5m-poll", s.runStaffDocumentFileCleanupTask)
}

func (s *Scheduler) runStaffDocumentFileCleanupTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in staff document file cleanup task",
		"staff document file cleanup using interval polling",
		0, func() time.Duration { return staffDocumentFileCleanupInterval }, s.checkAndRunStaffDocumentFileCleanup)
}

func (s *Scheduler) checkAndRunStaffDocumentFileCleanup(task *ScheduledTask) {
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
	if err := s.forEachTenantIncludingInactive(ctx, "staff-document-file-cleanup",
		s.cleanupStaffDocumentFilesForTenant); err != nil {
		s.getLogger().Error("staff document file cleanup failed",
			slog.String("error", err.Error()),
		)
	}
}

// cleanupStaffDocumentFilesForTenant never propagates a per-file failure to the
// caller. The cleanup pass runs inside the tenant transaction opened by
// forEachTenantIncludingInactive, and the file removals it records are already
// irreversible on disk. Returning an error would roll that transaction back and
// drop the completion marks of every file that WAS removed, so those items would
// come back on the next run forever as long as one file keeps failing. Failures
// are logged instead; the offending row stays pending and is retried next pass.
func (s *Scheduler) cleanupStaffDocumentFilesForTenant(tenantCtx context.Context) error {
	removed, err := s.staffDocumentFileCleaner.CleanupOrphanedStaffDocumentFiles(tenantCtx)
	if removed > 0 {
		s.getLogger().Info("staff document file cleanup completed",
			slog.Int("files_removed", removed),
		)
	}
	if err != nil {
		s.getLogger().Error("staff document file cleanup incomplete",
			slog.Int("files_removed", removed),
			slog.String("error", err.Error()),
		)
	}
	return nil
}
