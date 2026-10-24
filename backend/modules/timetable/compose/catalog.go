package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// NewCatalog exposes native values over the same activity service and occupancy query.
func NewCatalog(service activityContract.ActivityService) timetable.Catalog {
	if service == nil {
		return nil
	}
	return application.Catalog{Source: service, OccupancyQuery: service}
}
