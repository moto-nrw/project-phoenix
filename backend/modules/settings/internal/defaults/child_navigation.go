package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var childNavigationDefinitions = []config.Definition{
	{
		Key:             config.KeyChildSearchOnlyNavigation,
		Label:           "Kinder nur über „Alle Kinder“ finden",
		Description:     "In der Navigation steht nur noch „Alle Kinder“. Dort wählen Sie oben „Meine Gruppe“ oder „Meine Aufsicht“. Gruppenseite und Aufsicht öffnen Sie über das Menü (⋮) auf dieser Seite.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       2,
	},
}
