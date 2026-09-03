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
		DependsOn:       config.DependsOnEq(config.KeySessionEndEnabled, true),
		AccessPolicy:    config.AccessOperatorOnly,
	})

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
		Validation:      config.Range(1, 60),
		DependsOn:       config.DependsOnEq(config.KeySessionEndEnabled, true),
		AccessPolicy:    config.AccessOperatorOnly,
	})

	// --- Student Daily Checkout ---

	config.Register(config.Definition{
		Key:             config.KeyStudentDailyCheckoutTime,
		Label:           "Tägliche Abmeldezeit",
		Description:     "Uhrzeit, ab der Kinder aus dem Heimraum abgemeldet werden können. Wenn leer, ist die Abmeldung jederzeit möglich.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       1,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutEnabled,
		Label:           "Individuelle Abholzeiten verwenden",
		Description:     "Wenn aktiviert, wird die Abmeldung anhand der individuellen Abholzeiten der Kinder angezeigt statt der globalen Abmeldezeit.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "checkout",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

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
		Validation:      config.Range(0, 120),
		DependsOn:       config.DependsOnEq(config.KeyPerStudentCheckoutEnabled, true),
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
		Validation:      config.Range(5, 120),
		DependsOn:       config.DependsOnEq(config.KeySessionCleanupEnabled, true),
		AccessPolicy:    config.AccessOperatorOnly,
	})

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
		Validation:      config.Range(10, 480),
		DependsOn:       config.DependsOnEq(config.KeySessionCleanupEnabled, true),
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeySessionInactivityTimeoutMin,
		Label:           "Standard-Sitzungstimeout (Minuten)",
		Description:     "Standardzeit ohne Aktivität, nach der eine Sitzung automatisch beendet wird, sofern für die Sitzung kein eigenes Timeout gesetzt ist",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "system",
		Category:        "sitzungsbereinigung",
		SortOrder:       13,
		Validation:      config.Range(1, 480),
		AccessPolicy:    config.AccessOperatorOnly,
	})

	// --- Sichtbereich für Gruppen und laufende Betreuungen (#2801) ---

	config.Register(config.Definition{
		Key:             config.KeyOperationalOverviewScope,
		Label:           "Sichtbereich für Mitarbeitende",
		Description:     "Legt fest, welche Gruppen und laufenden Betreuungen Mitarbeitende sehen. Admins sehen immer alles. Die Auswahl gibt keine neuen Rechte.",
		Type:            config.FieldSelect,
		Default:         config.OverviewScopeAllStaff,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       1,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Ganzes Team", Value: config.OverviewScopeAllStaff},
				{Label: "Eigene Zuständigkeiten", Value: config.OverviewScopeOwn},
			},
		},
	})

	// --- Abweichende Ankunftszeit für eine Klasse (#2962) ---

	config.Register(config.Definition{
		Key:             config.KeyClassArrivalExceptionEditors,
		Label:           "Andere Ankunftszeit für eine Klasse eintragen",
		Description:     "Legt fest, wer für eine ganze Klasse an einem Tag eine andere Ankunftszeit eintragen darf, zum Beispiel bei Unterrichtsausfall. Sehen können die Änderung alle.",
		Type:            config.FieldSelect,
		Default:         config.ClassArrivalExceptionEditorsAdmins,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       2,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nur Koordination und Admins", Value: config.ClassArrivalExceptionEditorsAdmins},
				{Label: "Alle Mitarbeitenden", Value: config.ClassArrivalExceptionEditorsAllStaff},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeySchoolPortalWriteScope,
		Label:           "Was Lehrkräfte in moto schule eintragen dürfen",
		Description:     "Gilt für Lehrkräfte mit Zugang zu moto schule. Zurzeit geht nur eines: eine andere Ankunftszeit für eine ganze Klasse an einem Tag, zum Beispiel bei Unterrichtsausfall. Die OGS sieht die Eintragung sofort überall dort, wo Ankunftszeiten stehen.",
		Type:            config.FieldSelect,
		Default:         config.SchoolPortalWriteScopeNone,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       3,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nichts. Die Schule sieht nur.", Value: config.SchoolPortalWriteScopeNone},
				{Label: "Andere Ankunftszeit für eine Klasse an einem Tag", Value: config.SchoolPortalWriteScopeClassArrivalExceptions},
			},
		},
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

	config.Register(config.Definition{
		Key:             config.KeyTimeTrackingEnforcePlannedStart,
		Label:           "Einstempeln erst ab geplanter Startzeit",
		Description:     "Wenn aktiviert, können Mitarbeitende erst ab der Startzeit einstempeln, die im Arbeitszeitmodell für den jeweiligen Tag hinterlegt ist. Tage ohne Startzeit bleiben unverändert.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       2,
	})

	config.Register(config.Definition{
		Key:         config.KeyTimeTrackingRequireDeviationReason,
		Label:       "Begründung bei Abweichung vom Dienstplan",
		Description: "Wenn aktiviert, verlangt das Ein- und Ausstempeln außerhalb des Toleranzfensters um die geplante Schichtzeit eine Begründung. Gilt auch für nachträgliche Änderungen eigener Zeiten. Tage ohne geplante Schicht bleiben unverändert.",
		Type:        config.FieldBoolean,
		// Default on (#1844): Planabweichungen sollen standardmäßig begründet
		// werden, damit spätere Zeiten (später Bus, längerer Einsatz) im Audit-Log
		// nachvollziehbar sind. Schulen können es pro Mandant abschalten.
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       3,
	})

	config.Register(config.Definition{
		Key:             config.KeyTimeTrackingDeviationToleranceMinutes,
		Label:           "Toleranzfenster für Abweichungen (Minuten)",
		Description:     "So viele Minuten darf die Ist-Zeit von der geplanten Schichtzeit abweichen, bevor eine Begründung verlangt wird.",
		Type:            config.FieldNumber,
		Default:         15,
		Validation:      config.Range(0, 120),
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       4,
		DependsOn:       config.DependsOnEq(config.KeyTimeTrackingRequireDeviationReason, true),
	})

	// Break auto-end interval is NOT registered here. It controls a global ticker
	// and is configured via BREAK_AUTO_END_INTERVAL_SECONDS env var only.

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
		Description:     "Legt fest, wann die Krankmeldung eines Kindes automatisch aufgehoben wird.",
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
		Description:     "Legt fest, wann die Entschuldigung eines Kindes automatisch aufgehoben wird.",
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
		Description:     "Detailliert erfasst Räume und Aktivitäten. Binär erfasst nur, ob ein Kind in der Schule ist (ohne Raumverfolgung).",
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

	// --- Bundesland (public-holiday calendar, operator-only, #1418 3a) ---

	config.Register(config.Definition{
		Key:             config.KeyFederalState,
		Label:           "Bundesland",
		Description:     "Bundesland des Standorts. Bestimmt die gesetzlichen Feiertage in der Zeiterfassung (Soll = 0 an Feiertagen).",
		Type:            config.FieldSelect,
		Default:         "DE-NW",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "standort",
		SortOrder:       45,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Baden-Württemberg", Value: "DE-BW"},
				{Label: "Bayern", Value: "DE-BY"},
				{Label: "Berlin", Value: "DE-BE"},
				{Label: "Brandenburg", Value: "DE-BB"},
				{Label: "Bremen", Value: "DE-HB"},
				{Label: "Hamburg", Value: "DE-HH"},
				{Label: "Hessen", Value: "DE-HE"},
				{Label: "Mecklenburg-Vorpommern", Value: "DE-MV"},
				{Label: "Niedersachsen", Value: "DE-NI"},
				{Label: "Nordrhein-Westfalen", Value: "DE-NW"},
				{Label: "Rheinland-Pfalz", Value: "DE-RP"},
				{Label: "Saarland", Value: "DE-SL"},
				{Label: "Sachsen", Value: "DE-SN"},
				{Label: "Sachsen-Anhalt", Value: "DE-ST"},
				{Label: "Schleswig-Holstein", Value: "DE-SH"},
				{Label: "Thüringen", Value: "DE-TH"},
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
		Key:   config.KeyGroupMode,
		Label: "Arbeit mit festen Gruppen",
		Description: "Legt fest, ob Kinder im Alltag festen OGS-Gruppen zugeordnet sind oder ob alle berechtigten Mitarbeitenden mit allen Kindern arbeiten. " +
			"Diese Einstellung beschreibt nur die Organisation. Wer welche Räume in der Aktuellen Aufsicht sieht, steht unter \"Sicht auf alle Räume\".",
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

	config.Register(config.Definition{
		Key:             config.KeyStudentActivationIntervalMin,
		Label:           "Kinderaktivierung Intervall (Minuten)",
		Description:     "Wie oft geprüft wird, ob Kinder mit Status \"ausstehend\" oder mit Abmeldedatum in der Vergangenheit ihren Status wechseln müssen.",
		Type:            config.FieldNumber,
		Default:         60,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "schüleraktivierung",
		SortOrder:       50,
		Validation:      config.Range(5, 1440),
	})

	// --- Web-An/Abmeldung Zugriff (who can toggle presence via web UI) ---

	config.Register(config.Definition{
		Key:             config.KeyWebSpontaneousActivities,
		Label:           "Spontane Aktivitäten über Web/App",
		Description:     "Erlaubt Mitarbeitenden, in der mobilen Weboberfläche unter aktueller Aufsicht spontane Aktivitäten zu starten. Die Aktivität belegt den Raum und wird in den Betreuungsplan geschrieben, auch wenn die Betreuungsplanung deaktiviert ist.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       42,
		DependsOn:       config.DependsOnEq(config.KeyCareConcept, config.CareConceptOpenRooms),
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

	config.Register(config.Definition{
		Key:             config.KeyRequirePickupOfferingReview,
		Label:           "Angebotsabgleich für dauerhafte Gehzeiten",
		Description:     "Bei einer Abweichung wählen Sie ein anderes Angebot oder eine Ausnahme.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "betreuungszeiten",
		SortOrder:       1,
		AccessPolicy:    config.AccessShared,
	})

	// --- Geburtstage (#1542) ---
	//
	// Two switches, not one, because the two populations are not comparable.
	// A child's birthday is everyday OGS business and the display defaults ON;
	// a colleague's birth date is that person's own data, so putting staff
	// names on a screen every team member sees is an explicit decision the
	// school has to make (default OFF). Even then an individual can still
	// remove themselves via the opt-out on their profile page — the setting
	// permits the display, it does not compel anyone into it.

	config.Register(config.Definition{
		Key:             config.KeyBirthdayDisplayEnabled,
		Label:           "Geburtstage auf der Startseite",
		Description:     "Zeigt auf der Startseite, wer heute Geburtstag hat. Montags werden zusätzlich die Geburtstage vom Wochenende nachgetragen. Kinder ohne hinterlegtes Geburtsdatum erscheinen nicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "geburtstage",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyBirthdayDisplayIncludeStaff,
		Label:           "Geburtstage von Mitarbeitenden mitanzeigen",
		Description:     "Zeigt auf der Startseite auch die Geburtstage des Personals, ohne Geburtsjahr. Jede Person kann sich im eigenen Profil davon abmelden.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "geburtstage",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyBirthdayDisplayEnabled, true),
	})

	// --- Notfallliste (#2609) ---
	//
	// The printed Notfallliste is a school's offline backup for the moment the
	// internet is gone, so the health note a school already stores on the child
	// belongs next to the phone number. It is Art. 9 data on a sheet of paper
	// that lies around, though, so the school decides: default ON, because the
	// schools asking for the list are the ones who want it, and a school with a
	// stricter data-protection concept can switch it off. The column is not a
	// second read gate — the note is already visible to every account with
	// users:read in the child's record; the switch only decides whether it is
	// printed.

	config.Register(config.Definition{
		Key:             config.KeyEmergencyListHealthInfo,
		Label:           "Gesundheitsinfos auf der Notfallliste",
		Description:     "Druckt zu jedem anwesenden Kind die hinterlegten Gesundheitsinfos mit: Allergien, Medikamente, medizinische Hinweise. Kinder ohne Eintrag erscheinen als \"Nicht hinterlegt\". Ausgeschaltet enthält die Liste nur Name, Klasse, Ort und Kontakte.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "notfallliste",
		SortOrder:       1,
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
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       60,
	})

	// Parent-submitted absences require approval by default (#2447/#2449). Schools
	// can opt out independently for sick and excused reports. Both switches are
	// only meaningful while the parent absence feature above is enabled.
	config.Register(config.Definition{
		Key:             config.KeyParentSickRequiresApproval,
		Label:           "Krankmeldung muss bestätigt werden",
		Description:     "Eltern senden eine Anfrage. Das Team bestätigt die Krankmeldung oder lehnt sie ab. Bis dahin gilt das Kind als erwartet.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       62,
		DependsOn:       config.DependsOnEq(config.KeyParentSickNoteEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyParentExcusedRequiresApproval,
		Label:           "Entschuldigte Abmeldung muss bestätigt werden",
		Description:     "Eltern senden eine Anfrage. Das Team bestätigt die Abmeldung oder lehnt sie ab. Bis dahin gilt das Kind als erwartet.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       63,
		DependsOn:       config.DependsOnEq(config.KeyParentSickNoteEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyParentNotesEnabled,
		Label:           "Eltern-OGS-Nachrichten",
		Description:     "Wenn aktiviert, können Eltern dem Team über das Elternportal Nachrichten zu ihrem Kind schreiben und das Team kann direkt antworten. Die Unterhaltungen erscheinen im zentralen Nachrichten-Posteingang und in der Kinderdetailansicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       61,
	})

	// OGS-internal colleague chat (#2598). Defaults OFF: an internal staff
	// channel is switched on deliberately by the school, not sprung on it by a
	// deploy. Category "team" keeps it visibly apart from the "elternportal"
	// block right above, so nobody reads it as another parent-facing feature.
	config.Register(config.Definition{
		Key:             config.KeyStaffMessagingEnabled,
		Label:           "Team-Chat für Mitarbeitende",
		Description:     "Wenn aktiviert, können sich Mitarbeitende dieser Schule in moto gegenseitig Nachrichten schreiben. Eltern sehen davon nichts.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "team",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentCarePickupRequestEnabled,
		Label:           "Dauerhafte Abholzeiten durch Eltern ändern lassen",
		Description:     "Wenn aktiviert, können Eltern Änderungen an den dauerhaften wöchentlichen Abholzeiten zur Freigabe einreichen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       64,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentCareModeRequestEnabled,
		Label:           "Dauerhafte Abholart durch Eltern ändern lassen",
		Description:     "Wenn aktiviert, können Eltern Änderungen an der dauerhaften wöchentlichen Abholart zur Freigabe einreichen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       65,
	})

	// Whether a guardian sees the individual staff member's name (first name +
	// last initial, e.g. "Anna M.") on team replies instead of the neutral
	// "OGS [Schulname]" label. Defaults ON so a messaging-active school attributes
	// replies to a person by default. Only messages SENT while this is on are
	// revealed: the per-message visibility is frozen at send time on
	// users.parent_messages.staff_name_visible, so enabling it never retroactively
	// exposes older replies written under anonymity. Hidden in the UI unless
	// messaging (parent_notes_enabled) is on, since it only affects those messages.
	config.Register(config.Definition{
		Key:             config.KeyParentMessageStaffNameVisible,
		Label:           "Name des Teammitglieds in Nachrichten anzeigen",
		Description:     "Wenn aktiviert, sehen Eltern bei Antworten des Teams den Vornamen und den ersten Buchstaben des Nachnamens der antwortenden Person (z. B. „Anna M.“) statt nur „OGS [Schulname]“. Gilt nur für Nachrichten, die ab der Aktivierung geschrieben werden; ältere Nachrichten bleiben anonym. Bereits mit Namen gesendete Nachrichten bleiben sichtbar, wenn die Funktion später wieder deaktiviert wird.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       62,
		DependsOn:       config.DependsOnEq(config.KeyParentNotesEnabled, true),
	})

	// Related-accounts management. Whether a parent may invite further
	// guardians to their own child, and whether they may revoke another
	// account's access. Sensitive (controls access to child data) -> manage.
	config.Register(config.Definition{
		Key:             config.KeyGuardianParentInviteMode,
		Label:           "Weitere Bezugspersonen einladen (Eltern)",
		Description:     "Legt fest, ob Eltern über das Elternportal weitere Bezugspersonen zu ihrem Kind einladen dürfen. \"Mit Freigabe\" stellt die Einladung dem Team zur Bestätigung in eine Warteschlange. Das Team kann unabhängig davon immer einladen.",
		Type:            config.FieldSelect,
		Default:         config.ParentInviteModeDisabled,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       63,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Deaktiviert", Value: config.ParentInviteModeDisabled},
				{Label: "Direkt", Value: config.ParentInviteModeDirect},
				{Label: "Mit Freigabe durch das Team", Value: config.ParentInviteModeStaffApproval},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyGuardianParentCanRemove,
		Label:           "Bezugspersonen entfernen (Eltern)",
		Description:     "Wenn aktiviert, dürfen Eltern den Zugang anderer Konten zu ihrem Kind über das Elternportal entfernen. Die primäre Bezugsperson kann nicht von Eltern entfernt werden. Das Team kann unabhängig davon immer entfernen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       64,
		DependsOn:       config.DependsOnNeq(config.KeyGuardianParentInviteMode, config.ParentInviteModeDisabled),
	})

	config.Register(config.Definition{
		Key:             config.KeyParentPickupChangeEnabled,
		Label:           "Abholzeit über Elternportal ändern",
		Description:     "Wenn aktiviert, können Eltern über das Elternportal für einen einzelnen Tag eine abweichende Abhol- und Bringzeit hinterlegen. Die Änderung gilt nur für diesen Tag und erscheint im Betreuungsplan als von den Eltern geändert.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       65,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentMasterDataEditEnabled,
		Label:           "Stammdaten über Elternportal bearbeiten",
		Description:     "Wenn aktiviert, können Eltern die von ihnen gepflegten Stammdaten ihres Kindes (Gesundheitsangaben, eigene Kontaktdaten) direkt über das Elternportal ändern. Die Änderungen werden sofort übernommen und protokolliert.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       66,
	})

	// Defaults ON, like the other parents-portal write features. The
	// safety-critical part (granting/revoking pickup authority) is constrained
	// structurally, not by this toggle: the can_pickup / is_emergency_contact
	// flags can only ever be set for guardians WITHOUT their own portal account,
	// and every change is audited (audit.guardian_changes). So the toggle
	// only governs whether the feature is exposed at all; a school can still
	// switch it off. config:manage because it can expose pickup-authority changes.
	// NOTE: this toggle also gates a parent editing their OWN contact data — the
	// only portal path for that is UpdateGuardianContact (isSelf). Switching it
	// off therefore disables self-edit too; the Description says so explicitly.
	config.Register(config.Definition{
		Key:             config.KeyParentGuardianManagementEnabled,
		Label:           "Kontaktdaten und Abholberechtigung über Elternportal verwalten",
		Description:     "Wenn aktiviert, können berechtigte Eltern über das Elternportal ihre eigenen Kontaktdaten sowie die von Bezugspersonen ohne eigenen Zugang bearbeiten und deren Abhol- und Notfallberechtigung setzen. Bezugspersonen mit eigenem Konto bleiben geschützt: deren Daten und Berechtigungen ändert nur das Team. Ist die Funktion deaktiviert, können Eltern auch ihre eigenen Kontaktdaten nicht mehr über das Portal ändern.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       67,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentMasterDataRequestEnabled,
		Label:           "Stammdaten-Änderungen zur Freigabe einreichen",
		Description:     "Wenn aktiviert, können Eltern für besonders sensible Angaben (Name, Geburtsdatum, dauerhafte Gehzeiten) über das Elternportal Änderungen vorschlagen. Diese werden dem Team zur Prüfung und Freigabe vorgelegt und erst nach Bestätigung übernommen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       68,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentRequestGroupLeaderReviewEnabled,
		Label:           "Gruppenleitungen dürfen Elternanfragen entscheiden",
		Description:     "Aus: Nur OGS-Admins entscheiden. Ein: Aktuelle Gruppenleitungen und Vertretungen entscheiden zusätzlich für Kinder ihrer Gruppen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       69,
		AccessPolicy:    config.AccessShared,
	})

	config.Register(config.Definition{
		Key:   config.KeyParentRequestReasonPolicy,
		Label: "Begründung bei Anfragen",
		Description: "Legt fest, wer bei einer Anfrage einen Grund schreiben muss. " +
			"Eltern begründen beim Absenden. Mitarbeitende begründen beim Freigeben. " +
			"Eine Ablehnung braucht immer einen Grund. Das ändert diese Einstellung nicht.",
		Type:            config.FieldSelect,
		Default:         config.ReasonPolicyBoth,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       70,
		AccessPolicy:    config.AccessShared,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Niemand muss begründen", Value: config.ReasonPolicyNobody},
				{Label: "Nur Eltern", Value: config.ReasonPolicyGuardians},
				{Label: "Nur Mitarbeitende", Value: config.ReasonPolicyStaff},
				{Label: "Eltern und Mitarbeitende", Value: config.ReasonPolicyBoth},
			},
		},
	})

	// Essensplan. Unlike the other parents-portal features this one is
	// opt-out (default ON): every school gets the meal plan out of the box and
	// can switch it off if it doesn't serve food. When on, staff maintain a
	// per-day dish + optional note and parents can view the current and next
	// week in the parents portal.
	config.Register(config.Definition{
		Key:             config.KeyMealPlanEnabled,
		Label:           "Essensplan",
		Description:     "Wenn aktiviert, kann das Team pro Tag ein Gericht mit optionalem Hinweis hinterlegen. Eltern sehen den Essensplan für die aktuelle und nächste Woche im Elternportal.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       68,
	})

	// Parent broadcast announcements (#1669). When on, staff with the
	// communications:announce permission can publish news to guardians, and
	// guardians see the Neuigkeiten feed in the parents portal. Defaults ON
	// (opt-out), like the other parents-portal features: schools get it
	// immediately and can disable per school.
	config.Register(config.Definition{
		Key:             config.KeyParentNewsEnabled,
		Label:           "Elternmitteilungen (Neuigkeiten)",
		Description:     "Wenn aktiviert, kann das Team über das Elternportal Mitteilungen an ausgewählte Elterngruppen senden (ganze Schule, Klassen, Gruppen, AGs, einzelne Kinder oder offene Anmeldungen). Eltern sehen die Mitteilungen als Neuigkeiten im Elternportal; optional kann eine Lesebestätigung verlangt werden.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       68,
	})
}
