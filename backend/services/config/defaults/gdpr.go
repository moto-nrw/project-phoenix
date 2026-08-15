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
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	})

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
		Validation:      config.Range(5, 120),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	})

	// --- Anwesenheitsprotokoll / Raumverlauf (per-student attendance history) ---

	config.Register(config.Definition{
		Key:             config.KeyAttendanceLogEnabled,
		Label:           "Anwesenheitsprotokoll aktivieren",
		Description:     "Ermöglicht Mitarbeitenden den Zugriff auf das Anwesenheitsprotokoll und den Raumverlauf einzelner Kinder. Aus Datenschutzgründen standardmäßig deaktiviert.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       10,
	})

	config.Register(config.Definition{
		Key:             config.KeyAttendanceVisibleDays,
		Label:           "Sichtbarkeitsdauer Anwesenheit (Tage)",
		Description:     "Maximaler Zeitraum, für den Anwesenheitsdaten (An- und Abmeldezeiten) eines Kindes angezeigt werden dürfen.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       11,
		Validation:      config.Range(1, 365),
		DependsOn:       config.DependsOnEq(config.KeyAttendanceLogEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyRoomDetailVisibleDays,
		Label:           "Sichtbarkeitsdauer Raumdetails (Tage)",
		Description:     "Maximaler Zeitraum, für den zusätzlich zur Anwesenheit auch die besuchten Räume eines Kindes angezeigt werden dürfen. Sollte kleiner oder gleich der Sichtbarkeitsdauer Anwesenheit sein.",
		Type:            config.FieldNumber,
		Default:         7,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       12,
		Validation:      config.Range(1, 365),
		DependsOn:       config.DependsOnEq(config.KeyAttendanceLogEnabled, true),
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
		DependsOn: config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	})

	// --- Datenschutz-Einwilligung: Aufbewahrung der Besuchsdaten ---
	//
	// Standard-Aufbewahrungsfenster (Tage) für Besuchsdaten, wenn die
	// Einwilligung eines Kindes keinen eigenen Wert hinterlegt hat.
	// Issue #586 (Rule 12): Der frühere fest verdrahtete 30-Tage-Standard
	// samt 1..31-Grenzen wurde aus dem PrivacyConsent-Modell in diese
	// per-Schule konfigurierbare Einstellung verschoben.
	config.Register(config.Definition{
		Key:             config.KeyPrivacyConsentRetentionDays,
		Label:           "Standard-Aufbewahrungsdauer Besuchsdaten (Tage)",
		Description:     "Wie lange Besuchsdaten eines Kindes aufbewahrt werden, wenn dessen Datenschutz-Einwilligung keinen eigenen Wert festlegt. Wird beim automatischen Löschen abgelaufener Besuchsdaten angewendet.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       14,
		Validation:      config.Range(1, 31),
	})

	// --- Änderungsprotokoll pro Kind: Aufbewahrung (issue #1455) ---
	//
	// Das Änderungsprotokoll (wer hat welche Angabe zu einem Kind geändert)
	// dient konkreten Rückfragen im OGS-Alltag, nicht der Langzeit-Archivierung
	// oder Leistungskontrolle. Entsprechend kurz und per Schule konfigurierbar.
	// Standard 90 Tage; das automatische Löschen läuft über denselben
	// nächtlichen Cleanup-Job (gdpr.data_cleanup_enabled / _time).
	minChangeLogRetentionDays := float64(30)
	maxChangeLogRetentionDays := float64(365)
	config.Register(config.Definition{
		Key:             config.KeyGDPRStudentChangeLogRetentionDays,
		Label:           "Aufbewahrungsdauer Änderungsprotokoll (Tage)",
		Description:     "Wie lange das Änderungsprotokoll pro Kind (wer hat welche Angabe geändert) aufbewahrt wird, bevor alte Einträge automatisch gelöscht werden.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       30,
		Validation:      &config.ValidationRules{Min: &minChangeLogRetentionDays, Max: &maxChangeLogRetentionDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

}
