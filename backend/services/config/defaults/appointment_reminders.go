package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Appointment reminder settings (issue #1671).
//
// These sit on the "Erinnerungen" tab next to the staff reminders, but they
// address the other side: guardians who were invited to an appointment get a
// reminder shortly before it happens.
//
// The default is ON, which is deliberate and narrower than it looks. A reminder
// only ever goes to an appointment whose organizer ticked "Eltern per E-Mail
// benachrichtigen" when creating it (appointments.notify_guardians) — the same
// opt-in that already sends the published/updated/cancelled mails. So a school
// that switches this off is suppressing a second mail about an appointment its
// parents were mailed about anyway, not losing a channel it never had.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyCalendarAppointmentReminderEnabled,
		Label:           "Terminerinnerung für Eltern",
		Description:     "Erinnert Eltern kurz vor einem Termin noch einmal per E-Mail. Gilt nur für Termine, bei denen beim Anlegen die Benachrichtigung der Eltern gewählt wurde.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "termine",
		SortOrder:       20,
	})

	config.Register(config.Definition{
		Key:             config.KeyCalendarAppointmentReminderLeadHours,
		Label:           "Vorlaufzeit Terminerinnerung (Stunden)",
		Description:     "Wie viele Stunden vor dem Termin die Erinnerung verschickt wird.",
		Type:            config.FieldNumber,
		Default:         24,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "termine",
		SortOrder:       21,
		Validation:      config.Range(1, 168),
		DependsOn:       config.DependsOnEq(config.KeyCalendarAppointmentReminderEnabled, true),
	})
}
