package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Holiday registration of the Träger (#4703): what the holiday care wish
// of the online registration does in a holiday period whose office switched
// it on. The customer decided precedence for now (decision of 09.11.); the
// strict variant stays a choice of the Träger. Enrollment knows the values.
func init() {
	registerDefinitions(holidayRegistrationDefinitions)
}

var holidayRegistrationDefinitions = []config.Definition{
	{
		Key:             config.KeyHolidayRegistrationWishMode,
		Levels:          config.LevelTraeger,
		Label:           "Was gilt ohne Ferienwunsch?",
		Description:     "Gilt für Ferien, bei denen Sie den Ferienwunsch berücksichtigen. Den Ferienwunsch geben Eltern in der OGS-Anmeldung an. Feriengäste ohne OGS-Platz sind nicht betroffen.",
		Type:            config.FieldSelect,
		Default:         "priority",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-anmeldung",
		SortOrder:       5,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Kinder mit Ferienwunsch haben Vorrang", Value: "priority"},
				{Label: "Nur Kinder mit Ferienwunsch können sich anmelden", Value: "wish_only"},
			},
		},
	},
}
