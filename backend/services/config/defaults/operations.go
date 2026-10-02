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
		Label:           "„Nach Hause“ ab Uhrzeit",
		Description:     "Ab dieser Uhrzeit können Kinder am Tablet „Nach Hause“ wählen. „Jederzeit“ heißt: den ganzen Tag.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "nach-hause",
		SortOrder:       5,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutEnabled,
		Label:           "Abholzeit jedes Kindes beachten",
		Description:     "Eingeschaltet erscheint „Nach Hause“ erst kurz vor der Abholzeit des Kindes an diesem Tag. Für Kinder ohne Abholzeit gilt die Uhrzeit oben.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "nach-hause",
		SortOrder:       6,
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyPerStudentCheckoutDeltaMinutes,
		Label:           "„Nach Hause“ vor der Abholzeit (Minuten)",
		Description:     "Beispiel: Abholzeit 16:00 und 15 Minuten. Dann kann das Kind ab 15:45 „Nach Hause“ wählen.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "devices",
		Category:        "nach-hause",
		SortOrder:       7,
		Validation:      config.Range(0, 120),
		DependsOn:       config.DependsOnEq(config.KeyPerStudentCheckoutEnabled, true),
	})

	// --- Early checkout note (#3324) ---
	// Web-only and independent of NFC: the web checkout dialogs offer an
	// optional note when a child leaves this many minutes before its pickup
	// time. The note is never required.

	config.Register(config.Definition{
		Key:             config.KeyEarlyCheckoutNoteEnabled,
		Label:           "Grund bei frühem Gehen abfragen",
		Description:     "Wird ein Kind früher als geplant abgemeldet, erscheint ein Feld für den Grund. Der Grund ist freiwillig.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "frueh-abgeholt",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyEarlyCheckoutNoteToleranceMinutes,
		Label:           "Als früh gilt (Minuten vor der Abholzeit)",
		Description:     "Beispiel: Abholzeit 15:00 und 15 Minuten. Wer ein Kind vor 14:45 abmeldet, wird nach dem Grund gefragt.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "frueh-abgeholt",
		SortOrder:       2,
		Validation:      config.Range(0, 240),
		DependsOn:       config.DependsOnEq(config.KeyEarlyCheckoutNoteEnabled, true),
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
		Label:           "Welche Gruppen und Blöcke sieht das Team?",
		Description:     "Die Auswahl ändert nur den Überblick, nicht die Schreibrechte. Freigegebene offene Räume bleiben sichtbar.",
		Type:            config.FieldSelect,
		Default:         config.OverviewScopeAllStaff,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sehen-und-bearbeiten",
		SortOrder:       1,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Alle Gruppen und Blöcke", Value: config.OverviewScopeAllStaff},
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
		Description:     "Ab diesem Datum rechnet moto das Stundenkonto. Ohne Datum beginnt es am 1. Januar dieses Jahres.",
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
		Description:     "Mitarbeitende können erst ab der Startzeit aus ihrem Arbeitszeitmodell einstempeln. Tage ohne Startzeit sind nicht betroffen.",
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
		Description: "Wer deutlich früher oder später als geplant ein- oder ausstempelt, gibt einen Grund an. Das gilt auch für nachträgliche Änderungen eigener Zeiten. Tage ohne geplante Schicht sind nicht betroffen.",
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
		Label:           "Erlaubte Abweichung vom Dienstplan (Minuten)",
		Description:     "So viele Minuten darf die Stempelzeit von der geplanten Schicht abweichen, ohne dass ein Grund nötig ist.",
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
			{Label: "Manuell durch das Team", Value: config.ClearModeManual},
			{Label: "Beim nächsten Einchecken", Value: config.ClearModeNextCheckin},
			{Label: "Am Ende des Tages", Value: config.ClearModeEndOfDay},
		},
	}

	config.Register(config.Definition{
		Key:             config.KeySickClearMode,
		Label:           "Krankmeldung automatisch beenden",
		Description:     "Legt fest, wann eine Krankmeldung von selbst endet. „Am Ende des Tages“ heißt: um 18 Uhr. Bei „Beim nächsten Einchecken“ bleibt sie auch nach dem Enddatum stehen, bis das Kind wieder da ist.",
		Type:            config.FieldSelect,
		Default:         config.ClearModeEndOfDay,
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
		Description:     "Legt fest, wann eine Entschuldigung von selbst endet. „Am Ende des Tages“ heißt: um 18 Uhr.",
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
		Label:           "Was wird erfasst?",
		Description:     "Wählen Sie, ob auch der Aufenthaltsort erfasst wird.",
		Type:            config.FieldSelect,
		Default:         config.PresenceModeDetailed,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit-erfassen",
		SortOrder:       2,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Zusätzlich Räume und Aktivitäten", Value: config.PresenceModeDetailed},
				{Label: "Anwesend oder abwesend", Value: config.PresenceModeBinary},
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
		Label:           "Anwesenheit am Handy oder Computer erfassen",
		Description:     "Das Team kann Kinder an- und abmelden. Das geht auch zusätzlich zu NFC-Geräten.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit-erfassen",
		SortOrder:       0,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyAttendanceNFCEnabled,
		Label:           "NFC-Geräte verwenden",
		Description:     "Die OGS nutzt NFC-Armbänder oder Karten an Geräten, zum Beispiel für Räume, Schulhof oder Abmeldung.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit-erfassen",
		SortOrder:       1,
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

	// --- Spontane Aktivitäten (#3730) ---
	//
	// The only switch for spontaneous activities from the web and the app. It
	// replaced operations.care_concept, which read like a fundamental mode but
	// only ever gated this start path.

	config.Register(config.Definition{
		Key:             config.KeyWebSpontaneousActivities,
		Label:           "Spontane Aktivitäten erlauben",
		Description:     "Mitarbeitende können am Computer oder Handy unter \"Aktuelle Aufsicht\" spontan eine Aktivität starten. Die Aktivität belegt den Raum und steht danach im Betreuungsplan.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       42,
	})

	// Web assignments beyond an activity's participant limit (#3632). The
	// terminal always enforces the limit; this setting only decides whether
	// the web and app paths do too. Default on keeps every school's behavior.
	config.Register(config.Definition{
		Key:             config.KeyWebExceedParticipantLimit,
		Label:           "Mehr Kinder als die Teilnehmergrenze erlauben",
		Description:     "Gilt, wenn Mitarbeitende am Computer oder Handy Kinder einer Aktivität zuordnen. Eingeschaltet: Die Teilnehmergrenze darf dort überschritten werden. Ausgeschaltet: Ist die Aktivität voll, wird kein Kind zugeordnet. Am Tablet gilt die Grenze immer.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "anwesenheit",
		SortOrder:       43,
	})

	// --- Kinderfotos (Datenverwaltung-Erweiterung) ---

	config.Register(config.Definition{
		Key:             config.KeyStudentPhotosEnabled,
		Label:           "Kinderfotos",
		Description:     "Wer Kinder bearbeiten darf, kann in der Datenverwaltung Fotos hinterlegen. Nur mit nachweisbarer Einwilligung der Eltern. Die Fotos erscheinen in Suche, Räumen, Abholplan und beim Kind.",
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
		Label:           "Abgleich fester Abholzeiten mit dem Angebot",
		Description:     "Passt eine neue feste Abholzeit nicht zum gebuchten Betreuungsangebot, wählt das Team ein anderes Angebot oder eine Ausnahme.",
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
	//
	// Both live on the hand-written "Startseite für alle" tab next to the
	// birthday card (#3737); the generic settings page filters the
	// "startseite" tab out.

	config.Register(config.Definition{
		Key:             config.KeyBirthdayDisplayEnabled,
		Label:           "Geburtstage auf der Startseite",
		Description:     "Zeigt auf der Startseite, wer in dieser Woche Geburtstag hat. Man kann bis zu 4 Wochen zurück- und vorblättern. Kinder ohne hinterlegtes Geburtsdatum erscheinen nicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "startseite",
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
		Tab:             "startseite",
		Category:        "geburtstage",
		SortOrder:       2,
		DependsOn:       config.DependsOnEq(config.KeyBirthdayDisplayEnabled, true),
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
		Label:           "Nachrichten von Eltern",
		Description:     "Eltern schreiben dem Team im Elternportal zu ihrem Kind. Das Team antwortet direkt. Die Nachrichten stehen unter „Nachrichten“ und beim Kind.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       61,
	})

	// OGS-internal colleague chat (#2598). Defaults ON since #3254 (opt-out):
	// every school without an override gets the chat, and a school that does
	// not want it switches it off here; that explicit false stays
	// authoritative. Category "team" keeps it visibly apart from the
	// "elternportal" block right above, so nobody reads it as another
	// parent-facing feature.
	config.Register(config.Definition{
		Key:             config.KeyStaffMessagingEnabled,
		Label:           "Team-Chat für Mitarbeitende",
		Description:     "Mitarbeitende und Lehrkräfte Ihrer Schule schreiben sich in moto Nachrichten. Eltern sehen davon nichts. Ausgeschaltet ist der Team-Chat für niemanden sichtbar.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "team",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyParentCarePickupRequestEnabled,
		Label:           "Feste Abholzeiten ändern (Eltern)",
		Description:     "Eltern können im Elternportal neue feste Abholzeiten für die Wochentage beantragen. Sie gelten erst nach Freigabe durch das Team.",
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
		Label:           "Feste Abholart ändern (Eltern)",
		Description:     "Eltern können im Elternportal eine neue feste Abholart für die Wochentage beantragen. Sie gilt erst nach Freigabe durch das Team.",
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
		Description:     "Eltern sehen bei Antworten dann zum Beispiel „Anna M.“ statt nur „OGS [Schulname]“. Das gilt für Nachrichten ab dem Einschalten. Schon gesendete Nachrichten ändern sich nicht.",
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
		Description:     "Legt fest, ob Eltern im Elternportal weitere Bezugspersonen zu ihrem Kind einladen dürfen. Bei „Mit Freigabe“ bestätigt das Team jede Einladung. Das Team selbst kann immer einladen.",
		Type:            config.FieldSelect,
		Default:         config.ParentInviteModeDisabled,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       63,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Nicht erlaubt", Value: config.ParentInviteModeDisabled},
				{Label: "Ohne Freigabe", Value: config.ParentInviteModeDirect},
				{Label: "Mit Freigabe durch das Team", Value: config.ParentInviteModeStaffApproval},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyGuardianParentCanRemove,
		Label:           "Bezugspersonen entfernen (Eltern)",
		Description:     "Eltern dürfen im Elternportal anderen Personen den Zugang zu ihrem Kind entziehen. Die primäre Bezugsperson können Eltern nicht entfernen. Das Team kann immer entfernen.",
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
		Label:           "Abholzeit für einen Tag ändern (Eltern)",
		Description:     "Eltern können im Elternportal für einen einzelnen Tag eine andere Bring- oder Abholzeit eintragen. Im Betreuungsplan steht dann, dass die Eltern sie geändert haben.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       65,
	})

	// Same-day cutoff for the one-day pickup change (#3163). Empty (the
	// default) means no cutoff, so existing schools keep today's behaviour.
	// Only today is ever locked; later days stay open. The key names the
	// request kind on purpose: another parent request kind gets its own
	// cutoff key instead of sharing this one.
	config.Register(config.Definition{
		Key:             config.KeyParentPickupChangeCutoffTime,
		Label:           "Änderungsfrist für die Abholzeit am selben Tag",
		Description:     "Bis zu dieser Uhrzeit können Eltern die Abholzeit für heute ändern. Danach ist heute für Eltern gesperrt. Für morgen und spätere Tage gilt keine Frist. Das Team kann die Abholzeit immer ändern. „Jederzeit“ heißt: keine Frist.",
		Type:            config.FieldTime,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       66,
		DependsOn:       config.DependsOnEq(config.KeyParentPickupChangeEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyParentMasterDataEditEnabled,
		Label:           "Stammdaten bearbeiten (Eltern)",
		Description:     "Eltern ändern Gesundheitsangaben und ihre eigenen Kontaktdaten direkt im Elternportal. Die Änderung gilt sofort. moto hält fest, wer was geändert hat.",
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
		Label:           "Kontaktdaten und Abholberechtigung verwalten (Eltern)",
		Description:     "Eltern ändern im Elternportal ihre Kontaktdaten. Bei Bezugspersonen ohne eigenes Konto auch, ob sie abholen dürfen und im Notfall angerufen werden. Personen mit eigenem Konto ändert nur das Team. Ausgeschaltet können Eltern auch ihre eigenen Kontaktdaten dort nicht ändern.",
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
		Label:           "Änderungen an Stammdaten vorschlagen (Eltern)",
		Description:     "Eltern schlagen im Elternportal Änderungen an Name, Geburtsdatum oder festen Gehzeiten vor. Das Team prüft sie. Erst nach der Freigabe gelten sie.",
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
		Description:     "Das Team trägt pro Tag ein Gericht ein, auf Wunsch mit Hinweis. Eltern sehen den Plan für diese und nächste Woche im Elternportal.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       68,
	})

	config.Register(config.Definition{
		Key:             config.KeyMealRegistrationEnabled,
		Label:           "Anmeldung zum Mittagessen",
		Description:     "Eltern melden ihr Kind für feste Wochentage oder einzelne Tage zum Essen an. Das Team bekommt daraus eine Tagesliste für die Küche.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       69,
		DependsOn:       config.DependsOnEq(config.KeyMealPlanEnabled, true),
	})

	config.Register(config.Definition{
		Key:             config.KeyMealRegistrationCutoffTime,
		Label:           "Änderungsfrist für das Mittagessen",
		Description:     "Bis zu dieser Uhrzeit können Eltern die Anmeldung für den gleichen Tag ändern. Spätere Krankmeldungen ändern die Küchenliste nicht mehr.",
		Type:            config.FieldTime,
		Default:         "09:00",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       70,
		DependsOn:       config.DependsOnEq(config.KeyMealRegistrationEnabled, true),
	})

	// Parent broadcast announcements (#1669). When on, staff with the
	// communications:announce permission can publish news to guardians, and
	// guardians see the Neuigkeiten feed in the parents portal. Defaults ON
	// (opt-out), like the other parents-portal features: schools get it
	// immediately and can disable per school.
	config.Register(config.Definition{
		Key:             config.KeyParentNewsEnabled,
		Label:           "Elternmitteilungen (Neuigkeiten)",
		Description:     "Das Team schreibt Mitteilungen an Eltern: an die ganze Schule, Klassen, Gruppen, AGs, einzelne Kinder oder offene Anmeldungen. Eltern sehen sie im Elternportal unter Neuigkeiten. Auf Wunsch bestätigen Eltern, dass sie die Mitteilung gelesen haben.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "elternportal",
		SortOrder:       68,
	})
}
