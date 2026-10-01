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
	//
	// The feature stays off by default, but the first two labels come
	// prefilled (#3738): every school using it tracks lunch and homework, so
	// switching it on shows something useful right away. Schools that enabled
	// the feature before the prefill keep their exact labels (migration
	// 1.15.436 pins empty slots). They sit in the "Kinder" category next to
	// the child photos, the other switch for what a Kinderkarte shows.

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicatorsEnabled,
		Label:           "Häkchen auf der Kinderkarte",
		Description:     "Zeigt auf jeder Kinderkarte, ob das Kind heute schon an einem bestimmten Ort war, zum Beispiel in der Mensa. Ein grüner Haken heißt: Das Kind war heute schon dort.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "kinder",
		SortOrder:       1,
	})

	indicatorPattern := `^[a-zA-ZäöüÄÖÜß\s]{0,30}$`

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator1,
		Label:           "Häkchen 1",
		Description:     "Dieses Wort steht auf der Kinderkarte. Der Haken erscheint, wenn der Name eines besuchten Raums oder einer Aktivität das Wort enthält. Ein leeres Feld zeigt nichts an.",
		Type:            config.FieldText,
		Default:         "Mensa",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "kinder",
		SortOrder:       2,
		Validation:      &config.ValidationRules{Pattern: &indicatorPattern},
		DependsOn:       config.DependsOnEq(config.KeyTrackingIndicatorsEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator2,
		Label:           "Häkchen 2",
		Description:     "Dieses Wort steht auf der Kinderkarte. Der Haken erscheint, wenn der Name eines besuchten Raums oder einer Aktivität das Wort enthält. Ein leeres Feld zeigt nichts an.",
		Type:            config.FieldText,
		Default:         "Hausaufgaben",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "kinder",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Pattern: &indicatorPattern},
		DependsOn:       config.DependsOnEq(config.KeyTrackingIndicatorsEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyTrackingIndicator3,
		Label:           "Häkchen 3",
		Description:     "Dieses Wort steht auf der Kinderkarte. Der Haken erscheint, wenn der Name eines besuchten Raums oder einer Aktivität das Wort enthält. Ein leeres Feld zeigt nichts an.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "kinder",
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
