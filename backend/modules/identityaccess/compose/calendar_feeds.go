package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
)

type calendarFeeds struct {
	staffCalendarFeeds  *application.StaffCalendarFeeds
	parentCalendarFeeds *application.ParentCalendarFeeds
}

func newCalendarFeeds(service *application.Service, store *postgres.Store) calendarFeeds {
	return calendarFeeds{
		staffCalendarFeeds:  application.NewStaffCalendarFeeds(service, store),
		parentCalendarFeeds: application.NewParentCalendarFeeds(service, store),
	}
}
