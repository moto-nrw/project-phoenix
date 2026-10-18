package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var enrollment_registerEnrollmentMaster_allgemein_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentEnabled,
		Label:           "Online-Anmeldung",
		Description:     "Eltern melden ihr Kind über ein Formular im Internet an. Ausgeschaltet ist die Anmeldeseite nicht erreichbar.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "allgemein",
		SortOrder:       1,
	},
}
