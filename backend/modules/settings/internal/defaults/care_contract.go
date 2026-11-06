package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// How the contracts of a school come about (#4680, ADR 0088 section 6). The
// Träger sets the way for all its schools, a school may take the other one.
// The values are the constants of the Betreuungsvertrag
// (carecontract.ModeSendLater, carecontract.ModeOnAcceptance).
func init() {
	registerDefinitions(careContractDefinitions)
}

var careContractDefinitions = []config.Definition{
	{
		Key:             config.KeyCareContractConclusionMode,
		Levels:          config.LevelBoth,
		Label:           "Wann gilt der Betreuungsvertrag?",
		Description:     "Bei der Annahme: Nimmt die Schule eine Anmeldung an, gilt der Vertrag sofort. Die Eltern bekommen ihn per E-Mail und in der Eltern-App. Sie müssen nichts unterschreiben. Später verschicken: Das Träger-Büro erstellt den Vertrag und schickt ihn den Eltern.",
		Type:            config.FieldSelect,
		Default:         "nachtraeglich",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "vertragsabschluss",
		SortOrder:       90,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Später verschicken", Value: "nachtraeglich"},
				{Label: "Bei der Annahme", Value: "bei_annahme"},
			},
		},
	},
}
