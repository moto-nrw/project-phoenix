package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Session End (system tab — automated background process) ---

	config.Register(config.Definition{
		Key:             config.KeySessionEndEnabled,
		Label:           "Automatisches Sitzungsende",
		Description:     "Alle aktiven Sitzungen werden automatisch zur konfigurierten Uhrzeit beendet",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       1,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeySessionEndTime,
		Label:           "Sitzungsende Uhrzeit",
		Description:     "Uhrzeit, zu der alle aktiven Sitzungen automatisch beendet werden",
		Type:            config.FieldTime,
		Default:         "18:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       2,
		DependsOn: &config.Dependency{
			Key:       config.KeySessionEndEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	minTimeout := float64(1)
	maxTimeout := float64(60)
	config.Register(config.Definition{
		Key:             config.KeySessionEndTimeoutMinutes,
		Label:           "Sitzungsende Timeout (Minuten)",
		Description:     "Maximale Dauer für den automatischen Sitzungsende-Vorgang",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minTimeout, Max: &maxTimeout},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionEndEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	// --- Student Daily Checkout ---

	config.Register(config.Definition{
		Key:             config.KeyStudentDailyCheckoutTime,
		Label:           "Tägliche Abmeldezeit",
		Description:     "Uhrzeit, ab der Schüler aus dem Heimraum abgemeldet werden können. Wenn leer, ist die Abmeldung jederzeit möglich.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutEnabled,
		Label:           "Individuelle Abholzeiten verwenden",
		Description:     "Wenn aktiviert, wird die Abmeldung anhand der individuellen Abholzeiten der Schüler angezeigt statt der globalen Abmeldezeit.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       2,
	})

	minDelta := float64(0)
	maxDelta := float64(120)
	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutDeltaMinutes,
		Label:           "Vorlaufzeit vor Abholung (Minuten)",
		Description:     "Minuten vor der Abholzeit, ab der die Abmeldung am Gerät angeboten wird. Beispiel: Bei Abholzeit 15:00 und Vorlaufzeit 15 Min. ist die Abmeldung ab 14:45 möglich.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minDelta, Max: &maxDelta},
		DependsOn: &config.Dependency{
			Key:       config.KeyPerStudentCheckoutEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Abandoned Session Cleanup (system tab — automated background process) ---

	config.Register(config.Definition{
		Key:             config.KeySessionCleanupEnabled,
		Label:           "Bereinigung verlassener Sitzungen",
		Description:     "Automatische Bereinigung von Sitzungen ohne Aktivität",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	minInterval := float64(5)
	maxInterval := float64(120)
	config.Register(config.Definition{
		Key:             config.KeySessionCleanupIntervalMinutes,
		Label:           "Bereinigungsintervall (Minuten)",
		Description:     "Wie oft nach verlassenen Sitzungen geprüft wird",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       11,
		Validation:      &config.ValidationRules{Min: &minInterval, Max: &maxInterval},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	minThreshold := float64(10)
	maxThreshold := float64(480)
	config.Register(config.Definition{
		Key:             config.KeySessionAbandonedThresholdMin,
		Label:           "Inaktivitätsschwelle (Minuten)",
		Description:     "Minuten ohne Aktivität, bevor eine Sitzung als verlassen gilt",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       12,
		Validation:      &config.ValidationRules{Min: &minThreshold, Max: &maxThreshold},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	// --- Admin Supervision Overview ---

	config.Register(config.Definition{
		Key:             config.KeyAdminSupervisionOverview,
		Label:           "Administrator-Aufsichtsübersicht",
		Description:     "Administratoren können alle aktiven Aufsichten und anwesende Kinder einsehen",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       1,
	})

	// Break auto-end interval is NOT registered here — it controls a global ticker
	// (not per-tenant) and is configured via BREAK_AUTO_END_INTERVAL_SECONDS env var only.

	// --- Status flag auto-clear (Krank / Entschuldigt badge lifecycle) ---

	statusFlagOptions := &config.SelectOptions{
		Static: []config.SelectOption{
			{Label: "Manuell (nur durch Betreuer)", Value: config.ClearModeManual},
			{Label: "Beim nächsten Check-in", Value: config.ClearModeNextCheckin},
			{Label: "Am Ende des Tages", Value: config.ClearModeEndOfDay},
		},
	}

	config.Register(config.Definition{
		Key:             config.KeySickClearMode,
		Label:           "Krankmeldung automatisch beenden",
		Description:     "Legt fest, wann die Krankmeldung eines Schülers automatisch aufgehoben wird.",
		Type:            config.FieldSelect,
		Default:         config.ClearModeNextCheckin,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "status_flags",
		SortOrder:       30,
		Options:         statusFlagOptions,
	})

	config.Register(config.Definition{
		Key:             config.KeyExcusedClearMode,
		Label:           "Entschuldigung automatisch beenden",
		Description:     "Legt fest, wann die Entschuldigung eines Schülers automatisch aufgehoben wird.",
		Type:            config.FieldSelect,
		Default:         config.ClearModeEndOfDay,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "status_flags",
		SortOrder:       31,
		Options:         statusFlagOptions,
	})
}
