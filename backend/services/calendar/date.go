package calendar

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
)

func toCalendarDate(date timezone.Date) calModels.Date {
	return calModels.Date(date.String())
}

func toTimezoneDate(date calModels.Date) timezone.Date {
	return timezone.Date(date.String())
}

func toCalendarDates(dates []timezone.Date) []calModels.Date {
	converted := make([]calModels.Date, len(dates))
	for i, date := range dates {
		converted[i] = toCalendarDate(date)
	}
	return converted
}

func toCalendarDatePtr(date *timezone.Date) *calModels.Date {
	if date == nil {
		return nil
	}
	converted := toCalendarDate(*date)
	return &converted
}
