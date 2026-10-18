package api

import (
	"log/slog"

	careplanOwnerHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"

	fileCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
)

// NewStudentDocumentStorage keeps backend lookup lazy, so constructing a
// resource neither opens storage nor fixes a process-global backend snapshot.
func NewStudentDocumentStorage() careplanOwnerHTTP.StudentDocumentStorage {
	return func(kind string, store careplanOwnerHTTP.StudentDocumentFileStore, logger *slog.Logger) (careplanOwnerHTTP.StudentDocumentFiles, error) {
		backend, err := nativeUploads.PrivateUploadsBackend()
		if err != nil {
			return nil, err
		}
		return fileCompose.NewDocumentCoordinator(kind, backend, store, logger), nil
	}
}
