package api

import (
	"context"
	"errors"
	"log/slog"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	fileStorage "github.com/moto-nrw/project-phoenix/modules/filestorage"
	fileStorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// workerFileCleanup adds managed-upload namespaces to the existing bounded,
// tenant-scoped cleanup tick, including inactive schools.
type workerFileCleanup func(context.Context) (int, error)

func newFileStoreCleaner(db transactionruntime.Database, files *fileStorage.Module, logger *slog.Logger) workerFileCleanup {
	remover := peopledirectory.NewPhotoUnlinker(logger, "public", nativeUploads.UploadFiles{})
	photos := fileStorageCompose.NewUploadCleanup(db.Pool(), fileStorage.CleanupStudentPhotos)
	legal := fileStorageCompose.NewUploadCleanup(db.Pool(), fileStorage.CleanupEnrollmentLegalDocuments)
	filesBackend := nativeUploads.UploadFiles{}
	removeLegal := func(ctx context.Context, url string) error {
		path, err := filesBackend.StoredPath("public", url, "/uploads/enrollment-legal-documents/")
		if err != nil {
			return err
		}
		return filesBackend.RemoveUploadContext(ctx, path)
	}
	return workerFileCleanup(func(ctx context.Context) (int, error) {
		var removed int
		var fileErr error
		if files != nil {
			removed, fileErr = files.CleanupOrphanedFiles(ctx)
		}
		photosRemoved, photoErr := photos.Sweep(ctx, remover.RemoveStored)
		legalRemoved, legalErr := legal.Sweep(ctx, removeLegal)
		return removed + photosRemoved + legalRemoved, errors.Join(fileErr, photoErr, legalErr)
	})
}

func (cleanup workerFileCleanup) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	return cleanup(ctx)
}
