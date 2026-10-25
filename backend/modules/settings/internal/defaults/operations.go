package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func init() {
	registerDefinitions(operations_init_betreuungstage_0Definitions, operations_init_sitzungsende_1Definitions, operations_init_nach_hause_2Definitions, operations_init_frueh_abgeholt_3Definitions, operations_init_sitzungsbereinigung_4Definitions, operations_init_sehen_und_bearbeiten_5Definitions, operations_init_aufsicht_6Definitions, operations_init_zeiterfassung_7Definitions, operations_init_abwesenheit_8Definitions, operations_init_anwesenheit_erfassen_9Definitions, operations_init_standort_10Definitions, attendanceChannelsDefinitions, groupModeDefinitions, attendanceLimitsDefinitions, studentPhotosDefinitions, childNavigationDefinitions, pickupReviewsDefinitions, birthdayDisplayDefinitions, parentAttendanceDefinitions, staffMessagingDefinitions, parentFeaturesDefinitions, mealDefinitions)

}

// Operator-only: it turns Saturday and Sunday into care days for the
// whole school, with Friday's blocks, arrival and pickup times (#3921).
// It exists for the public demo schools; real schools keep it off.
// --- Session End (system tab — automated background process) ---
// --- Student Daily Checkout ---
// --- Early checkout note (#3324) ---
// Web-only and independent of NFC: the web checkout dialogs offer an
// optional note when a child leaves this many minutes before its pickup
// time. The note is never required.
// --- Abandoned Session Cleanup (system tab — automated background process) ---
// --- Sichtbereich für Gruppen und laufende Betreuungen (#2801) ---
// --- Abweichende Ankunftszeit für eine Klasse (#2962) ---
// --- Zeiterfassung ---
// Break auto-end interval is NOT registered here. It controls a global ticker
// and is configured via BREAK_AUTO_END_INTERVAL_SECONDS env var only.
// --- Status flag auto-clear (Krank / Entschuldigt badge lifecycle) ---
// --- Anwesenheits-Modus (presence tracking model, operator-only) ---
// --- Bundesland (public-holiday calendar, operator-only, #1418 3a) ---
// --- Anwesenheitserfassung (setup-level decisions) ---
// --- Organisationsmodell (setup-level decisions) ---
// --- Spontane Aktivitäten (#3730) ---
//
// The only switch for spontaneous activities from the web and the app. It
// replaced operations.care_concept, which read like a fundamental mode but
// only ever gated this start path.
// Web assignments beyond an activity's participant limit (#3632). The
// terminal always enforces the limit; this setting only decides whether
// the web and app paths do too. Default on keeps every school's behavior.
// --- Kinderfotos (Datenverwaltung-Erweiterung) ---
// One child navigation instead of three (#1321): "Meine Gruppe" and
// "Aktuelle Aufsicht" leave the sidebar and the bottom bar; the child
// search carries both as tabs. Group and supervision pages stay reachable
// from its menu, because handovers and block work are not a search.
// Default off keeps every school's navigation.
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
// --- Elternportal (parents-portal write features) ---
//
// These gate what guardians may submit through the parents app. Both
// default ON (opt-out): schools with the portal get them immediately
// and can disable per school. The parent endpoints resolve the value
// for the child's tenant before accepting a write.
// Parent-submitted absences require approval by default (#2447/#2449). Schools
// can opt out independently for sick and excused reports. Both switches are
// only meaningful while the parent absence feature above is enabled.
// OGS-internal colleague chat (#2598). Defaults ON since #3254 (opt-out):
// every school without an override gets the chat, and a school that does
// not want it switches it off here; that explicit false stays
// authoritative. Category "team" keeps it visibly apart from the
// "elternportal" block right above, so nobody reads it as another
// parent-facing feature.
// Whether a guardian sees the individual staff member's name (first name +
// last initial, e.g. "Anna M.") on team replies instead of the neutral
// "OGS [Schulname]" label. Defaults ON so a messaging-active school attributes
// replies to a person by default. Only messages SENT while this is on are
// revealed: the per-message visibility is frozen at send time on
// users.parent_messages.staff_name_visible, so enabling it never retroactively
// exposes older replies written under anonymity. Hidden in the UI unless
// messaging (parent_notes_enabled) is on, since it only affects those messages.
// Related-accounts management. Whether a parent may invite further
// guardians to their own child, and whether they may revoke another
// account's access. Sensitive (controls access to child data) -> manage.
// Same-day cutoff for the one-day pickup change (#3163). Empty (the
// default) means no cutoff, so existing schools keep today's behaviour.
// Only today is ever locked; later days stay open. The key names the
// request kind on purpose: another parent request kind gets its own
// cutoff key instead of sharing this one.
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
// Replaces the group-leader switch in the settings page; the switch stays
// stored as the inherited choice until a school picks a scope here.
// Essensplan. Unlike the other parents-portal features this one is
// opt-out (default ON): every school gets the meal plan out of the box and
// can switch it off if it doesn't serve food. When on, staff maintain a
// per-day dish + optional note and parents can view the current and next
// week in the parents portal.
// Parent broadcast announcements (#1669). When on, staff with the
// communications:announce permission can publish news to guardians, and
// guardians see the Neuigkeiten feed in the parents portal. Defaults ON
// (opt-out), like the other parents-portal features: schools get it
// immediately and can disable per school.

var operations_init_betreuungstage_0Definitions = []config.Definition{
	{
		Key:             config.KeyWeekendFollowsFriday,
		Label:           "Wochenende nach dem Freitagsplan",
		Description:     "Samstag und Sonntag gelten als Betreuungstage mit dem Plan vom Freitag: Blöcke, Ankunfts- und Abholzeiten. Für Demo-Schulen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "betreuungstage",
		SortOrder:       1,
		AccessPolicy:    config.AccessOperatorOnly,
	},
}

var operations_init_sitzungsende_1Definitions = []config.Definition{
	{
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
	},
	{
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
	},
	{
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
	},
}

var operations_init_nach_hause_2Definitions = []config.Definition{
	{
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
		DependsOn:       config.DependsOnEq(config.KeyModuleNFCAnwesenheit, true),
	},
	{
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
		DependsOn:       config.DependsOnEq(config.KeyModuleNFCAnwesenheit, true),
	},
	{
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
	},
}

var operations_init_frueh_abgeholt_3Definitions = []config.Definition{
	{
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
	},
	{
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
	},
}

var operations_init_sitzungsbereinigung_4Definitions = []config.Definition{
	{
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
	},
	{
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
	},
	{
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
	},
	{
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
	},
}

var operations_init_sehen_und_bearbeiten_5Definitions = []config.Definition{
	{
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
	},
}

var operations_init_aufsicht_6Definitions = []config.Definition{
	{
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
	},
	{
		Key:             config.KeySchoolPortalWriteScope,
		Label:           "Was Lehrkräfte in moto schule eintragen dürfen",
		Description:     "Gilt für Lehrkräfte mit Zugang zu moto schule. Ohne Häkchen sieht die Schule nur. Die OGS sieht jede Eintragung sofort.",
		Type:            config.FieldMultiSelect,
		Default:         []string{},
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       3,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Andere Ankunftszeit für eine Klasse an einem Tag", Value: config.SchoolPortalWriteScopeClassArrivalExceptions},
				{Label: "Klasse nach dem Unterricht an die OGS übergeben", Value: config.SchoolPortalWriteScopeClassHandover},
				{Label: "Ausflug oder Klassenfahrt der Klasse melden", Value: config.SchoolPortalWriteScopeClassTrips},
				{Label: "Hinweis für heute an die OGS, für die Klasse oder ein Kind", Value: config.SchoolPortalWriteScopeDayHints},
			},
		},
	},
	{
		Key:             config.KeySchoolPortalMorningAttendance,
		Label:           "Lehrkräfte tragen in moto schule ein, wer am Vormittag da ist",
		Description:     "Die Lehrkraft markiert für ihre Klasse heute bei jedem Kind „da“ oder „fehlt“. Die Anwesenheit in der OGS ändert sich dadurch nicht.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "aufsicht",
		SortOrder:       4,
	},
}

var operations_init_zeiterfassung_7Definitions = []config.Definition{
	{
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
	},
	{
		Key:             config.KeyTimeTrackingEnforcePlannedStart,
		Label:           "Einstempeln erst kurz vor Schichtbeginn",
		Description:     "Mitarbeitende können erst kurz vor ihrer ersten Schicht im Dienstplan einstempeln. Zu spät einstempeln geht immer. Tage ohne Schicht sind nicht betroffen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       4,
	},
	{
		Key:             config.KeyTimeTrackingPlannedStartToleranceMinutes,
		Label:           "Einstempeln so viele Minuten vor Schichtbeginn",
		Description:     "So früh vor dem Schichtbeginn können Mitarbeitende einstempeln.",
		Type:            config.FieldNumber,
		Default:         5,
		Validation:      config.Range(0, 120),
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "zeiterfassung",
		SortOrder:       5,
		DependsOn:       config.DependsOnEq(config.KeyTimeTrackingEnforcePlannedStart, true),
	},
	{
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
		SortOrder:       6,
	},
	{
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
		SortOrder:       7,
		DependsOn:       config.DependsOnEq(config.KeyTimeTrackingRequireDeviationReason, true),
	},
}

var operations_init_abwesenheit_8Definitions = []config.Definition{
	{
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
	},
	{
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
	},
}

var operations_init_anwesenheit_erfassen_9Definitions = []config.Definition{
	{
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
	},
}

var operations_init_standort_10Definitions = []config.Definition{
	{
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
	},
}

var statusFlagOptions = &config.SelectOptions{
	Static: []config.SelectOption{
		{Label: "Manuell durch das Team", Value: config.ClearModeManual},
		{Label: "Beim nächsten Einchecken", Value: config.ClearModeNextCheckin},
		{Label: "Am Ende des Tages", Value: config.ClearModeEndOfDay},
	},
}
