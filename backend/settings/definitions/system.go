package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// System settings
	settings.MustRegister(settings.Definition{
		Key:         "system.app_name",
		Type:        config.ValueTypeString,
		Default:     "Project Phoenix",
		Category:    "system",
		Tab:         "general",
		Label:       "Anwendungsname",
		Description: "Der Name der Anwendung, der in der Benutzeroberfläche angezeigt wird",
		Scopes:      []config.Scope{config.ScopeSystem},
		ViewPerm:    "",
		EditPerm:    "config:manage",
		Validation:  settings.StringLength(1, 100),
	})

	settings.MustRegister(settings.Definition{
		Key:         "system.maintenance_mode",
		Type:        config.ValueTypeBool,
		Default:     "false",
		Category:    "system",
		Tab:         "general",
		Label:       "Wartungsmodus",
		Description: "Wenn aktiviert, können nur Administratoren auf das System zugreifen",
		Scopes:      []config.Scope{config.ScopeSystem},
		ViewPerm:    "config:read",
		EditPerm:    "config:manage",
	})

	settings.MustRegister(settings.Definition{
		Key:         "system.default_language",
		Type:        config.ValueTypeEnum,
		Default:     "de",
		Category:    "system",
		Tab:         "general",
		Label:       "Standardsprache",
		Description: "Die Standardsprache für neue Benutzer",
		Scopes:      []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumValues:  []string{"de", "en"},
		ViewPerm:    "",
		EditPerm:    "config:manage",
	})

	settings.MustRegister(settings.Definition{
		Key:          "system.session_timeout_minutes",
		Type:         config.ValueTypeInt,
		Default:      "30",
		Category:     "session",
		Tab:          "security",
		Label:        "Sitzungstimeout (Minuten)",
		Description:  "Zeit in Minuten bis eine inaktive Sitzung beendet wird",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeDevice},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(5, 480),
		DisplayOrder: 10,
	})

	settings.MustRegister(settings.Definition{
		Key:          "system.max_login_attempts",
		Type:         config.ValueTypeInt,
		Default:      "5",
		Category:     "session",
		Tab:          "security",
		Label:        "Maximale Anmeldeversuche",
		Description:  "Anzahl der fehlgeschlagenen Anmeldeversuche bevor das Konto gesperrt wird",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(3, 10),
		DisplayOrder: 20,
	})
}
