package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

// Directory is the composed kiosk roster contract.
type Directory = devicescan.Directory

// NewDirectory binds the kiosk roster to the retained people adapter and the
// holiday guest hints (PyrePortal#499); it never builds a legacy factory graph.
func NewDirectory(persons PersonDirectory, activities devicescan.ActivityDirectory, guests devicescan.HolidayGuestSchools, logger *slog.Logger, principal devicescan.DevicePrincipal) devicescan.Directory {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	if persons == nil || guests == nil {
		panic("kiosk directory composition: users and holiday guests are required")
	}
	var catalog application.DirectoryActivities
	if activities != nil {
		catalog = activities
	}
	return application.NewHolidayGuestDirectory(directoryPeople{persons}, catalog, guests, principal, logger)
}

type directoryPeople struct{ PersonDirectory }

func (d directoryPeople) Teachers(ctx context.Context) ([]application.DirectoryTeacher, error) {
	return application.TeachersBinding(ctx, d.PersonDirectory)
}
