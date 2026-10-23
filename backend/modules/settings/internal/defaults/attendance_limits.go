package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var attendanceLimitsDefinitions = []config.Definition{
	{
		Key:             config.KeyWebSpontaneousActivities,
		Label:           "Spontane Aktivitäten erlauben",
		Description:     "Mitarbeitende können am Computer oder Handy unter \"Aktuelle Aufsicht\" spontan eine Aktivität starten. Die Aktivität belegt den Raum und steht danach im Tages- und Wochenplan.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       42,
	},
	{
		Key:             config.KeyWebExceedParticipantLimit,
		Label:           "Mehr Kinder als die Teilnehmergrenze erlauben",
		Description:     "Gilt, wenn Mitarbeitende am Computer oder Handy Kinder einer Aktivität zuordnen. Eingeschaltet: Die Teilnehmergrenze darf dort überschritten werden. Ausgeschaltet: Ist die Aktivität voll, wird kein Kind zugeordnet. Am Tablet gilt die Grenze immer.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       43,
	},
}
