package planning

import (
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func workforceDate(date calendar.Date) configModel.CalendarDate {
	if date.IsZero() {
		return ""
	}
	return configModel.NewCalendarDate(date.Year(), date.Month(), date.Day())
}

func workforceDatePointer(date *calendar.Date) *configModel.CalendarDate {
	if date == nil {
		return nil
	}
	converted := workforceDate(*date)
	return &converted
}

func calendarDate(date configModel.CalendarDate) calendar.Date {
	if date.IsZero() {
		return calendar.Date("")
	}
	value := date.UTCMidnight()
	return calendar.NewDate(value.Year(), value.Month(), value.Day())
}

func workforceDates(dates []calendar.Date) []configModel.CalendarDate {
	converted := make([]configModel.CalendarDate, len(dates))
	for index, date := range dates {
		converted[index] = workforceDate(date)
	}
	return converted
}
