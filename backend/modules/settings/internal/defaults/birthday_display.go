package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var birthdayDisplayDefinitions = []config.Definition{
	{
		Key:             config.KeyBirthdayDisplayEnabled,
		Label:           "Geburtstage auf der Startseite",
		Description:     "Zeigt auf der Startseite, wer in dieser Woche Geburtstag hat. Man kann bis zu 4 Wochen zurück- und vorblättern. Kinder ohne hinterlegtes Geburtsdatum erscheinen nicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "startseite",
		Category:        "geburtstage",
		SortOrder:       1,
	},
	{
		Key:             config.KeyBirthdayDisplayIncludeStaff,
		Label:           "Geburtstage von Mitarbeitenden mitanzeigen",
		Description:     "Zeigt auf der Startseite auch die Geburtstage des Personals, ohne Geburtsjahr. Jede Person kann sich im eigenen Profil davon abmelden.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "startseite",
		Category:        "geburtstage",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyBirthdayDisplayEnabled, true),
	},
}
