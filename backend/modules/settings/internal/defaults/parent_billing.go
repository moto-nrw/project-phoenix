package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Holiday billing of the Träger (#5621): whether its office enters the
// debtor number of the family when it confirms a holiday registration. Only
// the Träger sets it; Bissendorf switches it on, other Träger leave it off.
func init() {
	registerDefinitions(parentBillingDefinitions)
}

var parentBillingDefinitions = []config.Definition{
	{
		Key:             config.KeyParentBillingDebtorNumberOnConfirm,
		Levels:          config.LevelTraeger,
		Label:           "Debitorennummer beim Bestätigen abfragen",
		Description:     "Ist das an, tragen Sie beim Bestätigen einer Ferienanmeldung die Debitorennummer der Familie ein. Sie steht dann im Schreiben als Verwendungszweck.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-bestaetigung",
		SortOrder:       10,
	},
}
