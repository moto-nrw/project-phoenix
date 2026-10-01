package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             config.KeyCheckoutRaumwechselEnabled,
		Label:           "„Raumwechsel“ am Tablet anbieten",
		Description:     "Kinder können beim Auschecken am Tablet „Raumwechsel“ wählen.",
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
		Label:           "„Schulhof“ am Tablet anbieten",
		Description:     "Kinder können beim Auschecken am Tablet „Schulhof“ wählen. moto legt dafür einen Raum für den Schulhof an.",
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
		Label:           "„Toilette“ am Tablet anbieten",
		Description:     "Kinder können beim Auschecken am Tablet „Toilette“ wählen. moto legt dafür einen Raum für die Toilette an.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       12,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckoutDailyFromAllRoomsEnabled,
		Label:           "„Nach Hause“ in jedem Raum anzeigen",
		Description:     "Ausgeschaltet geht „Nach Hause“ nur im Gruppenraum und auf dem Schulhof.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "nach-hause",
		SortOrder:       8,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	// Capacity-detail disclosure toggles (issue #1879). When enabled, the
	// device checkin 409 includes the `details` object (name + occupancy)
	// that the kiosk renders as a rich German message; when disabled, the
	// response carries no details and the kiosk shows a generic hint.
	// Both default ON (#3633): without the activity's name, staff read the
	// generic hint as "room full" and raised the room capacity, which changes
	// nothing. Schools with a stored value keep it.
	config.Register(config.Definition{
		Key:             config.KeyCheckinActivityCapacityDetailsEnabled,
		Label:           "Name und Belegung bei voller Aktivität",
		Description:     "Das Tablet zeigt dann zum Beispiel „Fußball AG ist voll (20/20 Teilnehmer)“. So sieht Ihr Team, dass die Aktivität voll ist und nicht der Raum. Ausgeschaltet erscheint nur ein allgemeiner Hinweis.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "kapazität",
		SortOrder:       14,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckinRoomCapacityDetailsEnabled,
		Label:           "Name und Belegung bei vollem Raum",
		Description:     "Das Tablet zeigt dann zum Beispiel „Turnhalle ist voll (30/30 Plätze belegt)“. Ausgeschaltet erscheint nur ein allgemeiner Hinweis.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "kapazität",
		SortOrder:       15,
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
