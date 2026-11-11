package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

// StudentPhotoCareEnd is the nightly end of the photos of children whose
// care is over (#6650), over this directory's photo lifecycle: its rows and
// the photo runtime the root bound (durable file cleanup queue, after-commit
// unlink, live refresh). Without a bound runtime the run refuses instead of
// clearing rows whose files would stay.
func (d *Directory) StudentPhotoCareEnd(logger *slog.Logger) peopledirectory.StudentPhotoCareEndRetention {
	if logger == nil {
		logger = slog.Default()
	}
	return application.NewStudentPhotoCareEnd(d.photos, logger)
}
