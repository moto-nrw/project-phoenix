package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Notification settings (#1419 4d). Email notifications around the staff
// absence approval workflow. Brand-new setting with no legacy env var, so
// consumers resolve it directly without an env fallback.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyNotificationsAbsenceApprovalEmail,
		Label:           "E-Mails zu Abwesenheitsanträgen",
		Description:     "Sendet E-Mails, wenn Mitarbeitende einen Abwesenheitsantrag stellen und wenn die Leitung genehmigt, ablehnt oder eine Rückfrage stellt.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       5,
	})

	// Feature flag for the notification abstraction (#1624). On by default
	// since the personal-notification epic: every producer behind it now
	// requires the recipient's own consent, and consent starts empty, so an
	// enabled school still delivers nothing until someone asks for something.
	// Off would only mean that a person's own choice in their profile is
	// silently ignored.
	config.Register(config.Definition{
		Key:             config.KeyNotificationsDispatchEnabled,
		Label:           "Benachrichtigungen aktivieren",
		Description:     "Aktiviert die zentrale Benachrichtigungs-Funktion (In-App-Hinweise und Push-Nachrichten). Standardmäßig aktiv. Was tatsächlich ankommt, wählt jede Person im eigenen Profil.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       1,
	})

	// Delivery window. Checked once in the notification router, so it bounds
	// every producer rather than each one carrying its own quiet hours.
	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
		Key:             config.KeyNotificationsOnDutyOnly,
		Label:           "Nur im Dienst benachrichtigen",
		Description:     "Persönliche Hinweise erreichen nur Personen, die gerade eingestempelt sind. Schulen ohne Zeiterfassung müssen diese Einstellung ausschalten.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       4,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsDispatchEnabled, true),
	})

	// Health information about a child fanning out to a group's phones is a
	// data-minimisation decision for the school, not a matter of personal
	// taste, so it exists on top of the per-person opt-in.
	config.Register(config.Definition{
		Key:             config.KeyNotificationsAbsenceReportedEnabled,
		Label:           "Krankmeldungen melden",
		Description:     "Erlaubt Hinweise an die Gruppe und die Leitung, wenn für ein Kind eine Krankmeldung oder Entschuldigung eingetragen wird. Jede Person entscheidet zusätzlich im eigenen Profil.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       5,
	})

	// Cancellation notice to families (#2601). The gate is separate from the
	// parent-news flag: news is an optional broadcast channel, the notice is a
	// service obligation. The two companions only shape the dialog and the
	// delivery, so they hide when the gate is off.
	config.Register(config.Definition{
		Key:             config.KeyNotificationsCareCancelledEnabled,
		Label:           "Eltern bei Ausfall informieren",
		Description:     "Erlaubt beim Absagen eines Betreuungstermins eine Mitteilung an die Eltern der betroffenen Kinder. Die Mitteilung erscheint im Elternportal, auch wenn die Neuigkeiten ausgeschaltet sind.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       6,
	})

	config.Register(config.Definition{
		Key:             config.KeyNotificationsCareCancelledDefaultOn,
		Label:           "Eltern informieren vorauswählen",
		Description:     "Beim Absagen ist \"Eltern informieren\" bereits angehakt. Die absagende Person kann den Haken jederzeit entfernen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       7,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsCareCancelledEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyNotificationsCareCancelledEmail,
		Label:           "Ausfall zusätzlich per E-Mail",
		Description:     "Schickt die Ausfall-Mitteilung zusätzlich als E-Mail an die Eltern. Ohne diese Einstellung bleibt es bei Elternportal und Push-Nachricht.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       8,
		DependsOn:       config.DependsOnEq(config.KeyNotificationsCareCancelledEnabled, true),
	})
}
