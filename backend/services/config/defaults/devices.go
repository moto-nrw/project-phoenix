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
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
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
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
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
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	// Capacity-detail disclosure toggles (issue #1879). When enabled, the
	// device checkin 409 includes the `details` object (name + occupancy)
	// that the kiosk renders as a rich German message; when disabled, the
	// response carries no details and the kiosk shows a generic hint.
	// Activity defaults OFF (the rich message was never visible before);
	// room defaults ON (preserves the behavior schools already have).
	config.Register(config.Definition{
		Key:             config.KeyCheckinActivityCapacityDetailsEnabled,
		Label:           "Details bei voller Aktivität anzeigen",
		Description:     "Zeigt beim Erreichen der Teilnehmergrenze den Namen der Aktivität und die Belegung auf dem Gerät an (z. B. Fußball AG ist voll (20/20 Teilnehmer)). Wenn deaktiviert, erscheint nur ein allgemeiner Hinweis.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "kapazität",
		SortOrder:       13,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckinRoomCapacityDetailsEnabled,
		Label:           "Details bei vollem Raum anzeigen",
		Description:     "Zeigt beim Erreichen der Raumkapazität den Namen des Raums und die Belegung auf dem Gerät an (z. B. Turnhalle ist voll (30/30 Plätze belegt)). Wenn deaktiviert, erscheint nur ein allgemeiner Hinweis.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "kapazität",
		SortOrder:       14,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	// Device online/offline window (issue #586 — Rule 12 extraction). The
	// number of minutes a device's last_seen timestamp may be in the past
	// before it is treated as offline for health monitoring.
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
		Validation:      config.Range(1, 60),
		AccessPolicy:    config.AccessOperatorOnly,
	})
}
