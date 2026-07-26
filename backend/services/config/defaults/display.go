package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Info-point display settings (issue #1325). The feature is opt-in and
// defaults OFF: a school must explicitly enable it before the admin UI
// (/info-displays) and the public dashboard endpoint become reachable.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyDisplayEnabled,
		Label:           "Info-Displays aktivieren",
		Description:     "Ermöglicht die Erstellung von Info-Displays für große Bildschirme im Eingangsbereich.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "info-displays",
		SortOrder:       1,
	})
}
