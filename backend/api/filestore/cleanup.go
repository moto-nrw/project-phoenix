package filestore

import (
	"context"
	"errors"
	"fmt"

	documentModels "github.com/moto-nrw/project-phoenix/models/documents"
)

// CleanupOrphanedFiles retries object removal independently of UI traffic.
// The scheduler calls it in each tenant transaction after the upload grace
// period has elapsed. It is the only recovery path for a process that died
// between the object write and the metadata commit, and for a folder that was
// deleted while some of its files still had bytes on disk.
//
// Each pass is capped at documents.CleanupBatchSize per list; a full batch is
// logged rather than silently read as "everything reclaimed".
func (rs *Resource) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	coordinator, err := rs.coordinator()
	if err != nil {
		return 0, fmt.Errorf("file storage unavailable: %w", err)
	}

	removed := 0
	var cleanupErr error

	files, err := rs.Service.ListDeletedFilesPendingCleanups(ctx)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list deleted files: %w", err))
	} else {
		for _, file := range files {
			if err := coordinator.Remove(ctx, file.TenantID, file.FilenameStored); err != nil {
				rs.getLogger().Warn("file cleanup failed",
					"folder_id", file.FolderID,
					"file_id", file.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			if err := rs.Service.MarkFileDeleted(ctx, file.ID); err != nil {
				rs.getLogger().Error("file cleanup status update failed",
					"folder_id", file.FolderID,
					"file_id", file.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			removed++
		}
		rs.logCleanupBatchFull(len(files), "deleted")
	}

	cleanups, err := rs.Service.ListQueuedFileCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued file cleanups: %w", err))
	}
	for _, cleanup := range cleanups {
		if err := coordinator.Remove(ctx, cleanup.TenantID, cleanup.FilenameStored); err != nil {
			rs.getLogger().Warn("file orphan cleanup failed",
				"folder_id", cleanup.OwnerID,
				"cleanup_id", cleanup.ID,
				"error", err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.Service.MarkQueuedCleanupComplete(ctx, cleanup.ID); err != nil {
			rs.getLogger().Error("file orphan cleanup status update failed",
				"folder_id", cleanup.OwnerID,
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

func (rs *Resource) logCleanupBatchFull(count int, source string) {
	if count < documentModels.CleanupBatchSize {
		return
	}
	rs.getLogger().Info("file cleanup batch full, more pending",
		"batch_size", documentModels.CleanupBatchSize,
		"source", source)
}
