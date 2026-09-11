package appointments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOccurrenceStartInstant(t *testing.T) {
	t.Parallel()

	t.Run("a timed appointment starts at its wall clock, in Berlin", func(t *testing.T) {
		appointment := recurrenceFixture()
		start := OccurrenceStartInstant(appointment)

		assert.Equal(t, 2026, start.Year())
		assert.Equal(t, time.January, start.Month())
		assert.Equal(t, 5, start.Day())
		assert.Equal(t, 9, start.Hour())
		assert.Equal(t, 0, start.Minute())
	})

	t.Run("an all-day appointment is anchored in the morning, not at midnight", func(t *testing.T) {
		appointment := recurrenceFixture()
		appointment.AllDay = true

		start := OccurrenceStartInstant(appointment)

		assert.Equal(t, 8, start.Hour(),
			"midnight plus a 24h lead would mail parents the night before last")
		assert.Equal(t, 5, start.Day())
	})

	t.Run("an all-day appointment stays at 08:00 across DST changes", func(t *testing.T) {
		for _, date := range []Date{
			NewDate(2026, 3, 29),
			NewDate(2026, 10, 25),
		} {
			appointment := recurrenceFixture()
			appointment.StartDate = date
			appointment.EndDate = date
			appointment.AllDay = true

			assert.Equal(t, 8, OccurrenceStartInstant(appointment).Hour(), date.String())
		}
	})

	t.Run("summer time does not shift the clock", func(t *testing.T) {
		// 2026-07-01 is CEST (UTC+2), 2026-01-05 is CET (UTC+1). Both must report
		// the appointment's own wall clock: deriving the instant by adding a fixed
		// UTC offset would make one of them an hour off.
		appointment := recurrenceFixture()
		appointment.StartDate = NewDate(2026, 7, 1)
		appointment.EndDate = NewDate(2026, 7, 1)

		start := OccurrenceStartInstant(appointment)

		assert.Equal(t, 9, start.Hour())
		_, offset := start.Zone()
		assert.Equal(t, 2*60*60, offset, "July in Berlin is UTC+2")
	})
}

func TestAppointmentWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("moves the appointment onto the occurrence and keeps its span", func(t *testing.T) {
		appointment := recurrenceFixture()
		appointment.EndDate = NewDate(2026, 1, 6) // a two-day appointment

		effective := EffectiveOccurrence(appointment, NewDate(2026, 3, 9), nil)

		assert.Equal(t, NewDate(2026, 3, 9), effective.StartDate)
		assert.Equal(t, NewDate(2026, 3, 10), effective.EndDate,
			"the second day has to travel with the occurrence")
		assert.Equal(t, appointment.StartDate, NewDate(2026, 1, 5),
			"the stored appointment must not be mutated")
	})

	t.Run("a moved occurrence is reminded about at its new time", func(t *testing.T) {
		appointment := recurrenceFixture()
		movedStart := time.Date(2026, 1, 1, 15, 30, 0, 0, time.UTC)
		override := &AppointmentOccurrenceOverride{
			AppointmentID:  appointment.ID,
			OccurrenceDate: NewDate(2026, 1, 5),
			StartTime:      &movedStart,
		}

		effective := EffectiveOccurrence(appointment, NewDate(2026, 1, 5), override)

		assert.Equal(t, 15, OccurrenceStartInstant(effective).Hour())
		assert.Equal(t, 30, OccurrenceStartInstant(effective).Minute())
	})
}

// A single-occurrence override is what a parent actually sees in the reminder
// mail: whatever the organizer changed for THIS date has to reach the copy, or
// the mail describes an appointment that no longer happens that way.
func TestAppointmentWithOverrideAppliesEveryChangedField(t *testing.T) {
	t.Parallel()

	appointment := recurrenceFixture()
	occurrence := NewDate(2026, 3, 9)

	title := "Elterngespräch (verlegt)"
	location := "Raum 2"
	allDay := true
	movedStart := time.Date(2026, 1, 1, 15, 30, 0, 0, time.UTC)
	movedEnd := time.Date(2026, 1, 1, 16, 30, 0, 0, time.UTC)
	movedDate := NewDate(2026, 3, 10)
	movedEndDate := NewDate(2026, 3, 11)

	effective := EffectiveOccurrence(appointment, occurrence, &AppointmentOccurrenceOverride{
		AppointmentID:  appointment.ID,
		OccurrenceDate: occurrence,
		Title:          &title,
		Location:       &location,
		AllDay:         &allDay,
		StartTime:      &movedStart,
		EndTime:        &movedEnd,
		StartDate:      &movedDate,
		EndDate:        &movedEndDate,
	})

	assert.Equal(t, title, effective.Title)
	require.NotNil(t, effective.Location)
	assert.Equal(t, location, *effective.Location)
	assert.True(t, effective.AllDay)
	assert.Equal(t, movedStart, effective.StartTime)
	assert.Equal(t, movedEnd, effective.EndTime)
	assert.Equal(t, movedDate, effective.StartDate, "an explicit override date wins over the occurrence date")
	assert.Equal(t, movedEndDate, effective.EndDate)

	assert.Equal(t, "Planning", appointment.Title, "the stored appointment must not be mutated")
	assert.False(t, appointment.AllDay)
}
