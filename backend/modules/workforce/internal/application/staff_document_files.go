package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// staffDocumentFiles removes the stored bytes of staff documents below the
// public upload directory.
type staffDocumentFiles struct {
	documents workforce.StaffDocumentFileCleanups
	queue     workforce.DocumentCleanupQueue
	uploads   workforce.StaffDocumentUploads
	logger    *slog.Logger
}

// NewStaffDocumentFileRecovery builds the staff document file removal over
// the document service, the offboarding cleanup queue and the upload
// directory. Every dependency is required.
func NewStaffDocumentFileRecovery(documents workforce.StaffDocumentFileCleanups, queue workforce.DocumentCleanupQueue, uploads workforce.StaffDocumentUploads, logger *slog.Logger) (workforce.StaffDocumentFileRecovery, error) {
	if documents == nil || queue == nil || uploads == nil || logger == nil {
		return nil, errors.New("workforce staff document files: all dependencies are required")
	}
	return &staffDocumentFiles{documents: documents, queue: queue, uploads: uploads, logger: logger}, nil
}

// RemoveStoredDocument deletes one stored file of the tenant.
func (f *staffDocumentFiles) RemoveStoredDocument(tenantID int64, storedName string) error {
	if !validStoredDocumentName(storedName) {
		return errors.New("stored document filename failed validation on delete")
	}
	dir := workforce.StaffDocumentsDir(tenantID, f.uploads)
	if err := os.Remove(filepath.Join(dir, storedName)); err != nil && !os.IsNotExist(err) {
		return errors.New("failed to remove stored document")
	}
	return nil
}

// validStoredDocumentName accepts a plain file name: not empty, no "..", no
// directory part.
func validStoredDocumentName(name string) bool {
	return name != "" && !strings.Contains(name, "..") && filepath.Base(name) == name
}

// CleanupOrphanedStaffDocumentFiles retries file removal independently of
// document UI traffic. The scheduler calls it in each tenant transaction
// after the upload grace period has elapsed.
func (f *staffDocumentFiles) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	offboardedStaff, err := f.enqueueOffboardedStaff(ctx)
	if err != nil {
		return 0, err
	}
	removed, err := f.cleanupDeletedDocuments(ctx, offboardedStaff)
	cleanupErr := errors.Join(err)
	cleanups, err := f.documents.ListQueuedStaffDocumentFileCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued staff document cleanups: %w", err))
	}
	count, err := f.cleanupQueuedUploads(ctx, cleanups)
	removed += count
	cleanupErr = errors.Join(cleanupErr, err)

	if _, ambient := tenant.TransactionFromContext(ctx); ambient {
		// Recovery producers above must commit before a worker can claim.
		f.WakeOffboardingDocumentCleanup(ctx)
		return removed, cleanupErr
	}
	count, err = f.RunOffboardingDocumentCleanup(ctx)
	return removed + count, errors.Join(cleanupErr, err)
}

// enqueueOffboardedStaff queues the offboarding cleanup of every offboarded
// staff member with files still pending and reports who they are.
func (f *staffDocumentFiles) enqueueOffboardedStaff(ctx context.Context) (map[int64]bool, error) {
	offboarded, err := f.documents.ListOffboardedStaffDocumentsPendingFileCleanup(ctx)
	if err != nil {
		return nil, fmt.Errorf("list offboarded staff documents: %w", err)
	}
	offboardedStaff := make(map[int64]bool)
	for _, document := range offboarded {
		if offboardedStaff[document.StaffID] {
			continue
		}
		offboardedStaff[document.StaffID] = true
		if err := f.queue.Enqueue(ctx, document.StaffID); err != nil {
			return nil, err
		}
	}
	return offboardedStaff, nil
}

// cleanupDeletedDocuments removes the files of deleted documents; the
// offboarding cleanup owns those of offboarded staff.
func (f *staffDocumentFiles) cleanupDeletedDocuments(ctx context.Context, offboardedStaff map[int64]bool) (int, error) {
	documents, err := f.documents.ListDeletedStaffDocumentsPendingFileCleanups(ctx)
	if err != nil {
		return 0, fmt.Errorf("list deleted staff documents: %w", err)
	}
	pending := make([]workforce.StaffDocument, 0, len(documents))
	for _, document := range documents {
		if offboardedStaff[document.StaffID] {
			continue
		}
		pending = append(pending, document)
	}
	return f.cleanupPendingDocumentFiles(ctx, pending, "deleted")
}

// cleanupQueuedUploads removes the files of uploads whose metadata never
// committed and marks each cleanup it finished.
func (f *staffDocumentFiles) cleanupQueuedUploads(ctx context.Context, cleanups []workforce.StaffDocumentFileCleanup) (int, error) {
	removed := 0
	var cleanupErr error
	for _, cleanup := range cleanups {
		if err := f.RemoveStoredDocument(cleanup.TenantID, cleanup.FilenameStored); err != nil {
			f.logger.Warn("staff document orphan cleanup failed",
				"staff_id", cleanup.StaffID,
				"cleanup_id", cleanup.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := f.documents.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanup.ID); err != nil {
			f.logger.Error("staff document orphan cleanup status update failed",
				"staff_id", cleanup.StaffID,
				"cleanup_id", cleanup.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}
	return removed, cleanupErr
}

// cleanupPendingDocumentFiles removes the stored bytes of documents whose
// file_deleted_at is still unset and stamps the ones it removed. A per-file
// failure is collected and skipped so the remaining documents still get their
// completion mark in this pass.
func (f *staffDocumentFiles) cleanupPendingDocumentFiles(ctx context.Context, documents []workforce.StaffDocument, source string) (int, error) {
	removed := 0
	var cleanupErr error
	for _, document := range documents {
		if err := f.RemoveStoredDocument(document.TenantID, document.FilenameStored); err != nil {
			f.logger.Warn("staff document cleanup failed",
				"staff_id", document.StaffID,
				"document_id", document.ID,
				"source", source,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := f.documents.MarkStaffDocumentFileDeleted(ctx, document.ID); err != nil {
			f.logger.Error("staff document cleanup status update failed",
				"staff_id", document.StaffID,
				"document_id", document.ID,
				"source", source,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}
	return removed, cleanupErr
}

// WakeOffboardingDocumentCleanup runs the offboarding cleanup after the
// transaction of ctx commits.
func (f *staffDocumentFiles) WakeOffboardingDocumentCleanup(ctx context.Context) {
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	tenant.RegisterAfterCommit(ctx, func() { f.runWokenOffboardingDocumentCleanup(cleanupCtx) })
}

// runWokenOffboardingDocumentCleanup runs the cleanup a commit woke, bounded
// to one minute, and reports a pass it could not finish.
func (f *staffDocumentFiles) runWokenOffboardingDocumentCleanup(ctx context.Context) {
	workerCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if _, err := f.RunOffboardingDocumentCleanup(workerCtx); err != nil {
		f.logger.Error("offboarding document cleanup incomplete", "error", err)
	}
}

// RunOffboardingDocumentCleanup claims and runs due offboarding cleanups.
func (f *staffDocumentFiles) RunOffboardingDocumentCleanup(ctx context.Context) (int, error) {
	claims, err := f.queue.Claim(ctx, 100)
	if err != nil {
		return 0, err
	}
	removed := 0
	var failures error
	for _, claim := range claims {
		count, cleanupErr := f.cleanupClaimedStaffDocuments(ctx, claim.StaffID)
		removed += count
		finished, finishErr := f.queue.Finish(ctx, claim, cleanupErr == nil)
		if finishErr == nil && !finished {
			finishErr = errors.New("offboarding document cleanup lease expired")
		}
		failures = errors.Join(failures, cleanupErr, finishErr)
		if cleanupErr != nil || finishErr != nil {
			f.logger.Warn("offboarding document cleanup retry pending",
				"staff_id", claim.StaffID,
				"attempt", claim.Attempts,
			)
		}
	}
	backlog, err := f.queue.Backlog(ctx)
	if err == nil {
		f.logger.Debug("offboarding document cleanup backlog",
			"pending", backlog.Pending,
			"oldest_age_seconds", backlog.OldestAgeSeconds,
		)
	}
	return removed, errors.Join(failures, err)
}

func (f *staffDocumentFiles) cleanupClaimedStaffDocuments(ctx context.Context, staffID int64) (int, error) {
	var documents []workforce.StaffDocument
	var uploads []workforce.StaffDocumentFileCleanup
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var err error
		documents, err = f.documents.ListStaffDocumentsPendingFileCleanup(txCtx, staffID)
		if err != nil {
			return err
		}
		uploads, err = f.documents.ListQueuedStaffDocumentFileCleanup(txCtx, staffID)
		return err
	})
	if err != nil {
		return 0, err
	}
	removed := 0
	var failures error
	for _, document := range documents {
		if err := f.RemoveStoredDocument(document.TenantID, document.FilenameStored); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		if err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			return f.documents.MarkStaffDocumentFileDeleted(txCtx, document.ID)
		}); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		removed++
	}
	for _, upload := range uploads {
		if err := f.RemoveStoredDocument(upload.TenantID, upload.FilenameStored); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		if err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			return f.documents.MarkQueuedStaffDocumentFileCleanupComplete(txCtx, upload.ID)
		}); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		removed++
	}
	if failures != nil {
		return removed, fmt.Errorf("offboarding staff document cleanup: %w", failures)
	}
	return removed, nil
}
