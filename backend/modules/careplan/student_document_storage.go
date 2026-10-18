package careplan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// StudentDocumentStorageKind is the storage key prefix for child documents.
const StudentDocumentStorageKind = "student-documents"

// StudentDocumentFileRemoval opens the removal of stored child document
// objects of one kind over the document service's cleanup bookkeeping. The
// root binds File Storage's document coordinator; the storage backend is
// looked up on each call.
type StudentDocumentFileRemoval func(kind string, documents StudentDocuments, logger *slog.Logger) (StudentDocumentFileRemover, error)

// StudentDocumentFileCleanup is the scheduler's recovery pass over the stored
// objects of child documents.
type StudentDocumentFileCleanup struct {
	documents StudentDocuments
	removal   StudentDocumentFileRemoval
	logger    *slog.Logger
}

// NewStudentDocumentFileCleanup builds the recovery pass over the child
// document service and the stored-object removal. Every dependency is
// required.
func NewStudentDocumentFileCleanup(documents StudentDocuments, removal StudentDocumentFileRemoval, logger *slog.Logger) (*StudentDocumentFileCleanup, error) {
	if documents == nil || removal == nil || logger == nil {
		return nil, errors.New("care plan student document file cleanup: all dependencies are required")
	}
	return &StudentDocumentFileCleanup{documents: documents, removal: removal, logger: logger}, nil
}

// CleanupOrphanedStudentDocumentFiles runs one recovery pass in the tenant
// transaction of ctx; see CleanupOrphanedStudentDocumentFiles.
func (c *StudentDocumentFileCleanup) CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error) {
	return CleanupOrphanedStudentDocumentFiles(ctx, c.documents, c.removal, c.logger)
}

// CleanupOrphanedStudentDocumentFiles retries object removal independently of
// document UI traffic. The scheduler calls it in each tenant transaction after
// the upload grace period has elapsed.
//
// It is the only recovery path for two cases the request-scoped hooks cannot
// cover: a process that died between the object write and the metadata commit,
// and a child who was deleted while some of their documents still had bytes on
// disk (the rows cascade away, the queued cleanup intents do not).
//
// Each pass is capped at StudentDocumentSweepBatchSize per list. The cap is
// logged rather than silent: a pass that fills its batch has NOT reclaimed
// everything, and reading "cleanup completed" without that qualifier would be
// misleading. The next tick continues five minutes later.
func CleanupOrphanedStudentDocumentFiles(ctx context.Context, documents StudentDocuments, removal StudentDocumentFileRemoval, logger *slog.Logger) (int, error) {
	if removal == nil {
		return 0, fmt.Errorf("student document storage unavailable: %w", errors.New("student document storage is not configured"))
	}
	remove, err := removal(StudentDocumentStorageKind, documents, logger)
	if err != nil {
		return 0, fmt.Errorf("student document storage unavailable: %w", err)
	}
	sweep, err := documents.SweepStudentDocumentFiles(ctx, remove)
	cleanupErr := err
	for _, failure := range sweep.Failures {
		logStudentDocumentCleanupFailure(logger, failure)
		cleanupErr = errors.Join(cleanupErr, failure.Err)
	}
	logStudentDocumentCleanupBatchFull(logger, sweep.DeletedListed, "deleted")
	logStudentDocumentCleanupBatchFull(logger, sweep.OrphansListed, "orphan")
	return sweep.Removed, cleanupErr
}

// logStudentDocumentCleanupFailure reports one item the sweep left for the
// next pass. A failed removal is a storage hiccup; a removed object whose row
// could not be settled is an inconsistency worth an error.
func logStudentDocumentCleanupFailure(logger *slog.Logger, failure StudentDocumentFileFailure) {
	source, idKey, id := "student document cleanup", "document_id", failure.DocumentID
	if failure.DocumentID == 0 {
		source, idKey, id = "student document orphan cleanup", "cleanup_id", failure.CleanupID
	}
	if failure.Stage == StudentDocumentSweepRemove {
		logger.Warn(source+" failed",
			"student_id", failure.StudentID,
			idKey, id,
			"error", failure.Err)
		return
	}
	logger.Error(source+" status update failed",
		"student_id", failure.StudentID,
		idKey, id,
		"error", failure.Err)
}

// logStudentDocumentCleanupBatchFull reports a pass that filled its batch.
// Without it a bounded sweep would read as "everything reclaimed" in the logs
// while rows were still waiting — a silent cap is worse than no cap.
func logStudentDocumentCleanupBatchFull(logger *slog.Logger, count int, source string) {
	if count < StudentDocumentSweepBatchSize {
		return
	}
	logger.Info("student document cleanup batch full, more pending",
		"batch_size", StudentDocumentSweepBatchSize,
		"source", source)
}
