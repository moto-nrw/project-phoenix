package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

// Directory is the composed kiosk roster contract.
type Directory = devicescan.Directory

// NewDirectory binds the kiosk roster to the retained people adapter. It
// accepts the existing service, never constructs a legacy factory graph.
func NewDirectory(persons PersonDirectory, activities devicescan.ActivityDirectory, logger *slog.Logger, principal devicescan.DevicePrincipal) devicescan.Directory {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	if persons == nil {
		panic("kiosk directory composition: users are required")
	}
	var catalog application.DirectoryActivities
	if activities != nil {
		catalog = activities
	}
	return application.NewDirectory(directoryPeople{persons}, catalog, principal, logger)
}

type directoryPeople struct{ PersonDirectory }

func (d directoryPeople) Teachers(ctx context.Context) ([]application.DirectoryTeacher, error) {
	return application.TeachersBinding(ctx, d.PersonDirectory)
}
