package active

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

func workforceDate(date timezone.Date) configModels.CalendarDate {
	if date.IsZero() {
		return ""
	}
	return configModels.NewCalendarDate(date.Year, date.Month, date.Day)
}

func workforceDatePointer(date *timezone.Date) *configModels.CalendarDate {
	if date == nil {
		return nil
	}
	converted := workforceDate(*date)
	return &converted
}

func calendarDate(date configModels.CalendarDate) timezone.Date {
	if date.IsZero() {
		return timezone.Date{}
	}
	value := date.UTCMidnight()
	return timezone.NewDate(value.Year(), value.Month(), value.Day())
}

func workforceDates(dates []timezone.Date) []configModels.CalendarDate {
	converted := make([]configModels.CalendarDate, len(dates))
	for index, date := range dates {
		converted[index] = workforceDate(date)
	}
	return converted
}
