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

	// Feature flag for the notification abstraction (#1624). Default off:
	// Notify(ctx, event) is a no-op until a school explicitly enables it.
	config.Register(config.Definition{
		Key:             config.KeyNotificationsDispatchEnabled,
		Label:           "Benachrichtigungen aktivieren",
		Description:     "Aktiviert die zentrale Benachrichtigungs-Funktion (In-App-Hinweise, Grundlage für spätere Erinnerungen und Push-Nachrichten). Standardmäßig deaktiviert.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "benachrichtigungen",
		SortOrder:       1,
	})
}
