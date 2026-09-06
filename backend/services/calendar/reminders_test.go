package calendar

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
		title, body := localizedAppointmentNotificationCopy(kind, "")
		assert.NotEmpty(t, title)
		assert.NotEmpty(t, body)
		assert.NotContains(t, body, "Planning", "the appointment title must not reach the payload")
	}
}
