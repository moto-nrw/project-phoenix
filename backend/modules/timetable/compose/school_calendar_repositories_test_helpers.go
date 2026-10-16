package compose

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// SchoolCalendarRepositories are the retained calendar period, closing day
// and dateframe repositories served by the School Calendar owner.
type SchoolCalendarRepositories struct {
	CalendarPeriod scheduleModels.CalendarPeriodRepository
	ClosingDay     scheduleModels.ClosingDayRepository
	Dateframe      scheduleModels.DateframeRepository
}

// NewSchoolCalendarRepositories builds the retained calendar repositories
// over the School Calendar owner; usage answers the period reference counts.
func NewSchoolCalendarRepositories(calendar schoolcalendar.Capability, usage ports.CalendarPeriodUsageRepository) SchoolCalendarRepositories {
	return SchoolCalendarRepositories{
		CalendarPeriod: application.NewCalendarPeriodRepository(calendar, usage),
		ClosingDay:     application.NewClosingDayRepository(calendar),
		Dateframe:      application.NewDateframeRepository(calendar),
	}
}

// NewClosingDayRepository serves the legacy closing day repository over the
// School Calendar owner (#2666).
func NewClosingDayRepository(calendar schoolcalendar.Capability) scheduleModels.ClosingDayRepository {
	return application.NewClosingDayRepository(calendar)
}

// NewDateframeRepository serves the legacy dateframe repository over the
// School Calendar owner (#2666).
func NewDateframeRepository(calendar schoolcalendar.Capability) scheduleModels.DateframeRepository {
	return application.NewDateframeRepository(calendar)
}
