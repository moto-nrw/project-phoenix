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
}
