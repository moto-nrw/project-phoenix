package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var attendanceChannelsDefinitions = []config.Definition{
	{
		Key:             config.KeyAttendanceWebEnabled,
		Label:           "Anwesenheit am Handy oder Computer erfassen",
		Description:     "Das Team kann Kinder an- und abmelden. Das geht auch zusätzlich zu NFC-Geräten.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit-erfassen",
		SortOrder:       0,
		AccessPolicy:    config.AccessOperatorOnly,
	},
}
