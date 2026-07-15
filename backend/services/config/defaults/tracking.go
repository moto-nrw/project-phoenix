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
		Description:     "Zeigt in das Kindkarten an, ob ein Kind heute bereits in bestimmten Räumen oder Aktivitäten war",
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
		DependsOn:       config.DependsOnEq(config.KeyTrackingIndicatorsEnabled, true),
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
		DependsOn:       config.DependsOnEq(config.KeyTrackingIndicatorsEnabled, true),
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
		DependsOn:       config.DependsOnEq(config.KeyTrackingIndicatorsEnabled, true),
	})

	// --- Automatic checkout at planned shift end (#1798) ---
	// Opt-in: staff who forget to clock out are checked out automatically at
	// the end of their planned shift (schedule.staff_shifts) plus a grace
	// window. Staff without a planned shift are untouched.

	config.Register(config.Definition{
		Key:             config.KeyTrackingAutoCheckoutEnabled,
		Label:           "Automatische Ausstempelung",
		Description:     "Stempelt Mitarbeitende automatisch zum geplanten Dienstende aus, wenn sie vergessen haben, sich abzumelden. Gilt nur für Mitarbeitende mit geplanter Schicht im Dienstplan.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       2,
	})

	config.Register(config.Definition{
		Key:             config.KeyTrackingAutoCheckoutGraceMinutes,
		Label:           "Karenzzeit (Minuten)",
		Description:     "Wartezeit nach dem geplanten Dienstende, bevor automatisch ausgestempelt wird",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       3,
		Validation:      config.Range(0, 240),
		DependsOn:       config.DependsOnEq(config.KeyTrackingAutoCheckoutEnabled, true),
	})
}
