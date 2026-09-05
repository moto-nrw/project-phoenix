package calendar

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/stretchr/testify/assert"
)

func TestOccurrenceStartInstant(t *testing.T) {
	t.Parallel()

	t.Run("a timed appointment starts at its wall clock, in Berlin", func(t *testing.T) {
		appointment := helperAppointment()
		start := occurrenceStartInstant(appointment)

		assert.Equal(t, 2026, start.Year())
		assert.Equal(t, time.January, start.Month())
		assert.Equal(t, 5, start.Day())
		assert.Equal(t, 9, start.Hour())
		assert.Equal(t, 0, start.Minute())
	})

	t.Run("an all-day appointment is anchored in the morning, not at midnight", func(t *testing.T) {
		appointment := helperAppointment()
		appointment.AllDay = true

		start := occurrenceStartInstant(appointment)

		assert.Equal(t, 8, start.Hour(),
			"midnight plus a 24h lead would mail parents the night before last")
		assert.Equal(t, 5, start.Day())
	})

	t.Run("an all-day appointment stays at 08:00 across DST changes", func(t *testing.T) {
		for _, date := range []timezone.Date{
			timezone.NewDate(2026, 3, 29),
			timezone.NewDate(2026, 10, 25),
		} {
			appointment := helperAppointment()
			appointment.StartDate = calModels.Date(date)
			appointment.EndDate = calModels.Date(date)
			appointment.AllDay = true

			assert.Equal(t, 8, occurrenceStartInstant(appointment).Hour(), date.String())
		}
	})

	t.Run("summer time does not shift the clock", func(t *testing.T) {
		// 2026-07-01 is CEST (UTC+2), 2026-01-05 is CET (UTC+1). Both must report
		// the appointment's own wall clock: deriving the instant by adding a fixed
		// UTC offset would make one of them an hour off.
		appointment := helperAppointment()
		appointment.StartDate = calModels.NewDate(2026, 7, 1)
		appointment.EndDate = calModels.NewDate(2026, 7, 1)

		start := occurrenceStartInstant(appointment)

		assert.Equal(t, 9, start.Hour())
		_, offset := start.Zone()
		assert.Equal(t, 2*60*60, offset, "July in Berlin is UTC+2")
	})
}

func TestAppointmentWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("moves the appointment onto the occurrence and keeps its span", func(t *testing.T) {
		appointment := helperAppointment()
		appointment.EndDate = calModels.NewDate(2026, 1, 6) // a two-day appointment

		effective := appointmentWithOverride(appointment, timezone.NewDate(2026, 3, 9), nil)

		assert.Equal(t, calModels.NewDate(2026, 3, 9), effective.StartDate)
		assert.Equal(t, calModels.NewDate(2026, 3, 10), effective.EndDate,
			"the second day has to travel with the occurrence")
		assert.Equal(t, appointment.StartDate, calModels.NewDate(2026, 1, 5),
			"the stored appointment must not be mutated")
	})

	t.Run("a moved occurrence is reminded about at its new time", func(t *testing.T) {
		appointment := helperAppointment()
		movedStart := helperClock(15, 30)
		override := &calModels.AppointmentOccurrenceOverride{
			AppointmentID:  appointment.ID,
			OccurrenceDate: calModels.NewDate(2026, 1, 5),
			StartTime:      &movedStart,
		}

		effective := appointmentWithOverride(appointment, timezone.NewDate(2026, 1, 5), override)

		assert.Equal(t, 15, occurrenceStartInstant(effective).Hour())
		assert.Equal(t, 30, occurrenceStartInstant(effective).Minute())
	})
}

func TestLocalizedAppointmentNotificationCopyCarriesNoAppointmentTitle(t *testing.T) {
	t.Parallel()

	// The push payload reaches a lock screen. Appointment titles are free text
	// and can contain a family name.
	for _, kind := range []string{
		"appointment_published",
		"appointment_updated",
		"appointment_cancelled",
		"appointment_reminder",
	} {
		title, body := localizedAppointmentNotificationCopy(kind, "")
		assert.NotEmpty(t, title)
		assert.NotEmpty(t, body)
		assert.NotContains(t, body, "Planning", "the appointment title must not reach the payload")
	}
}
