package api

import (
	"context"
	"log/slog"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	fileCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
)

// NewEnrollmentUploadStorage opens the private store of the files parents
// attach to an enrollment (#4783) through File Storage's document
// coordinator. The backend is looked up on each call, like the child
// documents, so constructing the routes opens no storage.
func NewEnrollmentUploadStorage(logger *slog.Logger) enrollmentAPI.UploadFileStorage {
	return func(kind string) (enrollmentAPI.UploadFileStore, error) {
		backend, err := nativeUploads.PrivateUploadsBackend()
		if err != nil {
			return nil, err
		}
		return fileCompose.NewDocumentCoordinator(kind, backend, enrollmentUploadObjectStore{}, logger), nil
	}
}

// enrollmentUploadObjectStore settles nothing: an upload's cleanup intent is
// queued by the database when its row is deleted and settled by the
// Worker's sweep, not by the coordinator.
type enrollmentUploadObjectStore struct{}

func (enrollmentUploadObjectStore) MarkFileDeleted(context.Context, int64) error { return nil }
func (enrollmentUploadObjectStore) MarkQueuedCleanupCompleteByFilename(context.Context, string) error {
	return nil
}
func (enrollmentUploadObjectStore) ActivateQueuedCleanup(context.Context, string) error { return nil }
