package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/uptrace/bun"
)

// NewStudentPhotoCareEnd composes the nightly end of the photos of children
// whose care is over (#6650) over the directory rows and the photo runtime
// the root binds: the durable file cleanup queue, the after-commit unlink and
// the live refresh. Without the runtime no photo could be removed, so the
// composition refuses instead of clearing rows whose files would stay.
func NewStudentPhotoCareEnd(db *bun.DB, students StudentDirectoryRows, photos StudentPhotoRuntime, observe func(Observation), logger *slog.Logger) (peopledirectory.StudentPhotoCareEndRetention, error) {
	if db == nil || students == nil || photos == nil || observe == nil || logger == nil {
		return nil, errors.New("people directory compose: the photo care end needs the database, the student rows, the photo runtime, an observer and a logger")
	}
	database := postgres.NewDatabase()
	owners := application.NewStudentOwners(nil)
	store := postgres.NewStudentStore(database, students, owners.LockClassWrites, nil)
	lifecycle := application.NewStudentPhotos(store, application.StudentPhotoPort(photos), transaction{}, application.NewObserver(observe), time.Now)
	return application.NewStudentPhotoCareEnd(lifecycle, logger), nil
}
