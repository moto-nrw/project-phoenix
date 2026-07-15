package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Reminder settings (issue #1457). Visual-only reminders for staff — NO sound.
// Every type defaults OFF; a school opts in per event type. Surfaced on the
// "Erinnerungen" page (sidebar). All resolution is request-time and tenant
// scoped, so these are plain settings with no env-var fallback.
func init() {
	// --- Anstehende / überfällige Abholung ---

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	// --- Aktivitätsbeginn (jede geplante Aktivität, nicht nur AGs) ---

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
		Key:             config.KeyRemindersActivityOverdueEnabled,
		Label:           "Überfällige Aktivität",
		Description:     "Zeigt eine Erinnerung an, wenn eine geplante Aktivität nicht rechtzeitig gestartet wurde. Die Schwelle richtet sich nach der Überfälligkeits-Einstellung im Betreuungsplan.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "reminders",
		Category:        "aktivitaeten",
		SortOrder:       12,
	})
}
