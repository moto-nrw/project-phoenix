package application

import (
	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
)

func toCalendarDate(date appointmentcap.Date) appointmentcap.Date {
	return appointmentcap.Date(date.String())
}

func toTimezoneDate(date appointmentcap.Date) appointmentcap.Date {
	return appointmentcap.Date(date.String())
}

func toCalendarDates(dates []appointmentcap.Date) []appointmentcap.Date {
	converted := make([]appointmentcap.Date, len(dates))
	for i, date := range dates {
		converted[i] = toCalendarDate(date)
	}
	return converted
}

func toCalendarDatePtr(date *appointmentcap.Date) *appointmentcap.Date {
	if date == nil {
		return nil
	}
	converted := toCalendarDate(*date)
	return &converted
}
