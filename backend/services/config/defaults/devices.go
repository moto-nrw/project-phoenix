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
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckoutSchulhofEnabled,
		Label:           "Schulhof-Button anzeigen",
		Description:     "Zeigt den Schulhof-Button auf dem Geräte-Checkout-Bildschirm an (setzt voraus, dass ein Schulhof-Raum existiert)",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       2,
	})

	config.Register(config.Definition{
		Key:             config.KeyCheckoutWCEnabled,
		Label:           "Toilette-Button anzeigen",
		Description:     "Zeigt den Toilette-Button auf dem Geräte-Checkout-Bildschirm an (setzt voraus, dass ein WC-Raum existiert)",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "checkout",
		SortOrder:       3,
	})
}
