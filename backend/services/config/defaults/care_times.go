package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

// Care time presets (#3371). The weekly plan copies the clock time into an
// empty field on one click; nothing applies to a child until the team saves.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyCareDefaultArrivalTime,
		Label:           "Übliche Ankunftszeit",
		Description:     "Im Wochenplan eines Kindes tragen Sie diese Uhrzeit mit einem Klick ein. Für kein Kind gilt sie von selbst. Gilt die Klassenzeit, erscheint der Knopf nicht.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "betreuungszeiten",
		SortOrder:       2,
		AccessPolicy:    config.AccessShared,
	})

	config.Register(config.Definition{
		Key:             config.KeyCareDefaultPickupTime,
		Label:           "Übliche Abholzeit",
		Description:     "Im Wochenplan eines Kindes tragen Sie diese Uhrzeit mit einem Klick ein. Für kein Kind gilt sie von selbst.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "betreuungszeiten",
		SortOrder:       3,
		AccessPolicy:    config.AccessShared,
	})
}
