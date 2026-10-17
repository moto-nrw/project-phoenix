package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
)

func newCalendarFeeds(service *application.Service, store *postgres.Store) application.CalendarFeeds {
	return application.NewCalendarFeeds(application.NewStaffCalendarFeeds(service, store), application.NewParentCalendarFeeds(service, store))
}
