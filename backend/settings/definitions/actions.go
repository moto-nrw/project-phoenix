package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// System maintenance actions
	settings.MustRegister(settings.Definition{
		Key:          "system.clear_cache",
		Type:         config.ValueTypeAction,
		Category:     "maintenance",
		Tab:          "system",
		DisplayOrder: 100,
		Label:        "Cache leeren",
		Description:  "Löscht den internen Anwendungscache",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Icon:         "trash",
		// Action-specific
		ActionEndpoint:            "/api/settings/actions/system.clear_cache/execute",
		ActionMethod:              "POST",
		ActionRequiresConfirmation: true,
		ActionConfirmationTitle:   "Cache leeren?",
		ActionConfirmationMessage: "Alle zwischengespeicherten Daten werden gelöscht. Dies kann die Systemleistung kurzzeitig beeinträchtigen.",
		ActionConfirmationButton:  "Cache leeren",
		ActionSuccessMessage:      "Cache erfolgreich geleert",
		ActionErrorMessage:        "Fehler beim Leeren des Caches",
		ActionIsDangerous:         false,
	})

	settings.MustRegister(settings.Definition{
		Key:          "system.sync_settings",
		Type:         config.ValueTypeAction,
		Category:     "maintenance",
		Tab:          "system",
		DisplayOrder: 110,
		Label:        "Einstellungen synchronisieren",
		Description:  "Synchronisiert alle Einstellungsdefinitionen mit der Datenbank",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Icon:         "refresh",
		// Action-specific
		ActionEndpoint:            "/api/settings/actions/system.sync_settings/execute",
		ActionMethod:              "POST",
		ActionRequiresConfirmation: true,
		ActionConfirmationTitle:   "Einstellungen synchronisieren?",
		ActionConfirmationMessage: "Alle Einstellungsdefinitionen werden neu in die Datenbank geschrieben.",
		ActionConfirmationButton:  "Synchronisieren",
		ActionSuccessMessage:      "Einstellungen erfolgreich synchronisiert",
		ActionErrorMessage:        "Fehler bei der Synchronisierung",
		ActionIsDangerous:         false,
	})

	// Email action
	settings.MustRegister(settings.Definition{
		Key:          "email.test_connection",
		Type:         config.ValueTypeAction,
		Category:     "smtp",
		Tab:          "email",
		DisplayOrder: 100,
		Label:        "Verbindung testen",
		Description:  "Sendet eine Test-E-Mail an die angegebene Adresse",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Icon:         "mail",
		// Action-specific
		ActionEndpoint:            "/api/settings/actions/email.test_connection/execute",
		ActionMethod:              "POST",
		ActionRequiresConfirmation: true,
		ActionConfirmationTitle:   "Test-E-Mail senden?",
		ActionConfirmationMessage: "Eine Test-E-Mail wird an die konfigurierte Absenderadresse gesendet.",
		ActionConfirmationButton:  "Test senden",
		ActionSuccessMessage:      "Test-E-Mail erfolgreich gesendet",
		ActionErrorMessage:        "Fehler beim Senden der Test-E-Mail",
		ActionIsDangerous:         false,
	})
}
