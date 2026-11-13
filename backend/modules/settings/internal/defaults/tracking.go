package defaults

import (
	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
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

	registerDefinitions(tracking_init_kinder_0Definitions, tracking_init_zeiterfassung_1Definitions)

	// --- Automatic checkout at planned shift end (#1798) ---
	// Opt-in: staff who forget to clock out are checked out automatically at
	// the end of their planned shift (schedule.staff_shifts) plus a grace
	// window. Staff without a planned shift are untouched.

}

var tracking_init_kinder_0Definitions = []config.Definition{
	{
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
	},
	{
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
	},
	{
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
	},
	{
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
	},
}

var tracking_init_zeiterfassung_1Definitions = []config.Definition{
	{
		Key:             config.KeyTrackingAutoCheckoutEnabled,
		Label:           "Automatisch ausstempeln",
		Description:     "Wer das Ausstempeln vergisst, wird zum geplanten Dienstende ausgestempelt. Gilt nur bei einer geplanten Schicht im Dienstplan.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       2,
	},
	{
		Key:             config.KeyTrackingAutoCheckoutGraceMinutes,
		Label:           "Wartezeit nach Dienstende (Minuten)",
		Description:     "So lange wartet moto nach dem geplanten Dienstende, bevor es automatisch ausstempelt.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       3,
		Validation:      config.Range(0, 240),
		DependsOn:       config.DependsOnEq(config.KeyTrackingAutoCheckoutEnabled, true),
	},
	// --- Arbeitszeit-Gutschrift (#6851, #6852) ---
	// Both apply to work blocks closed after the change: the rule is stored
	// on each block when it is closed, so earlier blocks keep their values.
	// Order: stamped duration, plus pack-up time, then rounding, then breaks.
	{
		Key:             config.KeyTrackingPackUpMinutes,
		Label:           "Wegpackzeit beim Ausstempeln (Minuten)",
		Description:     "Diese Minuten zählt moto bei jedem Ausstempeln zur Arbeitszeit dazu, zum Beispiel für das Aufräumen. Nicht beim Pausenbeginn und nicht beim automatischen Ausstempeln. 0 heißt: aus. Gilt ab jetzt, frühere Tage bleiben gleich.",
		Type:            config.FieldNumber,
		Default:         0,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       9,
		Validation:      config.Range(0, 60),
	},
	{
		Key:             config.KeyTrackingRoundingIntervalMinutes,
		Label:           "Arbeitszeit runden",
		Description:     "moto rundet die Dauer jedes Arbeitsblocks, nachdem die Wegpackzeit dazugezählt ist. Danach wird die Pause abgezogen. Die gestempelten Uhrzeiten bleiben unverändert sichtbar. Gilt ab jetzt, frühere Tage bleiben gleich.",
		Type:            config.FieldSelect,
		Default:         config.TrackingRoundingOff,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       10,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nicht runden", Value: config.TrackingRoundingOff},
				{Label: "Auf 5 Minuten", Value: "5"},
				{Label: "Auf 10 Minuten", Value: "10"},
				{Label: "Auf 15 Minuten", Value: "15"},
				{Label: "Auf 30 Minuten", Value: "30"},
			},
		},
	},
	{
		Key:             config.KeyTrackingRoundingMode,
		Label:           "Wie gerundet wird",
		Description:     "Kaufmännisch: ab der Hälfte nach oben, sonst nach unten. Bei 15 Minuten wird 1:07 Std. zu 1:00 Std. und 1:08 Std. zu 1:15 Std.",
		Type:            config.FieldSelect,
		Default:         config.TrackingRoundingNearest,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       11,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Kaufmännisch", Value: config.TrackingRoundingNearest},
				{Label: "Immer aufrunden", Value: config.TrackingRoundingUp},
				{Label: "Immer abrunden", Value: config.TrackingRoundingDown},
			},
		},
		DependsOn: config.DependsOnNeq(config.KeyTrackingRoundingIntervalMinutes, config.TrackingRoundingOff),
	},
}

var indicatorPattern = `^[a-zA-ZäöüÄÖÜß\s]{0,30}$`
