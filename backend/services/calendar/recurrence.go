package calendar

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
)

// Calendar views adapt their legacy transport shape to the owner's recurrence engine.
func recurrenceAppointment(value *calModels.Appointment) *appointments.Appointment {
	return &appointments.Appointment{StartDate: value.StartDate, EndDate: value.EndDate}
}

func ownerAppointment(value *calModels.Appointment) *appointments.Appointment {
	if value == nil {
		return nil
	}
	return &appointments.Appointment{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		CancelledAt: value.CancelledAt, DeletedAt: value.DeletedAt,
		NotifyGuardians: value.NotifyGuardians, Revision: value.Revision,
	}
}

func expandOccurrences(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	values := appointments.ExpandOccurrences(recurrenceAppointment(appointment), rule, toCalendarDate(from), toCalendarDate(to))
	result := make([]timezone.Date, 0, len(values))
	for _, value := range values {
		result = append(result, toTimezoneDate(value))
	}
	return result
}

func occurrenceExists(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, date timezone.Date) bool {
	return appointments.OccurrenceExists(recurrenceAppointment(appointment), rule, toCalendarDate(date))
}

func hasOccurrenceInWindow(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) bool {
	return appointments.HasOccurrenceInWindow(recurrenceAppointment(appointment), rule, toCalendarDate(from), toCalendarDate(to))
}

func firstRecurrenceOccurrence(appointment *calModels.Appointment, rule *calModels.RecurrenceRule) (timezone.Date, bool) {
	date, ok := appointments.FirstRecurrenceOccurrence(recurrenceAppointment(appointment), rule)
	return toTimezoneDate(date), ok
}

func appointmentWithOverride(appointment *calModels.Appointment, occurrence timezone.Date, override *calModels.AppointmentOccurrenceOverride) *calModels.Appointment {
	return calendarAppointment(appointments.EffectiveOccurrence(ownerAppointment(appointment), toCalendarDate(occurrence), override))
}

func occurrenceStartInstant(appointment *calModels.Appointment) time.Time {
	return appointments.OccurrenceStartInstant(ownerAppointment(appointment))
}
