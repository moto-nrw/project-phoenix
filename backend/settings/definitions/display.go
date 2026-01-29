package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// Display/UI settings
	settings.MustRegister(settings.Definition{
		Key:          "display.theme",
		Type:         config.ValueTypeEnum,
		Default:      "system",
		Category:     "appearance",
		Tab:          "display",
		Label:        "Farbschema",
		Description:  "Das Farbschema der Benutzeroberfläche",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumValues:   []string{"light", "dark", "system"},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 10,
	})

	settings.MustRegister(settings.Definition{
		Key:          "display.items_per_page",
		Type:         config.ValueTypeEnum,
		Default:      "50",
		Category:     "pagination",
		Tab:          "display",
		Label:        "Einträge pro Seite",
		Description:  "Standardanzahl der Einträge pro Seite in Listen",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumValues:   []string{"25", "50", "100"},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 20,
	})

	settings.MustRegister(settings.Definition{
		Key:          "display.date_format",
		Type:         config.ValueTypeEnum,
		Default:      "DD.MM.YYYY",
		Category:     "format",
		Tab:          "display",
		Label:        "Datumsformat",
		Description:  "Wie Datumsangaben angezeigt werden",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumValues:   []string{"DD.MM.YYYY", "YYYY-MM-DD", "MM/DD/YYYY"},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 30,
	})

	settings.MustRegister(settings.Definition{
		Key:          "display.time_format",
		Type:         config.ValueTypeEnum,
		Default:      "24h",
		Category:     "format",
		Tab:          "display",
		Label:        "Zeitformat",
		Description:  "Wie Uhrzeiten angezeigt werden",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumValues:   []string{"24h", "12h"},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 40,
	})

	// Device-specific display settings
	settings.MustRegister(settings.Definition{
		Key:          "display.screen_timeout_seconds",
		Type:         config.ValueTypeInt,
		Default:      "300",
		Category:     "device",
		Tab:          "display",
		Label:        "Bildschirmtimeout (Sekunden)",
		Description:  "Zeit bis der Bildschirm automatisch gesperrt wird",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeDevice},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(30, 3600),
		DisplayOrder: 100,
	})

	settings.MustRegister(settings.Definition{
		Key:          "display.show_student_photos",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "privacy",
		Tab:          "display",
		Label:        "Schülerfotos anzeigen",
		Description:  "Zeigt Fotos der Schüler in Listen und Profilen",
		Scopes:       []config.Scope{config.ScopeSystem, config.ScopeDevice},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 110,
	})
}
