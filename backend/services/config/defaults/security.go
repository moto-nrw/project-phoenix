package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:             config.KeyOGSDevicePIN,
		Label:           "OGS Geräte-PIN",
		Description:     "PIN für die Authentifizierung an RFID-Geräten. Wird als Klartext gespeichert und in der Oberfläche maskiert.",
		Type:            config.FieldPassword,
		Default:         "1234",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "devices",
		Category:        "pin",
		SortOrder:       1,
		Validation:      &config.ValidationRules{Pattern: &pinPattern},
		AccessPolicy:    config.AccessAdminOnly,
	})
}
