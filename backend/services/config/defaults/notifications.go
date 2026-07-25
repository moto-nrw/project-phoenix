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

	// E-Mail-Zustellung (#1937). Only terminal failures notify: a temporary
	// delay is something the mail server resolves on its own, and alerting on
	// every retry would train the office to ignore the alerts.
	config.Register(config.Definition{
		Key:             config.KeyEmailDeliveryAlertsEnabled,
		Label:           "Warnung bei unzustellbaren E-Mails",
		Description:     "Benachrichtigt die hinterlegten Adressen, wenn eine E-Mail an Eltern endgültig nicht zugestellt werden konnte oder als Spam gemeldet wurde. Vorübergehende Verzögerungen lösen keine Benachrichtigung aus.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "e-mail",
		SortOrder:       10,
	})

	config.Register(config.Definition{
		Key:             config.KeyEmailDeliveryAlertEmails,
		Label:           "Empfänger für Zustellwarnungen",
		Description:     "Kommagetrennte E-Mail-Adressen, die über unzustellbare Eltern-E-Mails informiert werden.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "e-mail",
		SortOrder:       11,
		DependsOn:       config.DependsOnEq(config.KeyEmailDeliveryAlertsEnabled, true),
	})
}
