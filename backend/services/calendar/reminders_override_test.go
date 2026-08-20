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
	movedDate := timezone.NewDate(2026, 3, 10)
	movedEndDate := timezone.NewDate(2026, 3, 11)

	effective := appointmentWithOverride(appointment, occurrence, &calModels.AppointmentOccurrenceOverride{
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

// A claim is taken per guardian profile before the push is dispatched. Between
// claim and dispatch a guardian can lose access, and those stale claims have to
// be released — otherwise the reminder is never retried for them.
func TestWithoutReminderProfiles(t *testing.T) {
	t.Parallel()

	claimed := []int64{11, 12, 13}

	assert.Equal(t, []int64{12}, withoutReminderProfiles(claimed, []int64{11, 13}),
		"a guardian who is no longer reachable must have their claim released")
	assert.Empty(t, withoutReminderProfiles(claimed, claimed))
	assert.Equal(t, claimed, withoutReminderProfiles(claimed, nil),
		"losing every recipient releases every claim")
	assert.Empty(t, withoutReminderProfiles(nil, claimed))
}
