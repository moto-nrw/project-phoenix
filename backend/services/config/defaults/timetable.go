package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Timetable settings (WP-B7). Per-tenant configuration for the activity
// template → instance materialization pipeline, the staff-facing auto-start
// behaviour, and the GDPR retention window for completed/cancelled instances.
//
// Default values follow the RFC's §5.6 recommendations. In particular,
// `materialization_enabled` and `auto_start_planned` both default to FALSE
// so that WP-B7 is a pure no-op until the consuming services (WP-B8 / B9)
// are shipped and a tenant explicitly opts in via the settings UI.
//
// The weekday option values match ISO 8601 numbering (1 = Monday … 7 = Sunday)
// so they slot directly into time.Weekday comparisons after the usual +1 shift.
func init() {
	// --- Materialization (operations tab) ---

	timetableEnabledDependency := &config.Dependency{
		Key:       config.KeyTimetableEnabled,
		Condition: "eq",
		Value:     true,
	}

	config.Register(config.Definition{
		Key:             config.KeyTimetableEnabled,
		Label:           "Stundenplan aktivieren",
		Description:     "Aktiviert den Stundenplan in der Navigation und blendet die weiteren Stundenplan-Einstellungen ein.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       29,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationEnabled,
		Label:           "Automatische Stundenplan-Materialisierung",
		Description:     "Wenn aktiviert, erzeugt der Scheduler wöchentlich Aktivitäts-Instanzen aus den Aktivitäts-Vorlagen (Templates) für den kommenden Planungszeitraum.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       30,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationWeekday,
		Label:           "Materialisierungs-Wochentag",
		Description:     "Wochentag, an dem der Scheduler den kommenden Planungszeitraum materialisiert. Empfohlen ist ein Freitag, damit die Folgewoche ab Montag bereitsteht.",
		Type:            config.FieldSelect,
		Default:         5, // Friday (ISO 8601: Monday=1 … Sunday=7)
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       31,
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
		DependsOn: &config.Dependency{
			Key:       config.KeyTimetableMaterializationEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	minWeeksAhead := float64(1)
	maxWeeksAhead := float64(4)
	config.Register(config.Definition{
		Key:             config.KeyTimetableMaterializationWeeksAhead,
		Label:           "Vorlauf (Wochen)",
		Description:     "Anzahl der Wochen, die im Voraus materialisiert werden. 1 Woche ist Standard; größere Werte erzeugen mehr Daten auf einmal und erhöhen das Rollback-Risiko bei Template-Änderungen.",
		Type:            config.FieldNumber,
		Default:         1,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       32,
		Validation:      &config.ValidationRules{Min: &minWeeksAhead, Max: &maxWeeksAhead},
		DependsOn: &config.Dependency{
			Key:       config.KeyTimetableMaterializationEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Auto-start & staff UX (operations tab) ---

	config.Register(config.Definition{
		Key:             config.KeyTimetableAutoStartPlanned,
		Label:           "Automatischer Start geplanter Aktivitäten",
		Description:     "Wenn aktiviert, werden geplante Instanzen automatisch in eine aktive Sitzung überführt, sobald deren Startzeit erreicht ist (RFC E19 Level 3). Ohne Aktivierung zeigt die Staff-Oberfläche lediglich passive Hinweise an.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       33,
		DependsOn:       timetableEnabledDependency,
	})

	minOverdue := float64(1)
	maxOverdue := float64(30)
	config.Register(config.Definition{
		Key:             config.KeyTimetableOverdueThresholdMinutes,
		Label:           "Überfälligkeits-Schwelle (Minuten)",
		Description:     "Minuten nach dem geplanten Start, ab denen eine Instanz in der Staff-Ansicht als überfällig markiert wird.",
		Type:            config.FieldNumber,
		Default:         5,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       34,
		Validation:      &config.ValidationRules{Min: &minOverdue, Max: &maxOverdue},
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableShowExpectedChildrenCount,
		Label:           "Erwartete Kinderzahl in Instanzansicht anzeigen",
		Description:     "Zeigt in der Staff-Instanzansicht die erwartete Anzahl Kinder (auf Basis der Einschreibungen) neben der tatsächlich anwesenden Anzahl an.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       35,
		DependsOn:       timetableEnabledDependency,
	})

	// --- GDPR retention (gdpr tab) ---

	// Timetable retention is an independent window from KeyDataCleanupEnabled:
	// activity_instances rows may age out on a different cadence than the live
	// attendance data cleaned up by the scheduler job. Gate visibility on the
	// existing data-cleanup toggle so the GDPR UI surfaces both settings as a
	// coherent unit when cleanup is enabled.
	minRetention := float64(30)
	maxRetention := float64(1825) // 5 years
	config.Register(config.Definition{
		Key:             config.KeyGDPRTimetableRetentionDays,
		Label:           "Aufbewahrungsdauer Stundenplan (Tage)",
		Description:     "Anzahl Tage, für die abgeschlossene oder abgesagte Aktivitäts-Instanzen vorgehalten werden, bevor sie bereinigt werden. Trennt sich bewusst von der Bereinigung der Anwesenheitsdaten.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "stundenplan",
		SortOrder:       30,
		Validation:      &config.ValidationRules{Min: &minRetention, Max: &maxRetention},
		DependsOn: &config.Dependency{
			Key:       config.KeyTimetableEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Display range (operations tab) ---

	// The admin weekly calendar (Apple-style grid) renders hour rows between
	// these two HH:MM times by default. Events outside the window are still
	// rendered and become reachable via scroll. Defaults match the typical
	// OGS day (09:00 Schulende → 17:00 Abholung).
	config.Register(config.Definition{
		Key:             config.KeyTimetableDayStartTime,
		Label:           "Tagesansicht Beginn",
		Description:     "Standardmäßig sichtbarer Tagesbeginn in der Wochenansicht des Stundenplans. Termine vor dieser Uhrzeit bleiben sichtbar (per Scroll erreichbar).",
		Type:            config.FieldTime,
		Default:         "09:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       40,
		DependsOn:       timetableEnabledDependency,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimetableDayEndTime,
		Label:           "Tagesansicht Ende",
		Description:     "Standardmäßig sichtbares Tagesende in der Wochenansicht des Stundenplans. Termine nach dieser Uhrzeit bleiben sichtbar (per Scroll erreichbar).",
		Type:            config.FieldTime,
		Default:         "17:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "stundenplan",
		SortOrder:       41,
		DependsOn:       timetableEnabledDependency,
	})
}
