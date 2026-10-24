package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var childNavigationDefinitions = []config.Definition{
	{
		Key:             config.KeyChildSearchOnlyNavigation,
		Label:           "Kinder nur über „Alle Kinder“ finden",
		Description:     "In der Navigation steht nur noch „Alle Kinder“. Oben auf dieser Seite wählen Sie, welche Kinder Sie sehen. Weitere Seiten öffnen Sie über das Menü (⋮).",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       2,
	},
}
