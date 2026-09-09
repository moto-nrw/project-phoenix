package appointments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClock(hour, minute int) time.Time {
	return time.Date(1, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func validAppointment() *Appointment {
	return &Appointment{
		OrganizerStaffID: 1,
		Title:            "  Elternabend  ",
		StartDate:        NewDate(2026, 1, 5),
		EndDate:          NewDate(2026, 1, 5),
		StartTime:        testClock(9, 0),
		EndTime:          testClock(10, 0),
		DeliveryMode:     DeliveryModeRSVPRequired,
	}
}

func TestAppointmentValidateDefaultsAndTrims(t *testing.T) {
	t.Parallel()

	appointment := validAppointment()

	require.NoError(t, appointment.Validate())

	assert.Equal(t, "Elternabend", appointment.Title)
	assert.Equal(t, OverviewVisibilityOrganizer, appointment.OverviewVisibility)
}

func TestAppointmentValidateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Appointment)
		message string
	}{
		{name: "organizer required", mutate: func(a *Appointment) { a.OrganizerStaffID = 0 }, message: "organizer_staff_id is required"},
		{name: "title required", mutate: func(a *Appointment) { a.Title = "  " }, message: "title is required"},
		{name: "start date required", mutate: func(a *Appointment) { a.StartDate = "" }, message: "start_date is required"},
		{name: "end date required", mutate: func(a *Appointment) { a.EndDate = "" }, message: "end_date is required"},
		{name: "date order", mutate: func(a *Appointment) { a.EndDate = a.StartDate.AddDays(-1) }, message: "end_date must be on or after start_date"},
		{name: "same day time order", mutate: func(a *Appointment) { a.EndTime = a.StartTime }, message: "end_time must be after start_time on same-day appointments"},
		{name: "delivery mode", mutate: func(a *Appointment) { a.DeliveryMode = "email" }, message: "delivery_mode must be rsvp_required or informational"},
		{name: "overview visibility", mutate: func(a *Appointment) { a.OverviewVisibility = "parents" }, message: "overview_visibility must be organizer, staff, or all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appointment := validAppointment()
			tt.mutate(appointment)

			require.EqualError(t, appointment.Validate(), tt.message)
		})
	}
}

func TestAppointmentValidateAllowsAllDayAndMultiDayTimeOrder(t *testing.T) {
	t.Parallel()

	allDay := validAppointment()
	allDay.AllDay = true
	allDay.EndTime = allDay.StartTime
	require.NoError(t, allDay.Validate())

	multiDay := validAppointment()
	multiDay.EndDate = multiDay.StartDate.AddDays(1)
	multiDay.EndTime = testClock(8, 0)
	require.NoError(t, multiDay.Validate())
}
