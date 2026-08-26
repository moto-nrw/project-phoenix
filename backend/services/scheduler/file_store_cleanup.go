package scheduler

import (
	"context"
	"log/slog"
	"time"
)

const fileStoreCleanupInterval = 5 * time.Minute

// scheduleFileStoreCleanupTask registers recovery for objects of the school
// file storage (#2596) left behind by a process crash between the object
// write and the metadata commit, and for files whose folder was deleted
// before their bytes were removed.
func (s *Scheduler) scheduleFileStoreCleanupTask() {
	if s.fileStoreCleaner == nil {
		s.getLogger().Info("file store cleanup not configured")
		return
	}

	s.registerTask("file-store-cleanup", "5m-poll", s.runFileStoreCleanupTask)
}

func (s *Scheduler) runFileStoreCleanupTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in file store cleanup task",
		"file store cleanup using interval polling",
		0, func() time.Duration { return fileStoreCleanupInterval }, s.checkAndRunFileStoreCleanup)
}

func (s *Scheduler) checkAndRunFileStoreCleanup(task *ScheduledTask) {
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
	if err := s.forEachTenantIncludingInactive(ctx, "file-store-cleanup",
		s.cleanupFileStoreForTenant); err != nil {
		s.getLogger().Error("file store cleanup failed",
			slog.String("error", err.Error()),
		)
	}
}

// cleanupFileStoreForTenant never propagates a per-object failure: the
// removals it recorded already happened on disk, and an error would roll back
// their completion marks. Failures are logged; the row is retried next pass.
func (s *Scheduler) cleanupFileStoreForTenant(tenantCtx context.Context) error {
	removed, err := s.fileStoreCleaner.CleanupOrphanedFiles(tenantCtx)
	if removed > 0 {
		s.getLogger().Info("file store cleanup completed",
			slog.Int("files_removed", removed),
		)
	}
	if err != nil {
		s.getLogger().Error("file store cleanup incomplete",
			slog.Int("files_removed", removed),
			slog.String("error", err.Error()),
		)
	}
	return nil
}
