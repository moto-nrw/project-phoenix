package staff

import (
	"context"
	"errors"
	"fmt"
)

// CleanupOrphanedStaffDocumentFiles retries file removal independently of
// document UI traffic. The scheduler calls it in each tenant transaction
// after the upload grace period has elapsed.
func (rs *Resource) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	removed := 0
	var cleanupErr error

	documents, err := rs.StaffDocumentService.ListDeletedStaffDocumentsPendingFileCleanups(ctx)
	if err != nil {
		return 0, fmt.Errorf("list deleted staff documents: %w", err)
	}
	for _, document := range documents {
		if err := rs.removeStoredDocumentForTenant(document.TenantID, document.FilenameStored); err != nil {
			rs.getLogger().Warn("deleted staff document cleanup failed",
				"staff_id", document.StaffID,
				"document_id", document.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.StaffDocumentService.MarkStaffDocumentFileDeleted(ctx, document.ID); err != nil {
			rs.getLogger().Error("deleted staff document cleanup status update failed",
				"staff_id", document.StaffID,
				"document_id", document.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}

	cleanups, err := rs.StaffDocumentService.ListQueuedStaffDocumentFileCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued staff document cleanups: %w", err))
	}
	for _, cleanup := range cleanups {
		if err := rs.removeStoredDocumentForTenant(cleanup.TenantID, cleanup.FilenameStored); err != nil {
			rs.getLogger().Warn("staff document orphan cleanup failed",
				"staff_id", cleanup.StaffID,
				"cleanup_id", cleanup.ID,
				"error", err,
			)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanup.ID); err != nil {
			rs.getLogger().Error("staff document orphan cleanup status update failed",
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
