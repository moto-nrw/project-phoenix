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
	if err := s.forEachTenantIncludingInactive(ctx, "staff-document-file-cleanup", func(tenantCtx context.Context) error {
		removed, err := s.staffDocumentFileCleaner.CleanupOrphanedStaffDocumentFiles(tenantCtx)
		if err != nil {
			return err
		}
		if removed > 0 {
			s.getLogger().Info("staff document file cleanup completed",
				slog.Int("files_removed", removed),
			)
		}
		return nil
	}); err != nil {
		s.getLogger().Error("staff document file cleanup failed",
			slog.String("error", err.Error()),
		)
	}
}
