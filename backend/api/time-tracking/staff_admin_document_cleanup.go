package timetracking

import (
	"context"
	"errors"
	"fmt"
)

// CleanupOrphanedStaffDocumentFiles retries file removal independently of
// document UI traffic. The scheduler calls it in each tenant transaction
// after the upload grace period has elapsed.
func (rs *StaffAdminResource) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	removed := 0
	var cleanupErr error

	documents, err := rs.StaffDocumentService.ListDeletedStaffDocumentsPendingFileCleanups(ctx)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list deleted staff documents: %w", err))
	} else {
		pending := make([]pendingDocumentFile, 0, len(documents))
		for _, document := range documents {
			pending = append(pending, pendingDocumentFile{ID: document.ID, StaffID: document.StaffID, TenantID: document.TenantID, FilenameStored: document.FilenameStored})
		}
		count, err := rs.cleanupPendingDocumentFiles(ctx, pending, "deleted")
		removed += count
		cleanupErr = errors.Join(cleanupErr, err)
	}

	// Offboarding soft-deletes the staff row and unlinks its files after the
	// transaction commits; the document rows themselves stay active. A failed
	// or crashed unlink therefore leaves files the deleted-document pass above
	// never sees, and no UI route reaches a soft-deleted staff record, so this
	// pass is their only recovery path.
	offboarded, err := rs.StaffDocumentService.ListOffboardedStaffDocumentsPendingFileCleanup(ctx)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list offboarded staff documents: %w", err))
	} else {
		pending := make([]pendingDocumentFile, 0, len(offboarded))
		for _, document := range offboarded {
			pending = append(pending, pendingDocumentFile{ID: document.ID, StaffID: document.StaffID, TenantID: document.TenantID, FilenameStored: document.FilenameStored})
		}
		count, err := rs.cleanupPendingDocumentFiles(ctx, pending, "offboarded")
		removed += count
		cleanupErr = errors.Join(cleanupErr, err)
	}

	cleanups, err := rs.StaffDocumentService.ListQueuedStaffDocumentFileCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued staff document cleanups: %w", err))
	}
	for _, cleanup := range cleanups {
		if err := rs.removeStoredDocumentForTenant(cleanup.TenantID, cleanup.FilenameStored); err != nil {
			rs.logger.Warn("staff document orphan cleanup failed",
				"staff_id", cleanup.StaffID,
				"cleanup_id", cleanup.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanup.ID); err != nil {
			rs.logger.Error("staff document orphan cleanup status update failed",
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
// pendingDocumentFile is the projection of a staff document row that file
// cleanup needs. The HTTP layer must not import models/users (architecture
// policy: inbound-time-tracking/http may not import people-directory/domain),
// so the rows are projected where their type is inferred.
type pendingDocumentFile struct {
	ID             int64
	StaffID        int64
	TenantID       int64
	FilenameStored string
}

func (rs *StaffAdminResource) cleanupPendingDocumentFiles(ctx context.Context, documents []pendingDocumentFile, source string) (int, error) {
	removed := 0
	var cleanupErr error
	for _, document := range documents {
		if err := rs.removeStoredDocumentForTenant(document.TenantID, document.FilenameStored); err != nil {
			rs.logger.Warn("staff document cleanup failed",
				"staff_id", document.StaffID,
				"document_id", document.ID,
				"source", source,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.StaffDocumentService.MarkStaffDocumentFileDeleted(ctx, document.ID); err != nil {
			rs.logger.Error("staff document cleanup status update failed",
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
