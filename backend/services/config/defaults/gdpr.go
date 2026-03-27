package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             "gdpr.data_cleanup_enabled",
		Label:           "Automatische Datenbereinigung",
		Description:     "Automatische Löschung abgelaufener Besuchsdaten gemäß Datenschutzeinstellungen",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "cleanup",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             "gdpr.data_cleanup_time",
		Label:           "Bereinigungszeitpunkt",
		Description:     "Uhrzeit für die tägliche Datenbereinigung",
		Type:            config.FieldTime,
		Default:         "02:00",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "cleanup",
		SortOrder:       2,
		DependsOn: &config.Dependency{
			Key:       "gdpr.data_cleanup_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	minTimeout := float64(5)
	maxTimeout := float64(120)
	config.Register(config.Definition{
		Key:             "gdpr.data_cleanup_timeout_minutes",
		Label:           "Bereinigung Timeout (Minuten)",
		Description:     "Maximale Dauer für den Bereinigungsvorgang",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "cleanup",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minTimeout, Max: &maxTimeout},
		DependsOn: &config.Dependency{
			Key:       "gdpr.data_cleanup_enabled",
			Condition: "eq",
			Value:     true,
		},
	})
}
