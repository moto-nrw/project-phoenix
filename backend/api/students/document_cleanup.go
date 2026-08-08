package students

import (
	"context"
	"errors"
	"fmt"

	documentModels "github.com/moto-nrw/project-phoenix/models/documents"
)

// CleanupOrphanedStudentDocumentFiles retries object removal independently of
// document UI traffic. The scheduler calls it in each tenant transaction after
// the upload grace period has elapsed.
//
// It is the only recovery path for two cases the request-scoped hooks cannot
// cover: a process that died between the object write and the metadata commit,
// and a child who was deleted while some of their documents still had bytes on
// disk (the rows cascade away, the queued cleanup intents do not).
//
// Each pass is capped at documents.CleanupBatchSize per list. The cap is
// logged rather than silent: a pass that fills its batch has NOT reclaimed
// everything, and reading "cleanup completed" without that qualifier would be
// misleading. The next tick continues five minutes later.
func (rs *Resource) CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error) {
	coordinator, err := rs.studentDocumentCoordinator()
	if err != nil {
		return 0, fmt.Errorf("student document storage unavailable: %w", err)
	}

	removed := 0
	var cleanupErr error

	documents, err := rs.StudentDocumentService.ListDeletedStudentDocumentsPendingFileCleanups(ctx)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list deleted student documents: %w", err))
	} else {
		for _, document := range documents {
			if err := coordinator.Remove(ctx, document.TenantID, document.FilenameStored); err != nil {
				rs.getLogger().Warn("student document cleanup failed",
					"student_id", document.StudentID,
					"document_id", document.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			if err := rs.StudentDocumentService.MarkFileDeleted(ctx, document.ID); err != nil {
				rs.getLogger().Error("student document cleanup status update failed",
					"student_id", document.StudentID,
					"document_id", document.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			removed++
		}
		rs.logCleanupBatchFull(len(documents), "deleted")
	}

	cleanups, err := rs.StudentDocumentService.ListQueuedStudentDocumentFileCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued student document cleanups: %w", err))
	}
	for _, cleanup := range cleanups {
		if err := coordinator.Remove(ctx, cleanup.TenantID, cleanup.FilenameStored); err != nil {
			rs.getLogger().Warn("student document orphan cleanup failed",
				"student_id", cleanup.OwnerID,
				"cleanup_id", cleanup.ID,
				"error", err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.StudentDocumentService.MarkQueuedCleanupComplete(ctx, cleanup.ID); err != nil {
			rs.getLogger().Error("student document orphan cleanup status update failed",
				"student_id", cleanup.OwnerID,
				"cleanup_id", cleanup.ID,
				"error", err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}
	rs.logCleanupBatchFull(len(cleanups), "orphan")

	return removed, cleanupErr
}

// logCleanupBatchFull reports a pass that filled its batch. Without it a
// bounded sweep would read as "everything reclaimed" in the logs while rows
// were still waiting — a silent cap is worse than no cap.
func (rs *Resource) logCleanupBatchFull(count int, source string) {
	if count < documentModels.CleanupBatchSize {
		return
	}
	rs.getLogger().Info("student document cleanup batch full, more pending",
		"batch_size", documentModels.CleanupBatchSize,
		"source", source)
}
