package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/application"
)

func NewCalendarRenderer(renderer schoolcalendar.CalendarRenderer) portal.CalendarRenderer {
	return application.NewCalendarRenderer(renderer)
}
