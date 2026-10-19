package appointments

import (
	"context"
	"errors"
)

// AppointmentRetention deletes a school's ended appointments once its
// timetable window has passed after their last day (#4045). Recipients,
// targets, recurrence rules, occurrence overrides and reminder claims go
// with their appointment. Cancelled and deleted appointments go earlier,
// TombstoneRetentionDays after the cancellation (DeleteFeedTombstonesBefore).
type AppointmentRetention interface {
	DeleteExpiredAppointments(ctx context.Context, retentionDays int) (int, error)
}

// DeleteExpiredAppointments runs the retention the module was composed with.
func (m *Module) DeleteExpiredAppointments(ctx context.Context, retentionDays int) (int, error) {
	if m.retention == nil {
		return 0, errors.New("appointments: the retention of ended appointments is not composed")
	}
	return m.retention.DeleteExpiredAppointments(ctx, retentionDays)
}

// ErrAppointmentRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrAppointmentRetentionWindow = errors.New("appointment retention window must be positive")

// TombstoneRetentionDays is the fixed window of a cancelled or deleted
// appointment: every subscribed calendar feed has seen the cancellation for
// 90 days before the row goes.
const TombstoneRetentionDays = 90

// RetentionCandidate is an appointment that starts before a retention cutoff,
// with its recurrence rule, if any, and the last day an occurrence override
// moves one of its occurrences to.
type RetentionCandidate struct {
	Appointment    *Appointment
	Rule           *RecurrenceRule
	LatestOverride Date
}

// LastDay returns the last calendar day the appointment occupies: the end of
// a single appointment, the end of the last occurrence of a series bounded by
// a date or a count, or a later day an override moved an occurrence to. ok
// is false for an endless series, which never ends.
func (c RetentionCandidate) LastDay() (Date, bool) {
	appointment := c.Appointment
	last := appointment.EndDate
	if c.Rule != nil {
		span := appointment.StartDate.DaysUntil(appointment.EndDate)
		switch {
		case c.Rule.EndsOn != nil:
			last = c.Rule.EndsOn.AddDays(span)
		case c.Rule.OccurrenceCount != nil:
			if dates := boundedRecurrenceDates(appointment, c.Rule, *c.Rule.OccurrenceCount); len(dates) > 0 {
				last = dates[len(dates)-1].AddDays(span)
			}
		default:
			return "", false
		}
		if last.Before(appointment.EndDate) {
			last = appointment.EndDate
		}
	}
	if c.LatestOverride.After(last) {
		last = c.LatestOverride
	}
	return last, true
}
