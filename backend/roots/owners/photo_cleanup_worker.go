package owners

import (
	"context"
	"errors"
	"log/slog"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	fileStorage "github.com/moto-nrw/project-phoenix/modules/filestorage"
	fileStorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// workerFileCleanup adds managed-upload namespaces to the existing bounded,
// tenant-scoped cleanup tick, including inactive schools.
type workerFileCleanup func(context.Context) (int, error)

func newFileStoreCleaner(db transactionruntime.Database, files fileStorage.Capability, logger *slog.Logger) workerFileCleanup {
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
	uploads := NewEnrollmentRequestUploads(enrollmentCompose.New(), nil, nil, logger.With("service", "enrollment-upload-cleanup"))
	removeUpload := func(ctx context.Context, tenantID int64, storedName string) error {
		backend, err := nativeUploads.PrivateUploadsBackend()
		if err != nil {
			return err
		}
		coordinator := fileStorageCompose.NewDocumentCoordinator(enrollmentModule.RequestUploadStorageKind, backend, enrollmentUploadObjectStore{}, logger)
		return coordinator.Remove(ctx, tenantID, storedName)
	}
	return workerFileCleanup(func(ctx context.Context) (int, error) {
		var removed int
		var fileErr error
		if files != nil {
			removed, fileErr = files.CleanupOrphanedFiles(ctx)
		}
		photosRemoved, photoErr := photos.Sweep(ctx, remover.RemoveStored)
		legalRemoved, legalErr := legal.Sweep(ctx, removeLegal)
		uploadsRemoved, uploadErr := uploads.SweepUploadObjects(ctx, removeUpload)
		return removed + photosRemoved + legalRemoved + uploadsRemoved, errors.Join(fileErr, photoErr, legalErr, uploadErr)
	})
}

func (cleanup workerFileCleanup) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	return cleanup(ctx)
}

// enrollmentUploadObjectStore settles nothing: an upload's cleanup intent is
// queued by the database when its row is deleted and settled by the sweep,
// not by the coordinator (#4783).
type enrollmentUploadObjectStore struct{}

func (enrollmentUploadObjectStore) MarkFileDeleted(context.Context, int64) error { return nil }
func (enrollmentUploadObjectStore) MarkQueuedCleanupCompleteByFilename(context.Context, string) error {
	return nil
}
func (enrollmentUploadObjectStore) ActivateQueuedCleanup(context.Context, string) error { return nil }
