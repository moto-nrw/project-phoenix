package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Tracking Indicators ---
	// Opt-in feature: shows in student cards whether a student has visited
	// specific rooms/activities today (e.g., "Hausaufgaben", "Mensa").
	// Admins configure up to 3 free-text labels; the backend matches them
	// against today's visit history (activity group name + room name).

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicatorsEnabled,
		Label:           "Aktivitäts-Indikatoren",
		Description:     "Zeigt in den Schülerkarten an, ob ein Kind heute bereits in bestimmten Räumen oder Aktivitäten war",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "indikatoren",
		SortOrder:       1,
	})

	indicatorPattern := `^[a-zA-ZäöüÄÖÜß\s]{0,30}$`

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator1,
		Label:           "Indikator 1",
		Description:     "Suchbegriff für Raum- oder Aktivitätsnamen (z.B. 'Mensa', 'Hausaufgaben')",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "indikatoren",
		SortOrder:       2,
		Validation:      &config.ValidationRules{Pattern: &indicatorPattern},
		DependsOn: &config.Dependency{
			Key:       config.KeyTrackingIndicatorsEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator2,
		Label:           "Indikator 2",
		Description:     "Suchbegriff für Raum- oder Aktivitätsnamen (z.B. 'Mensa', 'Hausaufgaben')",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "indikatoren",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Pattern: &indicatorPattern},
		DependsOn: &config.Dependency{
			Key:       config.KeyTrackingIndicatorsEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator3,
		Label:           "Indikator 3",
		Description:     "Suchbegriff für Raum- oder Aktivitätsnamen (z.B. 'Mensa', 'Hausaufgaben')",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "indikatoren",
		SortOrder:       4,
		Validation:      &config.ValidationRules{Pattern: &indicatorPattern},
		DependsOn: &config.Dependency{
			Key:       config.KeyTrackingIndicatorsEnabled,
			Condition: "eq",
			Value:     true,
		},
	})
}
