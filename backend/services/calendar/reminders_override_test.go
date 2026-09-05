package calendar

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single-occurrence override is what a parent actually sees in the reminder
// mail: whatever the organizer changed for THIS date has to reach the copy, or
// the mail describes an appointment that no longer happens that way.
func TestAppointmentWithOverrideAppliesEveryChangedField(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()
	occurrence := timezone.NewDate(2026, 3, 9)

	title := "Elterngespräch (verlegt)"
	location := "Raum 2"
	allDay := true
	movedStart := helperClock(15, 30)
	movedEnd := helperClock(16, 30)
	movedDate := calModels.NewDate(2026, 3, 10)
	movedEndDate := calModels.NewDate(2026, 3, 11)

	effective := appointmentWithOverride(appointment, occurrence, &calModels.AppointmentOccurrenceOverride{
		AppointmentID:  appointment.ID,
		OccurrenceDate: calModels.Date(occurrence),
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
