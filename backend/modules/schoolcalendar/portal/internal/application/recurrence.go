package application

import (
	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
)

// Calendar views adapt their legacy transport shape to the owner's recurrence engine.
func recurrenceAppointment(value *appointmentcap.Appointment) *appointmentcap.Appointment {
	return &appointmentcap.Appointment{StartDate: value.StartDate, EndDate: value.EndDate}
}

func expandOccurrences(appointment *appointmentcap.Appointment, rule *appointmentcap.RecurrenceRule, from, to appointmentcap.Date) []appointmentcap.Date {
	values := appointmentcap.ExpandOccurrences(recurrenceAppointment(appointment), rule, toCalendarDate(from), toCalendarDate(to))
	result := make([]appointmentcap.Date, 0, len(values))
	for _, value := range values {
		result = append(result, toTimezoneDate(value))
	}
	return result
}

func occurrenceExists(appointment *appointmentcap.Appointment, rule *appointmentcap.RecurrenceRule, date appointmentcap.Date) bool {
	return appointmentcap.OccurrenceExists(recurrenceAppointment(appointment), rule, toCalendarDate(date))
}

func hasOccurrenceInWindow(appointment *appointmentcap.Appointment, rule *appointmentcap.RecurrenceRule, from, to appointmentcap.Date) bool {
	return appointmentcap.HasOccurrenceInWindow(recurrenceAppointment(appointment), rule, toCalendarDate(from), toCalendarDate(to))
}

func firstRecurrenceOccurrence(appointment *appointmentcap.Appointment, rule *appointmentcap.RecurrenceRule) (appointmentcap.Date, bool) {
	date, ok := appointmentcap.FirstRecurrenceOccurrence(recurrenceAppointment(appointment), rule)
	return toTimezoneDate(date), ok
}
