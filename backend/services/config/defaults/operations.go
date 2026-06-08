package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	// --- Session End (system tab — automated background process) ---

	config.Register(config.Definition{
		Key:             config.KeySessionEndEnabled,
		Label:           "Automatisches Sitzungsende",
		Description:     "Alle aktiven Sitzungen werden automatisch zur konfigurierten Uhrzeit beendet",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       1,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeySessionEndTime,
		Label:           "Sitzungsende Uhrzeit",
		Description:     "Uhrzeit, zu der alle aktiven Sitzungen automatisch beendet werden",
		Type:            config.FieldTime,
		Default:         "18:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       2,
		DependsOn: &config.Dependency{
			Key:       config.KeySessionEndEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	minTimeout := float64(1)
	maxTimeout := float64(60)
	config.Register(config.Definition{
		Key:             config.KeySessionEndTimeoutMinutes,
		Label:           "Sitzungsende Timeout (Minuten)",
		Description:     "Maximale Dauer für den automatischen Sitzungsende-Vorgang",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsende",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minTimeout, Max: &maxTimeout},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionEndEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	// --- Student Daily Checkout ---

	config.Register(config.Definition{
		Key:             config.KeyStudentDailyCheckoutTime,
		Label:           "Tägliche Abmeldezeit",
		Description:     "Uhrzeit, ab der Schüler aus dem Heimraum abgemeldet werden können. Wenn leer, ist die Abmeldung jederzeit möglich.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       1,
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceNFCEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutEnabled,
		Label:           "Individuelle Abholzeiten verwenden",
		Description:     "Wenn aktiviert, wird die Abmeldung anhand der individuellen Abholzeiten der Schüler angezeigt statt der globalen Abmeldezeit.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       2,
		DependsOn: &config.Dependency{
			Key:       config.KeyAttendanceNFCEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	minDelta := float64(0)
	maxDelta := float64(120)
	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutDeltaMinutes,
		Label:           "Vorlaufzeit vor Abholung (Minuten)",
		Description:     "Minuten vor der Abholzeit, ab der die Abmeldung am Gerät angeboten wird. Beispiel: Bei Abholzeit 15:00 und Vorlaufzeit 15 Min. ist die Abmeldung ab 14:45 möglich.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       3,
		Validation:      &config.ValidationRules{Min: &minDelta, Max: &maxDelta},
		DependsOn: &config.Dependency{
			Key:       config.KeyPerStudentCheckoutEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

	// --- Abandoned Session Cleanup (system tab — automated background process) ---

	config.Register(config.Definition{
		Key:             config.KeySessionCleanupEnabled,
		Label:           "Bereinigung verlassener Sitzungen",
		Description:     "Automatische Bereinigung von Sitzungen ohne Aktivität",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	minInterval := float64(5)
	maxInterval := float64(120)
	config.Register(config.Definition{
		Key:             config.KeySessionCleanupIntervalMinutes,
		Label:           "Bereinigungsintervall (Minuten)",
		Description:     "Wie oft nach verlassenen Sitzungen geprüft wird",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       11,
		Validation:      &config.ValidationRules{Min: &minInterval, Max: &maxInterval},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	minThreshold := float64(10)
	maxThreshold := float64(480)
	config.Register(config.Definition{
		Key:             config.KeySessionAbandonedThresholdMin,
		Label:           "Inaktivitätsschwelle (Minuten)",
		Description:     "Minuten ohne Aktivität, bevor eine Sitzung als verlassen gilt",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       12,
		Validation:      &config.ValidationRules{Min: &minThreshold, Max: &maxThreshold},
		DependsOn: &config.Dependency{
			Key:       config.KeySessionCleanupEnabled,
			Condition: "eq",
			Value:     true,
		},
		AccessPolicy: config.AccessOperatorOnly,
	})

	// --- Admin Supervision Overview ---

	config.Register(config.Definition{
		Key:             config.KeyAdminSupervisionOverview,
		Label:           "Administrator-Aufsichtsübersicht",
		Description:     "Administratoren können alle aktiven Aufsichten und anwesende Kinder einsehen",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       1,
	})

	// --- Zeiterfassung ---

	config.Register(config.Definition{
		Key:             config.KeyTimeTrackingAccountStartDate,
		Label:           "Stundenkonto ab Datum berechnen",
		Description:     "Legt fest, ab welchem Datum das Stundenkonto neu berechnet wird. Wenn kein Datum gesetzt ist, startet die Berechnung am 1. Januar des aktuellen Jahres.",
		Type:            config.FieldDate,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       1,
	})

	// Break auto-end interval is NOT registered here — it controls a global ticker
	// (not per-tenant) and is configured via BREAK_AUTO_END_INTERVAL_SECONDS env var only.

	// --- Status flag auto-clear (Krank / Entschuldigt badge lifecycle) ---

	statusFlagOptions := &config.SelectOptions{
		Static: []config.SelectOption{
			{Label: "Manuell (nur durch Betreuer)", Value: config.ClearModeManual},
			{Label: "Beim nächsten Check-in", Value: config.ClearModeNextCheckin},
			{Label: "Am Ende des Tages", Value: config.ClearModeEndOfDay},
		},
	}

	config.Register(config.Definition{
		Key:             config.KeyStatusFlagClearTime,
		Label:           "Abwesenheit automatisch beenden um",
		Description:     "Uhrzeit, zu der Krankmeldungen und Entschuldigungen mit Einstellung \"Am Ende des Tages\" automatisch aufgehoben werden.",
		Type:            config.FieldTime,
		Default:         "18:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "abwesenheit",
		SortOrder:       29,
	})

	config.Register(config.Definition{
		Key:             config.KeySickClearMode,
		Label:           "Krankmeldung automatisch beenden",
		Description:     "Legt fest, wann die Krankmeldung eines Schülers automatisch aufgehoben wird.",
		Type:            config.FieldSelect,
		Default:         config.ClearModeNextCheckin,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "abwesenheit",
		SortOrder:       30,
		Options:         statusFlagOptions,
	})

	config.Register(config.Definition{
		Key:             config.KeyExcusedClearMode,
		Label:           "Entschuldigung automatisch beenden",
		Description:     "Legt fest, wann die Entschuldigung eines Schülers automatisch aufgehoben wird.",
		Type:            config.FieldSelect,
		Default:         config.ClearModeEndOfDay,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "abwesenheit",
		SortOrder:       31,
		Options:         statusFlagOptions,
	})

	// --- Anwesenheits-Modus (presence tracking model, operator-only) ---

	config.Register(config.Definition{
		Key:             config.KeyPresenceMode,
		Label:           "Anwesenheits-Modus",
		Description:     "Detailliert erfasst Räume und Aktivitäten. Binär erfasst nur, ob ein Schüler in der Schule ist (ohne Raumverfolgung).",
		Type:            config.FieldSelect,
		Default:         config.PresenceModeDetailed,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       40,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Detailliert (Räume & Aktivitäten)", Value: config.PresenceModeDetailed},
				{Label: "Binär (nur An-/Abwesend)", Value: config.PresenceModeBinary},
			},
		},
	})

	// --- Anwesenheitserfassung (setup-level decisions) ---

	config.Register(config.Definition{
		Key:             config.KeyAttendanceWebEnabled,
		Label:           "Anwesenheit über Web-App erfassen",
		Description:     "Mitarbeitende können Kinder über die Web-App an- und abmelden oder in Aktivitäten eintragen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       43,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyAttendanceNFCEnabled,
		Label:           "Anwesenheit über NFC-Geräte erfassen",
		Description:     "Die OGS nutzt NFC-Armbänder oder Karten an Geräten, zum Beispiel für Räume, Schulhof oder Abmeldung.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       44,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	// --- Organisationsmodell (setup-level decisions) ---

	config.Register(config.Definition{
		Key:             config.KeyGroupMode,
		Label:           "Arbeit mit festen Gruppen",
		Description:     "Legt fest, ob Kinder im Alltag festen OGS-Gruppen zugeordnet sind oder ob alle berechtigten Mitarbeitenden mit allen Kindern arbeiten.",
		Type:            config.FieldSelect,
		Default:         config.GroupModeFixedGroups,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       1,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Feste Gruppen", Value: config.GroupModeFixedGroups},
				{Label: "Offene Betreuung ohne feste Gruppen", Value: config.GroupModeOpenCare},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyCareConcept,
		Label:           "Betreuungskonzept",
		Description:     "Legt fest, ob die OGS mit einem festen Betriebsplan arbeitet oder Kinder sich frei zwischen offenen Räumen bewegen.",
		Type:            config.FieldSelect,
		Default:         config.CareConceptOpenRooms,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       2,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Fester Betriebsplan", Value: config.CareConceptFixedSchedule},
				{Label: "Offenes Raumkonzept", Value: config.CareConceptOpenRooms},
			},
		},
	})

	// --- Student Activation Scheduler (parent-enrollment lifecycle) ---
	//
	// Controls how often the activate-students tick re-evaluates pending and
	// active students against their enrolled_from / enrolled_until dates.
	// Date transitions only happen on day boundaries — the interval is a
	// safety-net for restarts and clock drift, not a precision dial.

	minActivationInterval := float64(5)
	maxActivationInterval := float64(1440)
	config.Register(config.Definition{
		Key:             config.KeyStudentActivationIntervalMin,
		Label:           "Schüleraktivierung Intervall (Minuten)",
		Description:     "Wie oft geprüft wird, ob Schüler mit Status \"ausstehend\" oder mit Abmeldedatum in der Vergangenheit ihren Status wechseln müssen.",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "schüleraktivierung",
		SortOrder:       50,
		Validation:      &config.ValidationRules{Min: &minActivationInterval, Max: &maxActivationInterval},
	})

	// --- Web-An/Abmeldung Zugriff (who can toggle presence via web UI) ---

	config.Register(config.Definition{
		Key:             config.KeyWebCheckinAccess,
		Label:           "Web-An/Abmeldung Zugriff",
		Description:     "Legt fest, welche Mitarbeitenden Schüler über die Weboberfläche an- und abmelden dürfen.",
		Type:            config.FieldSelect,
		Default:         config.WebCheckinAccessGroupSupervisors,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       41,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nur Gruppenbetreuer des Schülers", Value: config.WebCheckinAccessGroupSupervisors},
				{Label: "Alle berechtigten Mitarbeitenden", Value: config.WebCheckinAccessAllStaff},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyWebSpontaneousActivities,
		Label:           "Spontane Aktivitäten über Web/App",
		Description:     "Erlaubt Mitarbeitenden, in der mobilen Weboberfläche unter aktueller Aufsicht spontane Aktivitäten zu starten. Die Aktivität belegt den Raum und wird in den Stundenplan geschrieben, auch wenn die Stundenplanplanung deaktiviert ist.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       42,
		DependsOn: &config.Dependency{
			Key:       config.KeyCareConcept,
			Condition: "eq",
			Value:     config.CareConceptOpenRooms,
		},
	})

	// --- Kinderfotos (Datenverwaltung-Erweiterung) ---

	config.Register(config.Definition{
		Key:             config.KeyStudentPhotosEnabled,
		Label:           "Kinderfotos aktivieren",
		Description:     "Wenn aktiviert, können Mitarbeitende mit Bearbeitungsrecht in der Datenverwaltung Fotos zu Kindern hinterlegen (nur mit dokumentierter Einwilligung der Eltern). Fotos erscheinen anschließend in Suche, Räumen, Abholplan und Kinderdetail.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		// Category key is shown verbatim as the section header in the
		// settings UI (schema_builder.Label = catName). German umlaut here
		// matches the visible "Kinder" copy throughout the photo feature.
		Category:  "kinder",
		SortOrder: 50,
	})

	// --- Elternportal (parents-portal write features) ---
	//
	// These gate what guardians may submit through the parents app. Both
	// default ON (opt-out): schools with the portal get them immediately
	// and can disable per school. The parent endpoints resolve the value
	// for the child's tenant before accepting a write.

	config.Register(config.Definition{
		Key:             config.KeyParentSickNoteEnabled,
		Label:           "Krankmeldung über Elternportal",
		Description:     "Wenn aktiviert, können Eltern ihr Kind über das Elternportal für einen oder mehrere Tage krankmelden. Die Krankmeldung erscheint wie eine vom Team eingetragene Abwesenheit.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       60,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentNotesEnabled,
		Label:           "Elternnachrichten über Elternportal",
		Description:     "Wenn aktiviert, können Eltern dem Team über das Elternportal kurze Nachrichten zu ihrem Kind hinterlassen. Die neuesten Nachrichten erscheinen in der Kinderdetailansicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       61,
	})
}
