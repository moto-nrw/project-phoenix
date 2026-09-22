package students

import (
	"context"
	"errors"
	"fmt"

	documentModels "github.com/moto-nrw/project-phoenix/models/documents"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
	sweep, err := rs.StudentDocumentService.SweepStudentDocumentFiles(ctx, coordinator.Remove)
	cleanupErr := err
	for _, failure := range sweep.Failures {
		rs.logCleanupFailure(failure)
		cleanupErr = errors.Join(cleanupErr, failure.Err)
	}
	rs.logCleanupBatchFull(sweep.DeletedListed, "deleted")
	rs.logCleanupBatchFull(sweep.OrphansListed, "orphan")
	return sweep.Removed, cleanupErr
}

// logCleanupFailure reports one item the sweep left for the next pass. A
// failed removal is a storage hiccup; a removed object whose row could not be
// settled is an inconsistency worth an error.
func (rs *Resource) logCleanupFailure(failure careplan.StudentDocumentFileFailure) {
	source, idKey, id := "student document cleanup", "document_id", failure.DocumentID
	if failure.DocumentID == 0 {
		source, idKey, id = "student document orphan cleanup", "cleanup_id", failure.CleanupID
	}
	if failure.Stage == careplan.StudentDocumentSweepRemove {
		rs.getLogger().Warn(source+" failed",
			"student_id", failure.StudentID,
			idKey, id,
			"error", failure.Err)
		return
	}
	rs.getLogger().Error(source+" status update failed",
		"student_id", failure.StudentID,
		idKey, id,
		"error", failure.Err)
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
