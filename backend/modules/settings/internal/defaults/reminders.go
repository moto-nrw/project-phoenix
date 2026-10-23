package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Reminder settings (issue #1457). Visual-only reminders for staff — NO sound.
// Every type defaults OFF; a school opts in per event type. Surfaced on the
// "Erinnerungen" page (sidebar). All resolution is request-time and tenant
// scoped, so these are plain settings with no env-var fallback.
func init() {
	// --- Anstehende / überfällige Abholung ---

	registerDefinitions(reminders_init_abholung_0Definitions, reminders_init_aktivitaeten_1Definitions)

	// --- Aktivitätsbeginn (jede geplante Aktivität, nicht nur AGs) ---

}

var reminders_init_abholung_0Definitions = []config.Definition{
	{
		Key:             config.KeyRemindersPickupUpcomingEnabled,
		Label:           "Anstehende Abholung",
		Description:     "Zeigt eine Erinnerung an, wenn die Abholzeit eines Kindes näher rückt.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "abholung",
		SortOrder:       1,
	},
	{
		Key:             config.KeyRemindersPickupUpcomingLeadMinutes,
		Label:           "Vorlaufzeit anstehende Abholung (Minuten)",
		Description:     "Wie viele Minuten vor der Abholzeit die Erinnerung erscheint.",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "abholung",
		SortOrder:       2,
		Validation:      config.Range(1, 120),
		DependsOn:       config.DependsOnEq(config.KeyRemindersPickupUpcomingEnabled, true),
	},
	{
		Key:             config.KeyRemindersPickupOverdueEnabled,
		Label:           "Überfällige Abholung",
		Description:     "Zeigt eine Erinnerung an, wenn ein Kind nach seiner Abholzeit noch anwesend ist.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "abholung",
		SortOrder:       3,
	},
}

var reminders_init_aktivitaeten_1Definitions = []config.Definition{
	{
		Key:             config.KeyRemindersActivityStartEnabled,
		Label:           "Aktivitätsbeginn",
		Description:     "Zeigt eine Erinnerung an, wenn eine geplante Aktivität bald startet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "aktivitaeten",
		SortOrder:       10,
	},
	{
		Key:             config.KeyRemindersActivityStartLeadMinutes,
		Label:           "Vorlaufzeit Aktivitätsbeginn (Minuten)",
		Description:     "Wie viele Minuten vor dem Start einer Aktivität die Erinnerung erscheint.",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "aktivitaeten",
		SortOrder:       11,
		Validation:      config.Range(1, 120),
		DependsOn:       config.DependsOnEq(config.KeyRemindersActivityStartEnabled, true),
	},
	{
		Key:             config.KeyRemindersActivityOverdueEnabled,
		Label:           "Überfällige Aktivität",
		Description:     "Zeigt eine Erinnerung an, wenn eine geplante Aktivität nicht rechtzeitig gestartet wurde. Wann das ist, legen Sie unter „Betrieb“ beim Tages- und Wochenplan fest.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "aktivitaeten",
		SortOrder:       12,
	},
}
