package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             config.KeyFeedbackEnabled,
		Label:           "Feedback der Kinder beim Gehen",
		Description:     "Kinder zeigen beim Gehen am Tablet mit einem Smiley, wie ihr Tag war. Das Team sieht die Rückmeldungen beim Kind. Aus Datenschutzgründen ist das zunächst ausgeschaltet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "feedback",
		SortOrder:       20,
	})

	config.Register(config.Definition{
		Key:             config.KeyFeedbackDataRetentionDays,
		Label:           "Rückmeldungen aufbewahren (Tage)",
		Description:     "Danach löscht moto die Rückmeldungen der Kinder.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "feedback",
		SortOrder:       21,
		Validation:      config.Range(7, 365),
		DependsOn:       config.DependsOnEq(config.KeyFeedbackEnabled, true),
	})
}
