package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Notification settings (#1419 4d). Email notifications around the staff
// absence approval workflow. Brand-new setting with no legacy env var, so
// consumers resolve it directly without an env fallback.
func init() {
	registerDefinitions(notifications_init_zeiterfassung_0Definitions, notifications_init_benachrichtigungen_1Definitions)

	// Feature flag for the notification abstraction (#1624). On by default
	// since the personal-notification epic: every producer behind it now
	// requires the recipient's own consent, and consent starts empty, so an
	// enabled school still delivers nothing until someone asks for something.
	// Off would only mean that a person's own choice in their profile is
	// silently ignored.

	// Delivery window. Checked once in the notification router, so it bounds
	// every producer rather than each one carrying its own quiet hours.

	// Health information about a child fanning out to a group's phones is a
	// data-minimisation decision for the school, not a matter of personal
	// taste, so it exists on top of the per-person opt-in.

}

var notifications_init_zeiterfassung_0Definitions = []config.Definition{
	{
		Key:             config.KeyNotificationsAbsenceApprovalEmail,
		Label:           "E-Mails zu Abwesenheitsanträgen",
		Description:     "Sendet E-Mails, wenn Mitarbeitende einen Abwesenheitsantrag stellen und wenn die Leitung genehmigt, ablehnt oder eine Rückfrage stellt.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       8,
	},
}

var notifications_init_benachrichtigungen_1Definitions = []config.Definition{
	{
		Key:             config.KeyNotificationsDispatchEnabled,
		Label:           "Benachrichtigungen senden",
		Description:     "Hinweise in moto und auf dem Handy. Was ankommt, wählt jede Person im eigenen Profil.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       1,
	},
	{
		Key:             config.KeyNotificationsActiveWindowStart,
		Label:           "Benachrichtigungen ab",
		Description:     "Ab dieser Uhrzeit werden Benachrichtigungen zugestellt.",
		Type:            config.FieldTime,
		Default:         "06:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsDispatchEnabled, true),
	},
	{
		Key:             config.KeyNotificationsActiveWindowEnd,
		Label:           "Benachrichtigungen bis",
		Description:     "Bis zu dieser Uhrzeit werden Benachrichtigungen zugestellt. Danach bleibt es still bis zum nächsten Morgen.",
		Type:            config.FieldTime,
		Default:         "18:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       3,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsDispatchEnabled, true),
	},
	{
		Key:             config.KeyNotificationsOnDutyOnly,
		Label:           "Nur im Dienst benachrichtigen",
		Description:     "Persönliche Hinweise erreichen nur Personen, die gerade eingestempelt sind. Nutzen Sie das nur, wenn Ihr Team die Zeiterfassung verwendet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       4,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsDispatchEnabled, true),
	},
	{
		Key:             config.KeyNotificationsAbsenceReportedEnabled,
		Label:           "Hinweis bei Krankmeldung",
		Description:     "OGS-Admins können einen Hinweis bekommen. Bei festen Gruppen auch die Betreuung der Gruppe. In moto schule auch die Lehrkräfte der Klasse und die Schulverwaltung. Jede Person entscheidet selbst.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       5,
	},
}
