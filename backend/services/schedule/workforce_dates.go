package schedule

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

func workforceDate(date timezone.Date) configModel.CalendarDate {
	if date.IsZero() {
		return ""
	}
	return configModel.NewCalendarDate(date.Year(), date.Month(), date.Day())
}

func workforceDatePointer(date *timezone.Date) *configModel.CalendarDate {
	if date == nil {
		return nil
	}
	converted := workforceDate(*date)
	return &converted
}

func calendarDate(date configModel.CalendarDate) timezone.Date {
	if date.IsZero() {
		return timezone.Date("")
	}
	value := date.UTCMidnight()
	return timezone.NewDate(value.Year(), value.Month(), value.Day())
}

func workforceDates(dates []timezone.Date) []configModel.CalendarDate {
	converted := make([]configModel.CalendarDate, len(dates))
	for index, date := range dates {
		converted[index] = workforceDate(date)
	}
	return converted
}
