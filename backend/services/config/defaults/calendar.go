package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

func init() {
	config.Register(config.Definition{
		Key:             config.KeyCalendarCalDAVEnabled,
		Label:           "Kalenderzugang mit App-Passwort erlauben",
		Description:     "Zweiter Weg neben dem Abo-Link. Mitarbeitende erstellen sich in moto ein App-Passwort. Damit melden sie sich im Kalenderprogramm an. Termine können sie nur ansehen. Kalenderprogramme nennen diesen Zugang CalDAV.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "schnittstellen",
		SortOrder:       1,
		AccessPolicy:    config.AccessShared,
	})
}
