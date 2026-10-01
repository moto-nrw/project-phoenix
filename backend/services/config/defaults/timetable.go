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
		Label:           "Betreuungsplan nutzen",
		Description:     "Zeigt den Betreuungsplan im Menü. Die Einstellungen dazu erscheinen darunter.",
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
		Label:           "Geplante Aktivitäten automatisch starten",
		Description:     "Startet geplante Aktivitäten zur eingetragenen Uhrzeit. Ausgeschaltet erscheint ein Hinweis, und Ihr Team startet sie selbst.",
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
		Label:           "Start vor Planbeginn erlaubt (Minuten)",
		Description:     "So viele Minuten vor der geplanten Startzeit kann Ihr Team eine Aktivität starten.",
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
		Label:           "Beenden erst ab geplanter Endzeit",
		Description:     "Eine geplante Aktivität lässt sich erst ab ihrer eingetragenen Endzeit beenden. Spontane Aktivitäten sind ausgenommen.",
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
		Description:     "So viele Minuten nach der geplanten Startzeit zeigt moto eine Aktivität als überfällig an.",
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
		Label:           "Betreuungsschlüssel (Kinder pro Betreuungskraft)",
		Description:     "So viele Kinder soll eine Betreuungskraft in einem Termin höchstens allein betreuen. Sind es mehr, zeigt moto eine mögliche Unterbesetzung.",
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
		Label:           "Betreuungsplan aufbewahren (Tage)",
		Description:     "So lange bleiben beendete oder abgesagte Termine gespeichert.",
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
		Label:           "Beginn der Wochenansicht",
		Description:     "Ab dieser Uhrzeit zeigt die Wochenansicht den Tag. Frühere Termine erreichen Sie durch Scrollen.",
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
		Label:           "Ende der Wochenansicht",
		Description:     "Bis zu dieser Uhrzeit zeigt die Wochenansicht den Tag. Spätere Termine erreichen Sie durch Scrollen.",
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
		Label:           "Frühe Abholung bis",
		Description:     "Kinder mit Abholzeit bis einschließlich dieser Uhrzeit stehen unter „Listen“ in der Ganztagsliste mit früher Abholung.",
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
		Label:           "Späte Abholung bis",
		Description:     "Kinder mit späterer Abholzeit bis einschließlich dieser Uhrzeit stehen unter „Listen“ in der Ganztagsliste mit später Abholung.",
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
