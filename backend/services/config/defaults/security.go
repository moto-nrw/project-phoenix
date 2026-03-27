package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             "security.ogs_device_pin",
		Label:           "OGS Geräte-PIN",
		Description:     "PIN für die Authentifizierung an RFID-Geräten. Wird als Klartext gespeichert und in der Oberfläche maskiert.",
		Type:            config.FieldPassword,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "auth",
		SortOrder:       1,
	})
}
