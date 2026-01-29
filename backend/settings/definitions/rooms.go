package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// Room and check-in related settings
	settings.MustRegister(settings.Definition{
		Key:           "checkin.default_room",
		Type:          config.ValueTypeObjectRef,
		Default:       "",
		Category:      "checkin",
		Tab:           "general",
		Label:         "Standard Check-in Raum",
		Description:   "Der Standardraum für Check-ins wenn kein Raum angegeben ist",
		Scopes:        []config.Scope{config.ScopeSystem, config.ScopeDevice},
		ObjectRefType: "room",
		ObjectRefFilter: map[string]interface{}{
			"is_active": true,
		},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 10,
	})

	settings.MustRegister(settings.Definition{
		Key:          "checkin.auto_checkout_enabled",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "checkout",
		Tab:          "general",
		Label:        "Automatisches Auschecken",
		Description:  "Schüler werden automatisch ausgecheckt wenn die geplante Abholzeit erreicht ist",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 20,
	})

	settings.MustRegister(settings.Definition{
		Key:          "checkin.checkout_grace_period_minutes",
		Type:         config.ValueTypeInt,
		Default:      "15",
		Category:     "checkout",
		Tab:          "general",
		Label:        "Karenzzeit beim Auschecken (Minuten)",
		Description:  "Zusätzliche Zeit nach der geplanten Abholzeit bevor automatisch ausgecheckt wird",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(0, 60),
		DisplayOrder: 30,
	})

	settings.MustRegister(settings.Definition{
		Key:          "room.max_capacity_warning_percent",
		Type:         config.ValueTypeInt,
		Default:      "90",
		Category:     "capacity",
		Tab:          "general",
		Label:        "Kapazitätswarnung (%)",
		Description:  "Ab welcher Auslastung eine Warnung angezeigt wird",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(50, 100),
		DisplayOrder: 40,
	})
}
