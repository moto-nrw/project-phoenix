package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var staffMessagingDefinitions = []config.Definition{
	{
		Key:             config.KeyStaffMessagingEnabled,
		Label:           "Team-Chat für Mitarbeitende",
		Description:     "Mitarbeitende und Lehrkräfte Ihrer Schule schreiben sich in moto Nachrichten. Eltern sehen davon nichts. Ausgeschaltet ist der Team-Chat für niemanden sichtbar.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "team",
		SortOrder:       1,
	},
}
