package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
)

// CleanupOrphanedFiles retries object removal independently of UI traffic.
// The scheduler calls it in each tenant transaction after the upload grace
// period has elapsed. It is the only recovery path for a process that died
// between the object write and the metadata commit, and for a folder or
// announcement that was deleted while some of its documents still had bytes
// on disk.
//
// Each pass is capped at domain.CleanupBatchSize per list; a full batch is
// logged rather than silently read as "everything reclaimed". Per-object
// failures are joined and reported, never allowed to stop the pass.
func (s *Service) CleanupOrphanedFiles(ctx context.Context) (removed int, err error) {
	err = s.run("cleanup_orphaned_files", func(o *op) error {
		filesRemoved, filesErr := s.sweep(ctx, o, s.files(), "deleted", "orphan")
		attachmentsRemoved, attachmentsErr := s.sweep(ctx, o, s.attachments(), "attachment-deleted", "attachment-orphan")
		removed = filesRemoved + attachmentsRemoved
		return errors.Join(filesErr, attachmentsErr)
	})
	return removed, err
}

func (s *Service) sweep(ctx context.Context, o *op, k kind, deletedSource, orphanSource string) (int, error) {
	removed := 0
	var sweepErr error

	documents, stats, err := k.store.ListDeletedPendingCleanup(ctx)
	o.add(stats)
	if err != nil {
		sweepErr = errors.Join(sweepErr, fmt.Errorf("list deleted %s: %w", k.name, err))
	} else {
		for _, document := range documents {
			if err := k.objects.Remove(ctx, document.TenantID, document.FilenameStored); err != nil {
				s.deps.Logger.Warn("document cleanup failed",
					"kind", k.name,
					"owner_id", document.OwnerID,
					"document_id", document.ID,
					"error", err)
				sweepErr = errors.Join(sweepErr, err)
				continue
			}
			if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
				stats, err := k.store.MarkBytesDeleted(txCtx, document.ID)
				o.add(stats)
				return err
			}); err != nil {
				s.deps.Logger.Error("document cleanup status update failed",
					"kind", k.name,
					"owner_id", document.OwnerID,
					"document_id", document.ID,
					"error", err)
				sweepErr = errors.Join(sweepErr, err)
				continue
			}
			removed++
		}
		s.logBatchFull(len(documents), deletedSource)
	}

	intents, stats, err := k.cleanups.ListQueued(ctx)
	o.add(stats)
	if err != nil {
		return removed, errors.Join(sweepErr, fmt.Errorf("list queued %s cleanups: %w", k.name, err))
	}
	for _, intent := range intents {
		if err := k.objects.Remove(ctx, intent.TenantID, intent.FilenameStored); err != nil {
			s.deps.Logger.Warn("orphan cleanup failed",
				"kind", k.name,
				"owner_id", intent.OwnerID,
				"cleanup_id", intent.ID,
				"error", err)
			sweepErr = errors.Join(sweepErr, err)
			continue
		}
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			stats, err := k.cleanups.MarkComplete(txCtx, intent.ID)
			o.add(stats)
			return err
		}); err != nil {
			s.deps.Logger.Error("orphan cleanup status update failed",
				"kind", k.name,
				"owner_id", intent.OwnerID,
				"cleanup_id", intent.ID,
				"error", err)
			sweepErr = errors.Join(sweepErr, err)
			continue
		}
		removed++
	}
	s.logBatchFull(len(intents), orphanSource)
	return removed, sweepErr
}

func (s *Service) logBatchFull(count int, source string) {
	if count < domain.CleanupBatchSize {
		return
	}
	s.deps.Logger.Info("file cleanup batch full, more pending",
		"batch_size", domain.CleanupBatchSize,
		"source", source)
}
