package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Session End ---

	config.Register(config.Definition{
		Key:             "operations.session_end_enabled",
		Label:           "Automatisches Sitzungsende",
		Description:     "Alle aktiven Sitzungen werden automatisch zur konfigurierten Uhrzeit beendet",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sessions",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             "operations.session_end_time",
		Label:           "Sitzungsende Uhrzeit",
		Description:     "Uhrzeit, zu der alle aktiven Sitzungen automatisch beendet werden",
		Type:            config.FieldTime,
		Default:         "18:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sessions",
		SortOrder:       2,
		DependsOn: &config.Dependency{
			Key:       "operations.session_end_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	minTimeout := float64(1)
	maxTimeout := float64(60)
	config.Register(config.Definition{
		Key:             "operations.session_end_timeout_minutes",
		Label:           "Sitzungsende Timeout (Minuten)",
		Description:     "Maximale Dauer für den automatischen Sitzungsende-Vorgang",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sessions",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minTimeout, Max: &maxTimeout},
		DependsOn: &config.Dependency{
			Key:       "operations.session_end_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Student Daily Checkout ---

	config.Register(config.Definition{
		Key:             "operations.student_daily_checkout_time",
		Label:           "Tägliche Abmeldezeit",
		Description:     "Uhrzeit, ab der Schüler aus dem Heimraum abgemeldet werden können",
		Type:            config.FieldTime,
		Default:         "15:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       1,
	})

	// --- Abandoned Session Cleanup ---

	config.Register(config.Definition{
		Key:             "operations.session_cleanup_enabled",
		Label:           "Bereinigung verlassener Sitzungen",
		Description:     "Automatische Bereinigung von Sitzungen ohne Aktivität",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "cleanup",
		SortOrder:       1,
	})

	minInterval := float64(5)
	maxInterval := float64(120)
	config.Register(config.Definition{
		Key:             "operations.session_cleanup_interval_minutes",
		Label:           "Bereinigungsintervall (Minuten)",
		Description:     "Wie oft nach verlassenen Sitzungen geprüft wird",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "cleanup",
		SortOrder:       2,
		Validation:      &config.ValidationRules{Min: &minInterval, Max: &maxInterval},
		DependsOn: &config.Dependency{
			Key:       "operations.session_cleanup_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	minThreshold := float64(10)
	maxThreshold := float64(480)
	config.Register(config.Definition{
		Key:             "operations.session_abandoned_threshold_minutes",
		Label:           "Inaktivitätsschwelle (Minuten)",
		Description:     "Minuten ohne Aktivität, bevor eine Sitzung als verlassen gilt",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "cleanup",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minThreshold, Max: &maxThreshold},
		DependsOn: &config.Dependency{
			Key:       "operations.session_cleanup_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Break Auto-End ---

	minBreakInterval := float64(10)
	maxBreakInterval := float64(300)
	config.Register(config.Definition{
		Key:             "operations.break_auto_end_interval_seconds",
		Label:           "Pausen-Auto-Ende Intervall (Sekunden)",
		Description:     "Wie oft geprüft wird, ob Pausen automatisch beendet werden sollen",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "cleanup",
		SortOrder:       4,
		Validation:      &config.ValidationRules{Min: &minBreakInterval, Max: &maxBreakInterval},
	})
}
