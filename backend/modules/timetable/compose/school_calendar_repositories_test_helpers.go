//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// SchoolCalendarRepositories are the retained period, closing day and dateframe repositories.
type SchoolCalendarRepositories struct {
	CalendarPeriod ports.CalendarPeriodRecords
	ClosingDay     ports.ClosingDayRecords
	Dateframe      ports.DateframeRecords
}

// NewSchoolCalendarRepositories binds School Calendar; usage counts period references.
func NewSchoolCalendarRepositories(calendar schoolcalendar.Capability, usage ports.CalendarPeriodUsageRepository) SchoolCalendarRepositories {
	return SchoolCalendarRepositories{
		CalendarPeriod: application.NewCalendarPeriodRepository(calendar, usage),
		ClosingDay:     application.NewClosingDayRepository(calendar),
		Dateframe:      application.NewDateframeRepository(calendar),
	}
}

// NewClosingDayRepository serves the legacy closing day repository over the
// School Calendar owner (#2666).
func NewClosingDayRepository(calendar schoolcalendar.Capability) ports.ClosingDayRecords {
	return application.NewClosingDayRepository(calendar)
}

// NewDateframeRepository serves the legacy dateframe repository over the
// School Calendar owner (#2666).
func NewDateframeRepository(calendar schoolcalendar.Capability) ports.DateframeRecords {
	return application.NewDateframeRepository(calendar)
}
