package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// Display/UI settings
	settings.MustRegister(settings.Definition{
		Key:         "display.theme",
		Type:        config.ValueTypeEnum,
		Default:     "system",
		Category:    "appearance",
		Tab:         "display",
		Label:       "Farbschema",
		Description: "Das Farbschema der Benutzeroberfläche",
		Scopes:      []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumOptions: []settings.EnumOption{
			{Value: "light", Label: "Hell"},
			{Value: "dark", Label: "Dunkel"},
			{Value: "system", Label: "Systemstandard"},
		},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 10,
	})

	settings.MustRegister(settings.Definition{
		Key:         "display.items_per_page",
		Type:        config.ValueTypeEnum,
		Default:     "50",
		Category:    "pagination",
		Tab:         "display",
		Label:       "Einträge pro Seite",
		Description: "Standardanzahl der Einträge pro Seite in Listen",
		Scopes:      []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumOptions: []settings.EnumOption{
			{Value: "25", Label: "25 Einträge"},
			{Value: "50", Label: "50 Einträge"},
			{Value: "100", Label: "100 Einträge"},
		},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 20,
	})

	settings.MustRegister(settings.Definition{
		Key:         "display.date_format",
		Type:        config.ValueTypeEnum,
		Default:     "DD.MM.YYYY",
		Category:    "format",
		Tab:         "display",
		Label:       "Datumsformat",
		Description: "Wie Datumsangaben angezeigt werden",
		Scopes:      []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumOptions: []settings.EnumOption{
			{Value: "DD.MM.YYYY", Label: "31.12.2024 (Deutsch)"},
			{Value: "YYYY-MM-DD", Label: "2024-12-31 (ISO)"},
			{Value: "MM/DD/YYYY", Label: "12/31/2024 (US)"},
		},
		ViewPerm:     "",
		EditPerm:     "",
		DisplayOrder: 30,
	})

	settings.MustRegister(settings.Definition{
		Key:         "display.time_format",
		Type:        config.ValueTypeEnum,
		Default:     "24h",
		Category:    "format",
		Tab:         "display",
		Label:       "Zeitformat",
		Description: "Wie Uhrzeiten angezeigt werden",
		Scopes:      []config.Scope{config.ScopeSystem, config.ScopeUser},
		EnumOptions: []settings.EnumOption{
			{Value: "24h", Label: "24-Stunden (14:30)"},
			{Value: "12h", Label: "12-Stunden (2:30 PM)"},
		},
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

	// Demo setting for testing settings functionality
	settings.MustRegister(settings.Definition{
		Key:          "display.show_welcome_banner",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "appearance",
		Tab:          "display",
		Label:        "Willkommensbanner anzeigen",
		Description:  "Zeigt einen Willkommensbanner auf der Einstellungsseite",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "",
		EditPerm:     "config:manage",
		DisplayOrder: 5,
	})
}
