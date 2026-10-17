package api

import (
	"log/slog"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	fileCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	studentsHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
)

// NewStudentDocumentStorage keeps backend lookup lazy, so constructing a
// resource neither opens storage nor fixes a process-global backend snapshot.
func NewStudentDocumentStorage() studentsHTTP.StudentDocumentStorage {
	return func(kind string, store studentsHTTP.StudentDocumentFileStore, logger *slog.Logger) (studentsHTTP.StudentDocumentFiles, error) {
		backend, err := nativeUploads.PrivateUploadsBackend()
		if err != nil {
			return nil, err
		}
		return fileCompose.NewDocumentCoordinator(kind, backend, store, logger), nil
	}
}
