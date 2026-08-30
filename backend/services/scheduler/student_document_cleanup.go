package scheduler

import (
	"context"
	"log/slog"
	"time"
)

const studentDocumentFileCleanupInterval = 5 * time.Minute

// scheduleStudentDocumentFileCleanupTask registers recovery for objects left
// behind by a process crash between the object write and the metadata commit,
// and for documents whose child was deleted before their bytes were removed.
func (s *Scheduler) scheduleStudentDocumentFileCleanupTask() {
	if s.studentDocumentFileCleaner == nil {
		s.getLogger().Info("student document file cleanup not configured")
		return
	}

	s.registerTask("student-document-file-cleanup", "5m-poll", s.runStudentDocumentFileCleanupTask)
}

func (s *Scheduler) runStudentDocumentFileCleanupTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in student document file cleanup task",
		"student document file cleanup using interval polling",
		0, func() time.Duration { return studentDocumentFileCleanupInterval }, s.checkAndRunStudentDocumentFileCleanup)
}

func (s *Scheduler) checkAndRunStudentDocumentFileCleanup(task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(5 * time.Minute)
	defer cancel()
	if err := s.forEachTenantIncludingInactive(ctx, "student-document-file-cleanup",
		s.cleanupStudentDocumentFilesForTenant); err != nil {
		s.getLogger().Error("student document file cleanup failed",
			slog.String("error", err.Error()),
		)
	}
}

// cleanupStudentDocumentFilesForTenant never propagates a per-object failure
// to the caller. The pass runs inside the tenant transaction opened by
// forEachTenantIncludingInactive, and the removals it records already happened
// on disk. Returning an error would roll that transaction back and drop the
// completion marks of every object that WAS removed, so those items would come
// back on every later run for as long as one object keeps failing. Failures
// are logged instead; the offending row stays pending and is retried next pass.
func (s *Scheduler) cleanupStudentDocumentFilesForTenant(tenantCtx context.Context) error {
	removed, err := s.studentDocumentFileCleaner.CleanupOrphanedStudentDocumentFiles(tenantCtx)
	if removed > 0 {
		s.getLogger().Info("student document file cleanup completed",
			slog.Int("files_removed", removed),
		)
	}
	if err != nil {
		s.getLogger().Error("student document file cleanup incomplete",
			slog.Int("files_removed", removed),
			slog.String("error", err.Error()),
		)
	}
	return nil
}
