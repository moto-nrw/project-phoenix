package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func init() {

	registerDefinitions(gdpr_init_datenbereinigung_0Definitions, gdpr_init_bewegungsdaten_1Definitions, gdpr_init_zeiterfassung_2Definitions, gdpr_init_bewegungsdaten_3Definitions, gdpr_init_sch_lerdaten_4Definitions, gdpr_init_bewegungsdaten_5Definitions, gdpr_init_personaldokumente_6Definitions)

}

// --- Data cleanup (system tab — automated background process) ---
// --- Anwesenheitsprotokoll / Raumverlauf (per-student attendance history) ---
// Daily attendance (#3957), counted from the care day. No statute sets a
// period for OGS attendance; a year covers the school year plus its
// follow-up questions. The frame of 30 days to 3 years is ADR 0048's.
// The settings service keeps gdpr.attendance_visible_days at or below it.
// --- Zeiterfassung Aufbewahrung (ArbZG + DSGVO retention window) ---
//
// §16 Abs. 2 ArbZG mandates at least 2 years for working-time records.
// §41 EStG (Lohnkonto-Belege) requires 6 years once the data feeds
// payroll. §147 AO / §257 HGB cap the legally defensible window at
// 8 years. Persisted as days (730/1095/.../2920) so the cleanup
// service stays day-precise, but exposed as a year-dropdown in the
// admin UI — the original number input made it tempting to type
// arbitrary day counts that don't map to legal milestones.
// --- Datenschutz-Einwilligung: Aufbewahrung der Besuchsdaten ---
//
// Aufbewahrungsfenster (Tage) für Besuchsdaten von Kindern ohne erteilte
// Einwilligung; eine erteilte Einwilligung bringt ihr eigenes Fenster mit.
// Der nächtliche Löschlauf liest diesen Wert (#3835).
// Issue #586 (Rule 12): Der frühere fest verdrahtete 30-Tage-Standard
// samt 1..31-Grenzen wurde aus dem PrivacyConsent-Modell in diese
// per-Schule konfigurierbare Einstellung verschoben.
// --- Änderungsprotokoll pro Kind: Aufbewahrung (issue #1455) ---
//
// Das Änderungsprotokoll (wer hat welche Angabe zu einem Kind geändert)
// dient konkreten Rückfragen im OGS-Alltag, nicht der Langzeit-Archivierung
// oder Leistungskontrolle. Entsprechend kurz und per Schule konfigurierbar.
// Standard 90 Tage; das automatische Löschen läuft über denselben
// nächtlichen Cleanup-Job (gdpr.data_cleanup_enabled / _time).
// Sick, excused and class-trip days (#3958). Sickness is health data
// (Art. 9 DSGVO); no statute sets a period for OGS attendance, so a year
// covers the school year plus its follow-up questions. The frame of
// 30 days to 3 years is ADR 0048's.
// Withdrawn and lapsed enrollments no child was created from (#3960),
// counted from settling. They have no further purpose; 90 days cover the
// questions after a withdrawal. The frame of 30 to 365 days is ADR 0048's.
// Rejected children keep enrollment.rejected_retention_days.
// Children whose care ended (#3962), counted from the last care day. A
// year covers questions after the departure (billing, a sibling's
// enrollment, a return). The frame of 30 days to 3 years is ADR 0048's.
// The deletion takes the child with every related record, the accepted
// enrollment included.
// Retention for the OGS-internal colleague chat (#2598). Staff messages are
// employee personal data, so the window ships with the feature rather than
// being added later. The floor of 30 days keeps a conversation usable across
// a holiday period; the ceiling of 2 years is the defensible upper bound for
// day-to-day operational chatter.
// Parent/OGS conversations (#3959). The frame of 30 days to 2 years
// matches the staff chat; open requests stay until they are decided.
// Staff sick notes (AU-Bescheinigungen, #3961), counted from the upload.
// They are health data (Art. 9 DSGVO); moto is no legal archive (ADR 0048).
// A year covers the twelve-month window of § 3 Abs. 1 S. 2 Nr. 2 EFZG for
// a repeated incapacity from the same illness. The frame of 90 days to
// 3 years is ADR 0048's. Other staff documents stay until the staff member
// leaves.

var gdpr_init_datenbereinigung_0Definitions = []config.Definition{
	{
		Key:             config.KeyDataCleanupEnabled,
		Label:           "Alte Daten automatisch löschen",
		Description:     "Notbremse: Hält die nächtliche Löschung dieser Schule an. Nur bei einem Vorfall ausschalten. Solange sie aus ist, hält moto die Löschfristen aus dem Vertrag nicht ein.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		// ADR 0048: the school cannot switch the nightly deletion off; only
		// the moto team halts it during an incident.
		AccessPolicy: config.AccessOperatorOnly,
		Tab:          "gdpr",
		Category:     "datenbereinigung",
		SortOrder:    1,
	},
	{
		Key:             config.KeyDataCleanupTime,
		Label:           "Uhrzeit für das Löschen",
		Description:     "Zu dieser Uhrzeit löscht moto jeden Tag die alten Daten.",
		Type:            config.FieldTime,
		Default:         "02:00",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "datenbereinigung",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
	{
		Key:             config.KeyDataCleanupTimeoutMinutes,
		Label:           "Bereinigung Timeout (Minuten)",
		Description:     "Maximale Dauer für den Bereinigungsvorgang",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "datenbereinigung",
		SortOrder:       3,
		Validation:      config.Range(5, 120),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
		AccessPolicy:    config.AccessOperatorOnly,
	},
}

var gdpr_init_bewegungsdaten_1Definitions = []config.Definition{
	{
		Key:             config.KeyAttendanceLogEnabled,
		Label:           "Anwesenheitsprotokoll",
		Description:     "Das Team sieht bei jedem Kind, wann es da war und in welchen Räumen. Aus Datenschutzgründen ist das zunächst ausgeschaltet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       10,
	},
	{
		Key:             config.KeyAttendanceVisibleDays,
		Label:           "Anwesenheit sichtbar für (Tage)",
		Description:     "So viele Tage zurück sieht das Team, wann ein Kind kam und ging. Höchstens so lange, wie moto die Anwesenheit aufbewahrt.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       11,
		Validation:      config.Range(1, 365),
		DependsOn:       config.DependsOnEq(config.KeyAttendanceLogEnabled, true),
	},
	{
		Key:             config.KeyGDPRAttendanceRetentionDays,
		Label:           "Anwesenheit aufbewahren (Tage)",
		Description:     "So lange speichert moto, wann ein Kind kam und ging. Danach löscht moto diese Angaben.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       13,
		Validation:      config.Range(30, 1095),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
	{
		Key:             config.KeyRoomDetailVisibleDays,
		Label:           "Räume sichtbar für (Tage)",
		Description:     "So viele Tage zurück sieht das Team, in welchen Räumen ein Kind war. Der Wert sollte nicht größer sein als bei der Anwesenheit.",
		Type:            config.FieldNumber,
		Default:         7,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       12,
		Validation:      config.Range(1, 365),
		DependsOn:       config.DependsOnEq(config.KeyAttendanceLogEnabled, true),
	},
}

var gdpr_init_zeiterfassung_2Definitions = []config.Definition{
	{
		Key:             config.KeyGDPRTimeTrackingRetentionDays,
		Label:           "Zeiterfassung aufbewahren",
		Description:     "So lange speichert moto Stempelzeiten, Pausen, Korrekturen und Abwesenheiten. Danach löscht moto sie. Bei ausgetretenem Personal löscht moto die Angaben nach derselben Frist ab dem Austritt. Mindestens 2 Jahre.",
		Type:            config.FieldSelect,
		Default:         730,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
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
	},
}

var gdpr_init_bewegungsdaten_3Definitions = []config.Definition{
	{
		Key:             config.KeyPrivacyConsentRetentionDays,
		Label:           "Besuchsdaten aufbewahren (Tage)",
		Description:     "So lange speichert moto, wann ein Kind in welchem Raum war. Gilt für Kinder ohne Datenschutz-Einwilligung. Danach löscht moto die Daten.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       14,
		Validation:      config.Range(1, 31),
	},
}

var gdpr_init_sch_lerdaten_4Definitions = []config.Definition{
	{
		Key:             config.KeyGDPRStudentChangeLogRetentionDays,
		Label:           "Änderungsprotokoll aufbewahren (Tage)",
		Description:     "So lange speichert moto, wer welche Angabe bei einem Kind geändert hat. Danach löscht moto die alten Einträge.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       30,
		Validation:      &config.ValidationRules{Min: &minChangeLogRetentionDays, Max: &maxChangeLogRetentionDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	},
	{
		Key:             config.KeyGDPRStatusDayRetentionDays,
		Label:           "Krank- und Entschuldigt-Tage aufbewahren (Tage)",
		Description:     "So lange speichert moto, an welchen Tagen ein Kind krank, entschuldigt oder auf Klassenfahrt war. Danach löscht moto diese Tage.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       31,
		Validation:      config.Range(30, 1095),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
	{
		Key:             config.KeyGDPRWithdrawnEnrollmentRetentionDays,
		Label:           "Zurückgezogene Anmeldungen aufbewahren (Tage)",
		Description:     "So lange speichert moto Anmeldungen, die zurückgezogen oder verfallen sind. Danach löscht moto sie. Das gilt nur, wenn daraus kein Kind angelegt wurde.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       32,
		Validation:      config.Range(30, 365),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
	{
		Key:             config.KeyGDPRStudentRetentionDays,
		Label:           "Kinder nach dem letzten Betreuungstag aufbewahren (Tage)",
		Description:     "So lange speichert moto ein Kind nach seinem letzten Betreuungstag. Danach löscht moto das Kind mit allen Angaben. Kinder, die wieder betreut werden, bleiben. Jahrgangswechsel löscht moto nach derselben Zeit.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       33,
		Validation:      config.Range(30, 1095),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
	{
		Key:             config.KeyGDPRGuardianRetentionDays,
		Label:           "Erziehungsberechtigte ohne Kind aufbewahren (Tage)",
		Description:     "So lange speichert moto Erziehungsberechtigte, die kein Kind mehr an der Schule haben. Danach löscht moto sie mit allen Angaben.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "schülerdaten",
		SortOrder:       34,
		Validation:      config.Range(30, 365),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
}

var gdpr_init_bewegungsdaten_5Definitions = []config.Definition{
	{
		Key:             config.KeyGDPRPWAUsageRetentionDays,
		Label:           "App-Nutzung aufbewahren (Tage)",
		Description:     "So lange speichert moto, wer moto zuletzt über das Symbol auf dem Startbildschirm geöffnet hat. Danach löscht moto die Einträge.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       15,
		Validation:      &config.ValidationRules{Min: &minPWAUsageRetentionDays, Max: &maxPWAUsageRetentionDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	},
	{
		Key:             config.KeyGDPRStaffMessageRetentionDays,
		Label:           "Team-Chat und Tagesinformationen aufbewahren (Tage)",
		Description:     "Wie lange Nachrichten im internen Team-Chat gespeichert bleiben. Ältere Nachrichten werden automatisch gelöscht. Unterhaltungen ohne Nachrichten verschwinden danach aus der Übersicht. Beendete Tagesinformationen löscht moto nach derselben Zeit.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       16,
		Validation:      &config.ValidationRules{Min: &minStaffMessageRetentionDays, Max: &maxStaffMessageRetentionDays},
		DependsOn: &config.Dependency{
			Key:       config.KeyDataCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
	},
	{
		Key:             config.KeyGDPRParentMessageRetentionDays,
		Label:           "Elternnachrichten und Elternbriefe aufbewahren (Tage)",
		Description:     "So lange speichert moto die Nachrichten zwischen Eltern und OGS. Danach löscht moto sie. Offene Anfragen bleiben, bis die OGS sie bearbeitet hat. Abgelaufene Elternbriefe löscht moto nach derselben Zeit. Ohne Ablauf zählt die Veröffentlichung. Beantwortete Einverständnisse bleiben, solange das Kind gespeichert ist.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "bewegungsdaten",
		SortOrder:       17,
		Validation:      config.Range(30, 730),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
}

var gdpr_init_personaldokumente_6Definitions = []config.Definition{
	{
		Key:             config.KeyGDPRStaffSickNoteRetentionDays,
		Label:           "AU-Bescheinigungen aufbewahren (Tage)",
		Description:     "So lange speichert moto AU-Bescheinigungen ab dem Hochladen. Danach löscht moto sie samt Datei. Andere Dokumente einer Person bleiben, bis sie ausscheidet.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "gdpr",
		Category:        "personaldokumente",
		SortOrder:       45,
		Validation:      config.Range(90, 1095),
		DependsOn:       config.DependsOnEq(config.KeyDataCleanupEnabled, true),
	},
}

var minChangeLogRetentionDays = float64(30)
var maxChangeLogRetentionDays = float64(365)
var minPWAUsageRetentionDays = float64(30)
var maxPWAUsageRetentionDays = float64(365)
var minStaffMessageRetentionDays = float64(30)
var maxStaffMessageRetentionDays = float64(730)
