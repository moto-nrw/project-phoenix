package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             config.KeyCheckoutRaumwechselEnabled,
		Label:           "Raumwechsel-Button anzeigen",
		Description:     "Zeigt den Raumwechsel-Button auf dem Geräte-Checkout-Bildschirm an",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       10,
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceNFCEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckoutSchulhofEnabled,
		Label:           "Schulhof-Button anzeigen",
		Description:     "Zeigt den Schulhof-Button auf dem Geräte-Checkout-Bildschirm an (Schulhof-Raum wird automatisch erstellt)",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       11,
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceNFCEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckoutWCEnabled,
		Label:           "Toilette-Button anzeigen",
		Description:     "Zeigt den Toilette-Button auf dem Geräte-Checkout-Bildschirm an (WC-Raum wird automatisch erstellt)",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       12,
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceNFCEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// Device online/offline window (issue #586 — Rule 12 extraction). The
	// number of minutes a device's last_seen timestamp may be in the past
	// before it is treated as offline for health monitoring.
	minOnlineWindow := float64(1)
	maxOnlineWindow := float64(60)
	config.Register(config.Definition{
		Key:             config.KeyDeviceOnlineWindowMinutes,
		Label:           "Online-Fenster für Geräte (Minuten)",
		Description:     "Minuten, in denen ein Gerät zuletzt gesehen worden sein muss, um als online zu gelten. Danach wird es als offline behandelt.",
		Type:            config.FieldNumber,
		Default:         5,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "monitoring",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Min: &minOnlineWindow, Max: &maxOnlineWindow},
		AccessPolicy:    config.AccessOperatorOnly,
	})
}
