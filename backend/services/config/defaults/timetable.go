package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Timetable settings (WP-B7). Per-tenant configuration for the activity
// template → instance materialization pipeline, the staff-facing auto-start
// behaviour, and the GDPR retention window for completed/cancelled instances.
//
// The top-level timetable feature is opt-out, so tenants see the navigation
// entry and related settings unless they explicitly disable it. Materialization
// is opt-out too: `materialization_enabled` defaults to TRUE, and the three
// materialization settings are operator-only because the cadence is platform
// plumbing. `auto_start_planned` stays opt-in and defaults to FALSE.
//
// The weekday option values match ISO 8601 numbering (1 = Monday … 7 = Sunday)
// so they slot directly into time.Weekday comparisons after the usual +1 shift.
func init() {
	// --- Materialization (operations tab) ---

	timetableEnabledDependency := config.DependsOnEq(config.KeyTimetableEnabled, true)

	config.Register(config.Definition{
		Key:             config.KeyTimetableEnabled,
		Label:           "Betreuungsplan aktivieren",
		Description:     "Zeigt den Betreuungsplan in der Navigation an und schaltet die passenden Einstellungen frei.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       29,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationEnabled,
		Label:           "Wiederkehrende Termine automatisch vorbereiten",
		Description:     "Legt aus den wiederkehrenden Aktivitäten automatisch die konkreten Termine für die kommenden Wochen an.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       30,
		AccessPolicy:    config.AccessOperatorOnly,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationWeekday,
		Label:           "Termine vorbereiten am",
		Description:     "Wochentag, an dem die Termine für die kommenden Wochen angelegt werden.",
		Type:            config.FieldSelect,
		Default:         5, // Friday (ISO 8601: Monday=1 … Sunday=7)
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       31,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Montag", Value: 1},
				{Label: "Dienstag", Value: 2},
				{Label: "Mittwoch", Value: 3},
				{Label: "Donnerstag", Value: 4},
				{Label: "Freitag", Value: 5},
				{Label: "Samstag", Value: 6},
				{Label: "Sonntag", Value: 7},
			},
		},
		DependsOn: config.DependsOnEq(config.KeyTimetableMaterializationEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationWeeksAhead,
		Label:           "Vorlauf (Wochen)",
		Description:     "Anzahl der Wochen, die im Voraus angelegt werden.",
		Type:            config.FieldNumber,
		Default:         1,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       32,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      config.Range(1, 4),
		DependsOn:       config.DependsOnEq(config.KeyTimetableMaterializationEnabled, true),
	})

	// --- Auto-start & staff UX (operations tab) ---

	config.Register(config.Definition{
		Key:             config.KeyTimetableAutoStartPlanned,
		Label:           "Automatischer Start geplanter Aktivitäten",
		Description:     "Startet geplante Aktivitäten automatisch zur eingetragenen Uhrzeit. Wenn deaktiviert, werden sie als Hinweis angezeigt und manuell gestartet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       33,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableAutoEndEnabled,
		Label:           "Laufende Termine automatisch beenden",
		Description:     "Beendet gestartete Termine aus dem Betreuungsplan nach Endzeit und Puffer. Spontane Aktivitäten bleiben offen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       34,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableAutoEndGraceMinutes,
		Label:           "Puffer nach Endzeit (Minuten)",
		Description:     "moto wartet diese Minuten nach der eingetragenen Endzeit.",
		Type:            config.FieldNumber,
		Default:         0,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       35,
		Validation:      config.Range(0, 120),
		DependsOn:       config.DependsOnEq(config.KeyTimetableAutoEndEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableStartLeadMinutes,
		Label:           "Aktivitäten vor Planbeginn starten (Minuten)",
		Description:     "Legt fest, wie viele Minuten vor der geplanten Startzeit eine Aktivität gestartet werden kann.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       36,
		Validation:      config.Range(0, 120),
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableEnforcePlannedEnd,
		Label:           "Aktivitäten nur nach Planende beenden",
		Description:     "Verhindert, dass eine geplante Aktivität vor ihrer eingetragenen Endzeit beendet wird. Spontane Aktivitäten sind ausgenommen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       37,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableOverdueThresholdMinutes,
		Label:           "Als überfällig markieren nach (Minuten)",
		Description:     "Minuten nach der geplanten Startzeit, ab denen eine Aktivität als überfällig angezeigt wird.",
		Type:            config.FieldNumber,
		Default:         5,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       38,
		Validation:      config.Range(1, 30),
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableShowExpectedChildrenCount,
		Label:           "Erwartete Kinderzahl anzeigen",
		Description:     "Zeigt bei einer Aktivität, wie viele Kinder erwartet werden und wie viele bereits anwesend sind.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       39,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableChildrenPerStaffRatio,
		Label:           "Betreuungsschlüssel (Kinder pro Betreuer)",
		Description:     "Anzahl der Kinder, die eine Betreuungskraft in einem Termin höchstens allein betreuen soll. Wird verwendet, um eine mögliche Unterbesetzung anzuzeigen.",
		Type:            config.FieldNumber,
		Default:         12,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       40,
		Validation:      config.Range(1, 30),
		DependsOn:       timetableEnabledDependency,
	})

	// --- GDPR retention (gdpr tab) ---

	// Timetable retention is an independent window from KeyDataCleanupEnabled:
	// activity_instances rows may age out on a different cadence than the live
	// attendance data cleaned up by the scheduler job. Gate visibility on the
	// existing data-cleanup toggle so the GDPR UI surfaces both settings as a
	// coherent unit when cleanup is enabled.
	config.Register(config.Definition{
		Key:             config.KeyGDPRTimetableRetentionDays,
		Label:           "Aufbewahrungsdauer Betreuungsplan (Tage)",
		Description:     "Anzahl der Tage, für die abgeschlossene oder abgesagte Termine gespeichert bleiben.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "stundenplan",
		SortOrder:       30,
		Validation:      config.Range(30, 1825),
		DependsOn:       config.DependsOnEq(config.KeyTimetableEnabled, true),
	})

	// --- Display range (operations tab) ---

	// The admin weekly calendar (Apple-style grid) renders hour rows between
	// these two HH:MM times by default. Events outside the window are still
	// rendered and become reachable via scroll. Defaults match the typical
	// OGS day (09:00 Schulende → 17:00 Abholung).
	config.Register(config.Definition{
		Key:             config.KeyTimetableDayStartTime,
		Label:           "Tagesansicht Beginn",
		Description:     "Uhrzeit, ab der die Wochenansicht standardmäßig beginnt. Frühere Termine bleiben per Scrollen erreichbar.",
		Type:            config.FieldTime,
		Default:         "09:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       42,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableDayEndTime,
		Label:           "Tagesansicht Ende",
		Description:     "Uhrzeit, bis zu der die Wochenansicht standardmäßig angezeigt wird. Spätere Termine bleiben per Scrollen erreichbar.",
		Type:            config.FieldTime,
		Default:         "17:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       43,
		DependsOn:       timetableEnabledDependency,
	})

	// --- Slot-list pickup buckets (operations tab) ---

	config.Register(config.Definition{
		Key:             config.KeySlotListShortDayCutoff,
		Label:           "Kurzer Ganztag bis",
		Description:     "Abholzeit-Grenze für die kurze Ganztagsliste. Kinder mit Abholzeit bis einschließlich dieser Uhrzeit erscheinen in dieser Kohorte.",
		Type:            config.FieldTime,
		Default:         "14:30",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       44,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeySlotListLongDayCutoff,
		Label:           "Langer Ganztag bis",
		Description:     "Abholzeit-Grenze für die lange Ganztagsliste. Kinder nach dem kurzen Ganztag und bis einschließlich dieser Uhrzeit erscheinen in dieser Kohorte.",
		Type:            config.FieldTime,
		Default:         "16:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       45,
		DependsOn:       timetableEnabledDependency,
	})
}
