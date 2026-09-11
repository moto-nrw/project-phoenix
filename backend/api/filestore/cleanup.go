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

	attachmentsRemoved, attachmentErr := rs.cleanupOrphanedAttachments(ctx)
	removed += attachmentsRemoved

	return removed, errors.Join(cleanupErr, attachmentErr)
}

// cleanupOrphanedAttachments is the same recovery for the attachments of
// Elternmitteilungen (#2890): uploads whose metadata never committed, and
// attachments whose announcement was deleted before their bytes were removed.
//
// It runs in the existing file-storage sweep rather than in a second scheduled
// task. The mechanism is identical — the same intents, the same batch cap, the
// same "log, never propagate" rule — only the tables and the storage prefix
// differ, so a separate task would only add a second thing to forget.
func (rs *Resource) cleanupOrphanedAttachments(ctx context.Context) (int, error) {
	coordinator, err := rs.attachmentCoordinator()
	if err != nil {
		return 0, fmt.Errorf("attachment storage unavailable: %w", err)
	}

	removed := 0
	var cleanupErr error

	attachments, err := rs.Service.ListDeletedAttachmentsPendingCleanups(ctx)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list deleted attachments: %w", err))
	} else {
		for _, attachment := range attachments {
			if err := coordinator.Remove(ctx, attachment.TenantID, attachment.FilenameStored); err != nil {
				rs.getLogger().Warn("announcement attachment cleanup failed",
					"announcement_id", attachment.AnnouncementID,
					"attachment_id", attachment.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			if err := rs.Service.MarkAttachmentFileDeleted(ctx, attachment.ID); err != nil {
				rs.getLogger().Error("announcement attachment cleanup status update failed",
					"announcement_id", attachment.AnnouncementID,
					"attachment_id", attachment.ID,
					"error", err)
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			removed++
		}
		rs.logCleanupBatchFull(len(attachments), "attachment-deleted")
	}

	intents, err := rs.Service.ListQueuedAttachmentCleanups(ctx)
	if err != nil {
		return removed, errors.Join(cleanupErr, fmt.Errorf("list queued attachment cleanups: %w", err))
	}
	for _, intent := range intents {
		if err := coordinator.Remove(ctx, intent.TenantID, intent.FilenameStored); err != nil {
			rs.getLogger().Warn("announcement attachment orphan cleanup failed",
				"announcement_id", intent.OwnerID,
				"cleanup_id", intent.ID,
				"error", err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := rs.Service.MarkQueuedAttachmentCleanupComplete(ctx, intent.ID); err != nil {
			rs.getLogger().Error("announcement attachment orphan cleanup status update failed",
				"announcement_id", intent.OwnerID,
				"cleanup_id", intent.ID,
				"error", err)
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}
	rs.logCleanupBatchFull(len(intents), "attachment-orphan")

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
