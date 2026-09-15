package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// ListFiles returns the folder and its live files, newest first, each with
// the actor's delete right. Objects whose prior post-commit removal did not
// finish are retried after the caller's transaction commits.
func (s *Service) ListFiles(ctx context.Context, folderID int64, actor domain.Actor) (folder domain.Folder, files []domain.FileView, err error) {
	err = s.run("list_files", func(o *op) error {
		var err error
		if folder, err = s.requireVisible(ctx, o, folderID, actor); err != nil {
			return err
		}
		documents, stats, err := s.deps.Files.ListByOwner(ctx, folderID)
		o.add(stats)
		if err != nil {
			return err
		}
		files = make([]domain.FileView, 0, len(documents))
		for _, document := range documents {
			canDelete, err := s.canDelete(ctx, document, actor)
			if err != nil {
				return err
			}
			files = append(files, domain.FileView{Document: document, CanDelete: canDelete})
		}
		s.retryCleanups(ctx, o, folderID)
		return nil
	})
	return folder, files, err
}

// retryCleanups schedules removal of objects whose unlink did not finish, for
// one folder only and capped at domain.RequestCleanupRetryLimit. Queued upload
// intents are the sweep's job: an intent whose upload is still running must
// not be touched by a page view. A failed lookup is logged and the list is
// still answered: the retry is best effort, the sweep is the guarantee.
func (s *Service) retryCleanups(ctx context.Context, o *op, folderID int64) {
	pending, stats, err := s.deps.Files.ListDeletedPendingCleanupByOwner(ctx, folderID)
	o.add(stats)
	if err != nil {
		s.deps.Logger.Warn("file cleanup retry lookup failed",
			"folder_id", folderID,
			"error", err)
		return
	}
	for _, document := range pending {
		s.scheduleCleanup(ctx, s.files(), document, "retry")
	}
}

// AuthorizeUpload answers "may this caller add a file to this folder" WITHOUT
// writing anything, so an unauthorized request costs nothing but the request
// itself. UploadFile repeats the check inside its transaction.
func (s *Service) AuthorizeUpload(ctx context.Context, folderID int64, actor domain.Actor) error {
	return s.run("authorize_upload", func(o *op) error {
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.requireUpload(txCtx, o, folderID, actor)
		})
	})
}

func (s *Service) requireUpload(ctx context.Context, o *op, folderID int64, actor domain.Actor) error {
	if _, err := s.requireVisible(ctx, o, folderID, actor); err != nil {
		return err
	}
	allowed, err := s.canUpload(ctx, actor)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: uploads are reserved for the leadership", domain.ErrForbidden)
	}
	return nil
}

// UploadFile stores one upload: intent, object, metadata, settle.
func (s *Service) UploadFile(ctx context.Context, folderID int64, upload domain.Upload, actor domain.Actor) (file domain.Document, err error) {
	err = s.run("upload_file", func(o *op) error {
		tenantID := s.deps.Tx.TenantID(ctx)
		if tenantID <= 0 {
			return fmt.Errorf("%w: no tenant context", domain.ErrInvalid)
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		upload.FilenameDisplay = strings.TrimSpace(upload.FilenameDisplay)
		if upload.FilenameDisplay == "" {
			return fmt.Errorf("%w: filename is required", domain.ErrInvalid)
		}
		// Authorize BEFORE anything is written, so a request that was never
		// allowed costs neither disk nor a cleanup row.
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.requireUpload(txCtx, o, folderID, actor)
		}); err != nil {
			return err
		}
		storedName, err := s.deps.StoredNames(upload.Extension)
		if err != nil {
			return err
		}
		k := s.files()
		if err := s.queueIntent(ctx, k, folderID, storedName, s.deps.Now().Add(domain.CleanupDelay)); err != nil {
			s.deps.Logger.Error("file upload cleanup intent failed",
				"folder_id", folderID,
				"error", err)
			return err
		}
		// One deadline bounds the object write and the metadata transaction,
		// so an intent can never be settled after its sweep eligibility.
		uploadCtx, cancel := context.WithTimeout(ctx, domain.UploadDeadline)
		defer cancel()
		size, err := s.storeObject(ctx, uploadCtx, k, tenantID, folderID, storedName, upload)
		if err != nil {
			return err
		}
		file, err = s.createFile(uploadCtx, o, tenantID, folderID, storedName, size, upload, actor)
		if err != nil {
			// Only request-shaped rejections prove the row never landed;
			// anything deeper is left to the queued intent.
			rejectedBeforeCommit := errors.Is(err, domain.ErrInvalid) ||
				errors.Is(err, domain.ErrForbidden) ||
				errors.Is(err, domain.ErrQuotaExceeded)
			s.releaseFailedUpload(ctx, k, tenantID, folderID, storedName, rejectedBeforeCommit, err)
			return err
		}
		return nil
	})
	return file, err
}

func (s *Service) createFile(ctx context.Context, o *op, tenantID, folderID int64, storedName string, size int64, upload domain.Upload, actor domain.Actor) (file domain.Document, err error) {
	err = s.write(ctx, o, func(txCtx context.Context, o *op) error {
		if err := s.requireUpload(txCtx, o, folderID, actor); err != nil {
			return err
		}
		// The quota is a tenant-wide aggregate rather than a row we can lock.
		// Serialize its check and the metadata insert so concurrent uploads
		// cannot each admit themselves against the same previous total.
		if err := s.deps.Tx.AcquireLock(txCtx, fmt.Sprintf("filestore-quota:%d", tenantID)); err != nil {
			return fmt.Errorf("lock file storage quota: %w", err)
		}
		if err := s.requireQuota(txCtx, o, size); err != nil {
			return err
		}
		document := domain.NewDocument{
			OwnerID: folderID, Category: domain.FileCategory,
			FilenameDisplay: upload.FilenameDisplay, FilenameStored: storedName,
			SizeBytes: size, ContentType: upload.ContentType, UploadedBy: actor.AccountID,
		}
		if err := document.Validate(); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrInvalid, err.Error())
		}
		created, stats, err := s.deps.Files.Create(txCtx, document)
		o.add(stats)
		if err != nil {
			return err
		}
		stats, err = s.deps.FileCleanups.MarkCompleteByFilename(txCtx, storedName)
		o.add(stats)
		if err != nil {
			return fmt.Errorf("complete file upload cleanup intent: %w", err)
		}
		file = created
		return s.record(txCtx, actor, ports.EventFileUploaded, &folderID, nil, &created.ID,
			fmt.Sprintf("Datei „%s“ hochgeladen (%d Bytes)", created.FilenameDisplay, created.SizeBytes))
	})
	return file, err
}

// requireQuota refuses an upload that would push the school past its storage
// quota. Soft-deleted files still count until the sweep removed their bytes,
// which is what actually occupies the disk.
func (s *Service) requireQuota(ctx context.Context, o *op, sizeBytes int64) error {
	maxBytes, err := s.deps.Settings.MaxStorageBytes(ctx)
	if err != nil {
		return err
	}
	if maxBytes <= 0 {
		return nil
	}
	used, stats, err := s.deps.Files.TotalStoredBytes(ctx)
	o.add(stats)
	if err != nil {
		return err
	}
	if used+sizeBytes > maxBytes {
		return domain.ErrQuotaExceeded
	}
	return nil
}

// OpenFile resolves a file the actor may see and opens its bytes. The caller
// closes the object.
func (s *Service) OpenFile(ctx context.Context, folderID, fileID int64, actor domain.Actor) (file domain.Document, object ports.Object, err error) {
	err = s.run("open_file", func(o *op) error {
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if _, err := s.requireVisible(txCtx, o, folderID, actor); err != nil {
				return err
			}
			found, ok, stats, err := s.deps.Files.FindForOwner(txCtx, folderID, fileID, false)
			o.add(stats)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrNotFound
			}
			file = found
			return nil
		}); err != nil {
			return err
		}
		var err error
		object, err = s.openObject(ctx, s.files(), s.deps.Tx.TenantID(ctx), file)
		return err
	})
	return file, object, err
}

// DeleteFile soft-deletes a file with an audit row and removes its bytes after
// commit. A file that is already soft-deleted but still has bytes gets its
// removal retried instead of a 404 the caller cannot act on.
func (s *Service) DeleteFile(ctx context.Context, folderID, fileID int64, actor domain.Actor) error {
	return s.run("delete_file", func(o *op) error {
		if err := requireActor(actor); err != nil {
			return err
		}
		var document domain.Document
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if _, err := s.requireVisible(txCtx, o, folderID, actor); err != nil {
				return err
			}
			live, found, stats, err := s.deps.Files.FindForOwner(txCtx, folderID, fileID, false)
			o.add(stats)
			if err != nil {
				return err
			}
			if !found {
				deleted, found, stats, err := s.deps.Files.FindForOwner(txCtx, folderID, fileID, true)
				o.add(stats)
				if err != nil {
					return err
				}
				if !found {
					return domain.ErrNotFound
				}
				if err := s.requireDeleteRight(txCtx, deleted, actor); err != nil {
					return err
				}
				document = deleted
				return nil
			}
			if err := s.requireDeleteRight(txCtx, live, actor); err != nil {
				return err
			}
			deleted, stats, err := s.deps.Files.SoftDelete(txCtx, live.ID, actor.AccountID)
			o.add(stats)
			if err != nil {
				return err
			}
			document = deleted
			return s.record(txCtx, actor, ports.EventFileDeleted, &folderID, nil, &deleted.ID,
				fmt.Sprintf("Datei „%s“ gelöscht", deleted.FilenameDisplay))
		}); err != nil {
			return err
		}
		if document.BytesPresent() {
			s.scheduleCleanup(ctx, s.files(), document, "delete")
		}
		return nil
	})
}

func (s *Service) requireDeleteRight(ctx context.Context, document domain.Document, actor domain.Actor) error {
	allowed, err := s.canDelete(ctx, document, actor)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: only own uploads can be deleted", domain.ErrForbidden)
	}
	return nil
}
