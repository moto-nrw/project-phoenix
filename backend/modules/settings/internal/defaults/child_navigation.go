package defaults

import config "github.com/moto-ogs/project-phoenix/modules/settings/records"

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
	{
		// #6956: Betreuungskräfte, die wenig mit Technik arbeiten, verlieren
		// in der vollen Seitenleiste den Überblick. Die Seiten selbst und die
		// Rechte bleiben unverändert; jede Person kann alle Seiten einblenden.
		Key:             config.KeySimpleStaffNavigation,
		Label:           "Einfache Seitenleiste für die Betreuung",
		Description:     "Wer die Schule nicht leitet, sieht nur die Seiten für den Betreuungsalltag: Startseite, Heute, Gruppen, Alle Kinder, Aktuelle Aufsicht, Räume, Zeiterfassung, Mein Kalender und Tagesinformationen. Jede Person kann unten in der Seitenleiste „Alle Seiten anzeigen“ wählen. Die Rechte ändern sich nicht.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       3,
	},
}
