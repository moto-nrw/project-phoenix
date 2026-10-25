package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

const (
	defaultGradeLevelMax = 4
	minGradeLevel        = 1
	maxGradeLevel        = 13
)

// Parent-enrollment settings registry. Plumbing-only in PR 4 - these values
// are not yet consumed anywhere outside the registry; PRs 5-8 wire them in.
//
// Tab structure:
//   - "form"           - public submission form fields + windows
//   - "anmeldung"      - care offerings + admin notification preferences
//   - "datenschutz"    - captcha, retention, waitlist visibility
//   - "system"         - outbox + status-token plumbing
//
// All write permissions are config:update (admin operational) except
// `enrollment.require_captcha`, `enrollment.rejected_retention_days`, and
// `enrollment.status_token_ttl_days`, which use config:manage. The status-
// token TTL uses AccessOperatorOnly to enforce the §11 "operators only" rule
// without introducing a new permission name (existing AccessOperatorOnly
// pattern matches what the plan calls "platform:config:update").
//
// All labels and descriptions are in German per CLAUDE.md / settings-system
// rules.

func init() {
	registerEnrollmentMaster()
	registerEnrollmentForm()
	registerEnrollmentCareOfferings()
	registerEnrollmentNotifications()
	registerEnrollmentSafety()
	registerEnrollmentLifecycle()
	registerEnrollmentSystem()
	registerEnrollmentPublicForm() // PR 7
	registerEnrollmentLegalTexts()
}

func registerEnrollmentMaster() {
	registerDefinitions(enrollment_registerEnrollmentMaster_allgemein_0Definitions)
}

func registerEnrollmentForm() {

	// enrollment.open_window_start / end were tenant-wide tunables in
	// the pre-phase model. Phases now own the open/close window per
	// row - see migration 1.15.67 + 1.15.68.

	registerDefinitions(enrollment_registerEnrollmentForm_formular_0Definitions)

	// Concrete-class collection (issue #1833). When on, the public form
	// asks for the concrete future class (e.g. "2a") in addition to the
	// grade level, but only from grade 2 upwards - grade 1 stays
	// grade-level only. The pick list and whether it is mandatory are
	// configured per phase (enrollment.phases.available_school_classes /
	// require_school_class), not here. Nested under the grade-level
	// toggle because the flow keys off the chosen grade.

}

func registerEnrollmentCareOfferings() {
	registerDefinitions(enrollment_registerEnrollmentCareOfferings_betreuungsangebote_0Definitions)

	// enrollment.care_offerings_required moved to per-phase
	// care_offering_selection_mode. Migration 1.15.97 backfills existing
	// phases from the old setting and then removes stored overrides.

	// Post-enrollment changes (#1665). Deliberately NOT dependent on
	// enrollment.enabled: a school can close its online enrollment for the year
	// and still let families change their booking. The gate is off by default —
	// every approved change moves a child between groups and touches capacity
	// and staff planning, so a school opts in.

	// Operator-only, because switching it changes what the whole school sees
	// as "wird erwartet": in this mode the booked care offerings decide which
	// weekdays a child is in care, and the arrival times follow them
	// (#2414, ADR 0005). Schools without a Halbjahresanmeldung keep the
	// class timetable as the only source and are unaffected.

	// Kursanmeldung durch Eltern (#3075, SH 4.3, ADR 0012). Läuft über den
	// Angebotsweg: ein Kurs ist ein Betreuungsangebot, das an einer AG hängt,
	// und die Anfrage ist dieselbe Änderungsanfrage. Deshalb hängt der
	// Schalter sichtbar an den Änderungsanfragen — ohne sie gibt es keine
	// Strecke, auf der eine Kursanfrage entschieden werden könnte.

}

func registerEnrollmentNotifications() {

	registerDefinitions(enrollment_registerEnrollmentNotifications_benachrichtigung_0Definitions)

	// enrollment.show_status_reason_to_parent moved to per-phase column -
	// each phase decides whether parents see admin-provided reason
	// strings on the status page.
}

func registerEnrollmentSafety() {

	registerDefinitions(enrollment_registerEnrollmentSafety_sicherheit_0Definitions)

}

func registerEnrollmentLifecycle() {

	registerDefinitions(enrollment_registerEnrollmentLifecycle_aktivierung_0Definitions)

}

func registerEnrollmentSystem() {
	registerDefinitions(enrollment_registerEnrollmentSystem_anmeldung_0Definitions)

}

// registerEnrollmentPublicForm wires PR 7's settings - grade-level cap, care-offering overflow mode. These are consumed by
// the public submission flow.
func registerEnrollmentPublicForm() {

	// The captcha keys are platform configuration (TURNSTILE_SITE_KEY,
	// TURNSTILE_SECRET_KEY): one Turnstile widget serves every school.
	registerDefinitions(enrollment_registerEnrollmentPublicForm_formular_1Definitions)

	// Grade level cap on the public form. Default 4 (OGS norm); a
	// Gymnasium can extend up to 13.

	// enrollment.care_overflow_mode moved to per-phase column - each
	// phase carries its own waitlist/reject/allow setting now. The
	// constants live on enrollmentModels.PhaseCareOverflow*; the
	// service reads phase.CareOverflowMode at submit time.
}

// registerEnrollmentLegalTexts wires the per-tenant legal blocks shown on
// the public enrollment form. Each block has its own show toggle and its
// own Markdown text.
//
// WritePermission is config:manage (not config:update): these are
// legally binding documents with GDPR implications, so they sit at the
// same permission level as the other security/GDPR enrollment settings.
func registerEnrollmentLegalTexts() {

	registerDefinitions(enrollment_registerEnrollmentLegalTexts_rechtstexte_0Definitions)

}

var enrollment_registerEnrollmentLegalTexts_rechtstexte_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentLegalTermsEnabled,
		Levels:          config.LevelBoth,
		Label:           "AGB / Teilnahmebedingungen im Anmeldeformular anzeigen",
		Description:     "Eltern müssen im Anmeldeformular den AGB, Teilnahmebedingungen oder dem Ganztag Info-Brief zustimmen. Nur einschalten, wenn Ihr Träger wirklich Vertragsbedingungen verwendet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       79,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalAGBText,
		Levels:          config.LevelBoth,
		Label:           "AGB-Text (Anmeldeformular)",
		Description:     "Legen Sie fest, ob Eltern die AGB als Text im Formular lesen oder als PDF öffnen. Über „AGB überarbeiten“ wechseln Sie die Quelle, bearbeiten den Text oder tauschen die PDF-Datei aus.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       80,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalAGBDocumentURL,
		Label:           "AGB-Datei (Anmeldeformular)",
		Description:     "Öffentlich abrufbare PDF-Datei mit AGB, Teilnahmebedingungen oder Ganztag Info-Brief. Wird angezeigt, wenn als Quelle PDF gewählt ist.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       80,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalAGBDisplayMode,
		Label:           "AGB-Quelle (Anmeldeformular)",
		Description:     "Legt fest, ob im Anmeldeformular der eingegebene AGB-Text oder ein PDF-Link angezeigt wird.",
		Type:            config.FieldSelect,
		Default:         config.EnrollmentLegalAGBDisplayModeText,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       80,
		DependsOn:       dependsOnEnabled,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Text eingeben", Value: config.EnrollmentLegalAGBDisplayModeText},
				{Label: "PDF-Datei hochladen", Value: config.EnrollmentLegalAGBDisplayModePDF},
			},
		},
	},
	{
		Key:             config.KeyEnrollmentLegalDSGVOEnabled,
		Levels:          config.LevelBoth,
		Label:           "Datenschutzinformation im Anmeldeformular anzeigen",
		Description:     "Eltern bestätigen im Anmeldeformular, dass sie die Datenschutzinformation gelesen haben.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       81,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:    config.KeyEnrollmentLegalDSGVOText,
		Levels: config.LevelBoth,
		Label:  "Datenschutzinformation (Anmeldeformular)",
		// Wording note: enrollment data is processed on a contractual /
		// legal-obligation basis, NOT on consent. The form therefore asks
		// parents to ACKNOWLEDGE (Kenntnisnahme) this information, it does
		// not collect a DSGVO "Einwilligung". Keep this description aligned
		// with the acknowledgement label rendered on the public form.
		Description:     "Datenschutzinformation nach Art. 13 DSGVO, die Eltern bei der Anmeldung lesen. Markdown ist möglich, zum Beispiel für Überschriften, Fettdruck, Listen und Links. Erscheint nur, wenn der Schalter an ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       82,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalPhotoEnabled,
		Levels:          config.LevelBoth,
		Label:           "Fotoeinwilligung im Anmeldeformular anzeigen",
		Description:     "Eltern können im Anmeldeformular freiwillig in Fotos einwilligen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       83,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalPhotoText,
		Levels:          config.LevelBoth,
		Label:           "Hinweis zur Fotoeinwilligung (Anmeldeformular)",
		Description:     "Erklärung zur freiwilligen Fotoeinwilligung, die Eltern jederzeit widerrufen können. Zum Beispiel, wo und wie Sie Fotos verwenden. Markdown ist möglich. Erscheint nur, wenn der Schalter an ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       84,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalEmailContactEnabled,
		Levels:          config.LevelBoth,
		Label:           "E-Mail-Kontakt im Anmeldeformular anzeigen",
		Description:     "Zeigt im Anmeldeformular einen Hinweis, wofür Sie die E-Mail-Adresse nutzen.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       85,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentLegalEmailContactText,
		Levels:          config.LevelBoth,
		Label:           "Hinweis zum E-Mail-Kontakt (Anmeldeformular)",
		Description:     "Wofür die Schule die E-Mail-Adresse nutzt, zum Beispiel für Rückfragen und Nachrichten zum Stand der Anmeldung. Markdown ist möglich. Erscheint nur, wenn der Schalter an ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       86,
		DependsOn:       dependsOnEnabled,
	},
}

var enrollment_registerEnrollmentPublicForm_formular_1Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentGradeLevelMax,
		Label:           "Höchste Klassenstufe im Formular",
		Description:     "Eltern können Klassenstufen 1 bis zu diesem Wert auswählen. Standard ist 4 (OGS-Schuljahre 1-4); Schulen mit weiterführenden Stufen erhöhen den Wert entsprechend.",
		Type:            config.FieldNumber,
		Default:         defaultGradeLevelMax,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       22,
		Validation:      config.Range(minGradeLevel, maxGradeLevel),
		DependsOn:       config.DependsOnEq(config.KeyEnrollmentCollectGradeLevel, true),
	},
}

var enrollment_registerEnrollmentSystem_anmeldung_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentOutboxMaxAttempts,
		Label:           "E-Mail-Versand: maximale Versuche",
		Description:     "Anzahl der Wiederholversuche, bevor eine fehlgeschlagene E-Mail als endgültig gescheitert markiert wird.",
		Type:            config.FieldNumber,
		Default:         6,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "anmeldung",
		SortOrder:       70,
		Validation:      config.Range(1, 20),
		AccessPolicy:    config.AccessOperatorOnly,
	},
	{
		Key:             config.KeyEnrollmentOutboxWorkerIntervalSeconds,
		Label:           "E-Mail-Versand: Worker-Intervall (Sekunden)",
		Description:     "Wie oft der Outbox-Worker neue ausstehende E-Mails aus der Warteschlange holt. Niedrige Werte = schnellerer Versand, höhere Datenbanklast.",
		Type:            config.FieldNumber,
		Default:         30,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "anmeldung",
		SortOrder:       72,
		Validation:      config.Range(10, 600),
		AccessPolicy:    config.AccessOperatorOnly,
	},
	{
		Key:             config.KeyEnrollmentStatusTokenTTLDays,
		Label:           "Status-Link Gültigkeit (Tage)",
		Description:     "Wie lange der per E-Mail versandte Status-/Bearbeitungs-Link gültig bleibt. Nur durch Operatoren änderbar.",
		Type:            config.FieldNumber,
		Default:         365,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "anmeldung",
		SortOrder:       71,
		Validation:      config.Range(7, 1825),
		AccessPolicy:    config.AccessOperatorOnly,
	},
}

var enrollment_registerEnrollmentLifecycle_aktivierung_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentDefaultActivationMode,
		Label:           "Start neuer Kinder nach Genehmigung",
		Description:     "Genehmigte Kinder gehören sofort zur OGS oder erst ab dem eingetragenen Anmeldedatum, zum Beispiel ab Schuljahresbeginn.",
		Type:            config.FieldSelect,
		Default:         config.EnrollmentActivationModeScheduled,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "aktivierung",
		SortOrder:       60,
		DependsOn:       dependsOnEnabled,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Sofort", Value: config.EnrollmentActivationModeImmediate},
				{Label: "Zum geplanten Anmeldedatum", Value: config.EnrollmentActivationModeScheduled},
			},
		},
	},
	{
		Key:             config.KeyEnrollmentAutoInviteGuardianOnApprove,
		Label:           "Eltern automatisch einladen",
		Description:     "Wird eine Anmeldung genehmigt, bekommen die Eltern sofort eine Einladung ins Elternportal. Ausgeschaltet verschickt die Verwaltung die Einladung selbst.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "aktivierung",
		SortOrder:       61,
		DependsOn:       dependsOnEnabled,
	},
}

var enrollment_registerEnrollmentSafety_sicherheit_0Definitions = []config.Definition{
	{
		Key:         config.KeyEnrollmentRequireCaptcha,
		Label:       "Spam-Schutz im Anmeldeformular",
		Description: "Hält Programme ab, die das Formular massenhaft abschicken (Cloudflare Turnstile).",
		Type:        config.FieldBoolean,
		// Default off: each school opts in. The Turnstile keys are platform
		// configuration the server requires at startup.
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       50,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentDuplicateHandling,
		Label:           "Doppelte Anmeldung eines Kindes",
		Description:     "Legt fest, was passiert, wenn für ein Kind schon eine Anmeldung vorliegt.",
		Type:            config.FieldSelect,
		Default:         config.EnrollmentDuplicateHandlingWarn,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       51,
		DependsOn:       dependsOnEnabled,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Anmeldung nicht annehmen", Value: config.EnrollmentDuplicateHandlingBlock},
				{Label: "Hinweis zeigen, Anmeldung annehmen", Value: config.EnrollmentDuplicateHandlingWarn},
				{Label: "Ohne Hinweis annehmen", Value: config.EnrollmentDuplicateHandlingIgnore},
			},
		},
	},
	{
		Key:             config.KeyEnrollmentRejectedRetentionDays,
		Label:           "Abgelehnte Anmeldungen aufbewahren (Tage)",
		Description:     "Danach löscht moto abgelehnte Anmeldungen. So verlangt es der Datenschutz.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       52,
		Validation:      config.Range(7, 730),
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentUploadRetentionDays,
		Label:           "Nachweise aus Anmeldungen aufbewahren (Tage)",
		Description:     "So lange nach der Entscheidung bleiben Dateien gespeichert, die Eltern hochgeladen haben, zum Beispiel Einkommensnachweise. Danach löscht moto die Dateien. Die Antworten im Formular bleiben.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		AccessPolicy:    config.AccessOperatorManaged,
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       52,
		Validation:      config.Range(7, 730),
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentWaitlistEnabled,
		Label:           "Warteliste",
		Description:     "Die Verwaltung kann Anmeldungen auf eine Warteliste setzen, statt sie gleich zu genehmigen oder abzulehnen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       53,
		DependsOn:       dependsOnEnabled,
	},
}

var enrollment_registerEnrollmentNotifications_benachrichtigung_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentNotificationEmails,
		Label:           "Empfänger für neue Anmeldungen",
		Description:     "Diese Adressen bekommen bei jeder neuen Anmeldung eine E-Mail. Mehrere Adressen trennen Sie mit einem Komma. Mitarbeitende können die E-Mail auch selbst unter Anmeldungen einschalten.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "benachrichtigung",
		SortOrder:       40,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentNotifyPerDecision,
		Label:           "E-Mail an Eltern zur Entscheidung",
		Description:     "Gilt, wenn eine Anmeldung mehrere Kinder enthält: eine E-Mail, wenn über alle entschieden ist, oder sofort eine E-Mail je Entscheidung.",
		Type:            config.FieldSelect,
		Default:         config.EnrollmentNotifyPerDecisionDigest,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "benachrichtigung",
		SortOrder:       41,
		DependsOn:       dependsOnEnabled,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Sammel-E-Mail nach allen Entscheidungen", Value: config.EnrollmentNotifyPerDecisionDigest},
				{Label: "Sofort nach jeder Entscheidung", Value: config.EnrollmentNotifyPerDecisionImmediate},
			},
		},
	},
	{
		Key:             config.KeyEnrollmentChangeRequestEmailNotificationsEnabled,
		Label:           "E-Mails zu Änderungsanfragen",
		Description:     "Sendet zusätzliche E-Mails, wenn Eltern nach Prüfungsbeginn eine Änderungsanfrage einreichen, beantworten oder wenn die Verwaltung dazu Rückfragen, Zusagen oder Ablehnungen sendet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "benachrichtigung",
		SortOrder:       42,
		DependsOn:       dependsOnEnabled,
	},
}

var enrollment_registerEnrollmentCareOfferings_betreuungsangebote_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentCareOfferingsEnabled,
		Label:           "Betreuungsangebote anbieten",
		Description:     "Eltern wählen im Formular aus dem Katalog der Betreuungsangebote (z. B. Regelbetreuung, Ferienbetreuung, Mensa).",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       30,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentOfferingChangesEnabled,
		Label:           "Änderungsanfragen zu Betreuungsangeboten erlauben",
		Description:     "Eltern können in der Eltern-App eine Änderung der gebuchten Betreuungsangebote und AGs beantragen. Die Änderung gilt erst nach Freigabe durch die OGS.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       31,
	},
	{
		Key:             config.KeyEnrollmentBookingsAuthoritative,
		Label:           "Buchungen bestimmen die Betreuungstage",
		Description:     "Die gebuchten Betreuungsangebote legen fest, an welchen Wochentagen ein Kind da ist. Ankunfts- und Abholzeiten gelten dann nur an gebuchten Tagen. Nur für Schulen mit Anmeldung über moto.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       32,
		AccessPolicy:    config.AccessOperatorOnly,
	},
	{
		Key:             config.KeyEnrollmentOfferingChangesLeadDays,
		Label:           "Vorlaufzeit für Änderungen (Tage)",
		Description:     "Wie viele Tage im Voraus eine beantragte Änderung frühestens wirksam werden darf. 0 bedeutet: ab morgen.",
		Type:            config.FieldNumber,
		Default:         14,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       32,
		Validation:      &config.ValidationRules{Min: &minLeadDays, Max: &maxLeadDays},
		DependsOn:       config.DependsOnEq(config.KeyEnrollmentOfferingChangesEnabled, true),
	},
	{
		Key:             config.KeyEnrollmentParentCourseRequestsEnabled,
		Label:           "Kursanfragen durch Eltern erlauben",
		Description:     "Eltern sehen in der Eltern-App die Kurse ihrer Schule und können ihr Kind dafür anfragen. Die Anfrage gilt erst nach Freigabe durch die OGS. Ist ein Kurs voll, entsteht ein Platz auf der Warteliste.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       33,
		DependsOn:       config.DependsOnEq(config.KeyEnrollmentOfferingChangesEnabled, true),
	},
}

var dependsOnEnabled = config.DependsOnEq(config.KeyEnrollmentEnabled, true)
var minLeadDays = float64(0)
var maxLeadDays = float64(90)
