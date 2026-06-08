package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Data cleanup (system tab — automated background process) ---

	config.Register(config.Definition{
		Key:             config.KeyDataCleanupEnabled,
		Label:           "Automatische Datenbereinigung",
		Description:     "Automatische Löschung abgelaufener Besuchsdaten gemäß Datenschutzeinstellungen",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "datenbereinigung",
		SortOrder:       20,
	})

	config.Register(config.Definition{
		Key:             config.KeyDataCleanupTime,
		Label:           "Bereinigungszeitpunkt",
		Description:     "Uhrzeit für die tägliche Datenbereinigung",
		Type:            config.FieldTime,
		Default:         "02:00",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "datenbereinigung",
		SortOrder:       21,
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	minTimeout := float64(5)
	maxTimeout := float64(120)
	config.Register(config.Definition{
		Key:             config.KeyDataCleanupTimeoutMinutes,
		Label:           "Bereinigung Timeout (Minuten)",
		Description:     "Maximale Dauer für den Bereinigungsvorgang",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "datenbereinigung",
		SortOrder:       22,
		Validation:      &config.ValidationRules{Min: &minTimeout, Max: &maxTimeout},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Anwesenheitsprotokoll / Raumverlauf (per-student attendance history) ---

	config.Register(config.Definition{
		Key:             config.KeyAttendanceLogEnabled,
		Label:           "Anwesenheitsprotokoll aktivieren",
		Description:     "Ermöglicht Mitarbeitenden den Zugriff auf das Anwesenheitsprotokoll und den Raumverlauf einzelner Schüler. Aus Datenschutzgründen standardmäßig deaktiviert.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       10,
	})

	minDays := float64(1)
	maxDays := float64(365)
	config.Register(config.Definition{
		Key:             config.KeyAttendanceVisibleDays,
		Label:           "Sichtbarkeitsdauer Anwesenheit (Tage)",
		Description:     "Maximaler Zeitraum, für den Anwesenheitsdaten (An- und Abmeldezeiten) eines Schülers angezeigt werden dürfen.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       11,
		Validation:      &config.ValidationRules{Min: &minDays, Max: &maxDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceLogEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyRoomDetailVisibleDays,
		Label:           "Sichtbarkeitsdauer Raumdetails (Tage)",
		Description:     "Maximaler Zeitraum, für den zusätzlich zur Anwesenheit auch die besuchten Räume eines Schülers angezeigt werden dürfen. Sollte kleiner oder gleich der Sichtbarkeitsdauer Anwesenheit sein.",
		Type:            config.FieldNumber,
		Default:         7,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       12,
		Validation:      &config.ValidationRules{Min: &minDays, Max: &maxDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceLogEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyAttendanceLogScope,
		Label:           "Zugriffsumfang",
		Description:     "Legt fest, welche Mitarbeitenden das Anwesenheitsprotokoll eines Schülers einsehen dürfen.",
		Type:            config.FieldSelect,
		Default:         config.AttendanceLogScopeGroupSupervisorsOnly,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       13,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nur Gruppenbetreuer des Schülers", Value: config.AttendanceLogScopeGroupSupervisorsOnly},
				{Label: "Alle berechtigten Mitarbeitenden", Value: config.AttendanceLogScopeAllStaff},
			},
		},
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceLogEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Zeiterfassung Aufbewahrung (ArbZG + DSGVO retention window) ---
	//
	// §16 Abs. 2 ArbZG mandates at least 2 years for working-time records.
	// §41 EStG (Lohnkonto-Belege) requires 6 years once the data feeds
	// payroll. §147 AO / §257 HGB cap the legally defensible window at
	// 8 years. Persisted as days (730/1095/.../2920) so the cleanup
	// service stays day-precise, but exposed as a year-dropdown in the
	// admin UI — the original number input made it tempting to type
	// arbitrary day counts that don't map to legal milestones.
	config.Register(config.Definition{
		Key:             config.KeyGDPRTimeTrackingRetentionDays,
		Label:           "Aufbewahrungsdauer Zeiterfassung",
		Description:     "Wie lange Arbeitszeit-Daten (Stempelzeiten, Pausen, Korrekturen, Abwesenheiten) aufbewahrt werden, bevor sie automatisch gelöscht werden. Mindestens 2 Jahre.",
		Type:            config.FieldSelect,
		Default:         730,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "zeiterfassung",
		SortOrder:       40,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "2 Jahre", Value: 730},
				{Label: "3 Jahre", Value: 1095},
				{Label: "4 Jahre", Value: 1460},
				{Label: "5 Jahre", Value: 1825},
				{Label: "6 Jahre", Value: 2190},
				{Label: "7 Jahre", Value: 2555},
				{Label: "8 Jahre", Value: 2920},
			},
		},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Schülerdaten-Zugriff (who can read full student profile data) ---

	config.Register(config.Definition{
		Key:             config.KeyStudentDataScope,
		Label:           "Schülerdaten-Zugriff",
		Description:     "Legt fest, welche Mitarbeitenden vollständige Schülerdaten (Profil, aktueller Aufenthaltsort, Besuchsinformationen, Datenschutzangaben und Abholpläne) einsehen dürfen. Schreiboperationen bleiben stets auf die Gruppenbetreuer des Schülers beschränkt.",
		Type:            config.FieldSelect,
		Default:         config.StudentDataScopeGroupSupervisorsOnly,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       20,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nur Gruppenbetreuer des Schülers", Value: config.StudentDataScopeGroupSupervisorsOnly},
				{Label: "Alle berechtigten Mitarbeitenden", Value: config.StudentDataScopeAllStaff},
			},
		},
	})
}
