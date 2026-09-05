package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

func init() {
	config.Register(config.Definition{
		Key:             config.KeyCalendarCalDAVEnabled,
		Label:           "Kalenderzugang mit App-Passwort erlauben",
		Description:     "Mitarbeitende können ihren moto-Kalender mit Benutzername und App-Passwort verbinden. Der normale Abo-Link bleibt weiterhin verfügbar. Termine können nur angesehen werden.",
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
