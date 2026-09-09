package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		title, body := calendarNotificationCopy(kind, "")
		assert.NotEmpty(t, title)
		assert.NotEmpty(t, body)
		assert.NotContains(t, body, "Planning", "the appointment title must not reach the payload")
	}
}

func TestCalendarNotificationCopyLocale(t *testing.T) {
	t.Parallel()
	title, _ := calendarNotificationCopy("appointment_cancelled", "en")
	assert.Equal(t, "Appointment cancelled", title)
	title, _ = calendarNotificationCopy("appointment_reminder", "")
	assert.Equal(t, "Terminerinnerung", title)
}
