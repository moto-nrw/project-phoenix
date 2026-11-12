package defaults

import (
	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// The signature of the care contract (#4679): after how many days a contract
// the parents have not signed yet is reminded. Only the Träger sets it; 0
// sends no reminder. The Betreuungsvertrag sends at most three reminders.
func init() {
	registerDefinitions(careContractSignatureDefinitions)
}

var careContractSignatureDefinitions = []config.Definition{
	{
		Key:             config.KeyCareContractSignatureReminderDays,
		Levels:          config.LevelTraeger,
		Label:           "Erinnerung an die Unterschrift nach (Tagen)",
		Description:     "Haben die Eltern einen Vertrag nach so vielen Tagen nicht unterschrieben, bekommen sie eine Erinnerung per Mail. Es gibt höchstens drei Erinnerungen. Bei 0 gibt es keine Erinnerung.",
		Type:            config.FieldNumber,
		Default:         7,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "vertraege",
		Category:        "vertraege-unterschrift",
		SortOrder:       10,
		Validation:      config.Range(0, 60),
	},
}
