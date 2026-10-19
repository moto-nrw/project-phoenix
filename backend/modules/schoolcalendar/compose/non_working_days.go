package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
)

func NewNonWorkingDays(source schoolcalendar.NonWorkingDayQuery) application.NonWorkingDays {
	return application.NewNonWorkingDays(source)
}
func NewTenantHolidays(source schoolcalendar.NonWorkingDayQuery) application.TenantHolidays {
	return application.NewTenantHolidays(source)
}
func NewTenantClosingDays(source schoolcalendar.NonWorkingDayQuery) application.TenantClosingDays {
	return application.NewTenantClosingDays(source)
}
func NewCalendarPeriodIDs(source schoolcalendar.Query) application.CalendarPeriodIDs {
	return application.NewCalendarPeriodIDs(source)
}
