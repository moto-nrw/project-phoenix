package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

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
	config.Register(config.Definition{
		Key:             config.KeyEnrollmentEnabled,
		Label:           "Online-Anmeldung aktivieren",
		Description:     "Schaltet das öffentliche Anmeldeformular für Eltern frei. Solange deaktiviert, ist die Anmeldeseite nicht erreichbar.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "allgemein",
		SortOrder:       1,
	})
}

func registerEnrollmentForm() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	// enrollment.open_window_start / end were tenant-wide tunables in
	// the pre-phase model. Phases now own the open/close window per
	// row - see migration 1.15.67 + 1.15.68.

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentCollectGradeLevel,
		Label:           "Klassenstufe abfragen",
		Description:     "Eltern wählen die zukünftige Klassenstufe ihres Kindes (1-13) im Formular. Die konkrete Klasse weist die Schulleitung später zu.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       20,
		DependsOn:       dependsOnEnabled,
	})

	// Concrete-class collection (issue #1833). When on, the public form
	// asks for the concrete future class (e.g. "2a") in addition to the
	// grade level, but only from grade 2 upwards - grade 1 stays
	// grade-level only. The pick list and whether it is mandatory are
	// configured per phase (enrollment.phases.available_school_classes /
	// require_school_class), not here. Nested under the grade-level
	// toggle because the flow keys off the chosen grade.
	config.Register(config.Definition{
		Key:             config.KeyEnrollmentCollectSchoolClass,
		Label:           "Konkrete Klasse abfragen (ab Klasse 2)",
		Description:     "Zusätzlich zur Klassenstufe wird ab der 2. Klasse die konkrete Klasse (z.B. 2a) abgefragt. Die auswählbaren Klassen und ob die Angabe verpflichtend ist, werden je Anmeldephase festgelegt. Für die 1. Klasse wird weiterhin nur die Klassenstufe erfasst.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       21,
		DependsOn: &config.Dependency{
			Key:       config.KeyEnrollmentCollectGradeLevel,
			Condition: "eq",
			Value:     true,
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentAllowSubmissionEdit,
		Label:           "Bearbeitung durch Eltern erlauben",
		Description:     "Eltern können ihre Einreichung über den Status-Link bearbeiten, solange noch keine Entscheidung getroffen wurde.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       21,
		DependsOn:       dependsOnEnabled,
	})
}

func registerEnrollmentCareOfferings() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)
	config.Register(config.Definition{
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
	})

	// enrollment.care_offerings_required moved to per-phase
	// care_offering_selection_mode. Migration 1.15.97 backfills existing
	// phases from the old setting and then removes stored overrides.

	// Post-enrollment changes (#1665). Deliberately NOT dependent on
	// enrollment.enabled: a school can close its online enrollment for the year
	// and still let families change their booking. The gate is off by default —
	// every approved change moves a child between groups and touches capacity
	// and staff planning, so a school opts in.
	config.Register(config.Definition{
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
	})

	// Operator-only, because switching it changes what the whole school sees
	// as "wird erwartet": in this mode the booked care offerings decide which
	// weekdays a child is in care, and the arrival times follow them
	// (#2414, ADR 0005). Schools without a Halbjahresanmeldung keep the
	// class timetable as the only source and are unaffected.
	config.Register(config.Definition{
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
	})

	minLeadDays := float64(0)
	maxLeadDays := float64(90)
	config.Register(config.Definition{
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
	})
}

func registerEnrollmentNotifications() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentNotificationEmails,
		Label:           "Benachrichtigungs-Empfänger",
		Description:     "Komma-getrennte Liste der E-Mail-Adressen, die bei jeder neuen Anmeldung benachrichtigt werden sollen.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "benachrichtigung",
		SortOrder:       40,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentNotifyPerDecision,
		Label:           "Benachrichtigung pro Entscheidung",
		Description:     "Eine Sammel-E-Mail nach Abschluss aller Entscheidungen oder eine eigene E-Mail pro Kind.",
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
	})

	config.Register(config.Definition{
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
	})

	// enrollment.show_status_reason_to_parent moved to per-phase column -
	// each phase decides whether parents see admin-provided reason
	// strings on the status page.
}

func registerEnrollmentSafety() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	config.Register(config.Definition{
		Key:         config.KeyEnrollmentRequireCaptcha,
		Label:       "Captcha verpflichtend",
		Description: "Schützt das öffentliche Formular vor automatisierten Einreichungen über einen Bot-Schutz (z. B. Cloudflare Turnstile). Erfordert konfigurierte Site- und Secret-Keys.",
		Type:        config.FieldBoolean,
		// Default off: a fresh tenant has no Turnstile keys configured,
		// and turning the gate on without keys breaks every public
		// submission. Admins flip this on once both keys are filled in.
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       50,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentDuplicateHandling,
		Label:           "Doppelt-Einreichung",
		Description:     "Verhalten, wenn ein Kind bereits einmal angemeldet wurde: Annahme blockieren, Hinweis anzeigen oder ohne Warnung akzeptieren.",
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
				{Label: "Blockieren", Value: config.EnrollmentDuplicateHandlingBlock},
				{Label: "Hinweis anzeigen, Einreichung erlauben", Value: config.EnrollmentDuplicateHandlingWarn},
				{Label: "Ohne Warnung akzeptieren", Value: config.EnrollmentDuplicateHandlingIgnore},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentRejectedRetentionDays,
		Label:           "Aufbewahrung abgelehnter Anmeldungen (Tage)",
		Description:     "Abgelehnte Anmeldungen werden nach Ablauf dieser Frist automatisch gelöscht (DSGVO-Konformität).",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       52,
		Validation:      config.Range(7, 730),
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentWaitlistEnabled,
		Label:           "Warteliste aktivieren",
		Description:     "Verwaltung kann Anmeldungen auf eine Warteliste setzen statt sofort abzulehnen oder zu genehmigen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       53,
		DependsOn:       dependsOnEnabled,
	})
}

func registerEnrollmentLifecycle() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentDefaultActivationMode,
		Label:           "Aktivierung neuer Kinder",
		Description:     "Genehmigte Kinder werden sofort aktiv geschaltet oder erst zum eingetragenen Anmeldedatum (z. B. Schuljahresbeginn).",
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
				{Label: "Sofort aktiv", Value: config.EnrollmentActivationModeImmediate},
				{Label: "Zum geplanten Anmeldedatum", Value: config.EnrollmentActivationModeScheduled},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentAutoInviteGuardianOnApprove,
		Label:           "Eltern automatisch einladen",
		Description:     "Bei der Genehmigung einer Anmeldung wird automatisch eine Einladung an den Erziehungsberechtigten verschickt. Wenn deaktiviert, muss die Verwaltung die Einladung manuell anstoßen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "aktivierung",
		SortOrder:       61,
		DependsOn:       dependsOnEnabled,
	})
}

func registerEnrollmentSystem() {
	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})
}

// registerEnrollmentPublicForm wires PR 7's settings - captcha keys,
// grade-level cap, care-offering overflow mode. These are consumed by
// the public submission flow.
func registerEnrollmentPublicForm() {
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	// Captcha site key - public, tenant-scoped. Embedded in the
	// public form HTML so the parent's browser can challenge.
	config.Register(config.Definition{
		Key:             config.KeyEnrollmentCaptchaSiteKey,
		Label:           "Captcha Site-Key (Cloudflare Turnstile)",
		Description:     "Öffentlicher Schlüssel des Bot-Schutz-Anbieters (Cloudflare Turnstile). Wird in das Anmeldeformular eingebettet.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       54,
		DependsOn:       dependsOnEnabled,
	})

	// Captcha secret key - server-side only. Sent to Turnstile's
	// /siteverify endpoint along with the parent's token. Operator-
	// only because the secret rotates outside tenant ops.
	config.Register(config.Definition{
		Key:             config.KeyEnrollmentCaptchaSecretKey,
		Label:           "Captcha Secret-Key (Cloudflare Turnstile)",
		Description:     "Geheimer Schlüssel des Bot-Schutz-Anbieters. Wird serverseitig zur Verifikation verwendet und ist niemals für Eltern sichtbar.",
		Type:            config.FieldPassword,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "sicherheit",
		SortOrder:       55,
		DependsOn:       dependsOnEnabled,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	// Grade level cap on the public form. Default 4 (OGS norm); a
	// Gymnasium can extend up to 13.
	config.Register(config.Definition{
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
	})

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
	dependsOnEnabled := config.DependsOnEq(config.KeyEnrollmentEnabled, true)

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalTermsEnabled,
		Label:           "AGB / Teilnahmebedingungen im Anmeldeformular anzeigen",
		Description:     "Blendet im Anmeldeformular eine verpflichtende Zustimmung zu den AGB, Teilnahmebedingungen oder dem Ganztag Info-Brief ein. Nur aktivieren, wenn Ihr Träger tatsächlich Vertragsbedingungen einbezieht.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       79,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalAGBText,
		Label:           "AGB-Text (Anmeldeformular)",
		Description:     "Lege fest, ob Eltern die AGB als Text im Formular lesen oder als PDF öffnen. Über „AGB überarbeiten“ kannst du die Quelle wechseln, den Text bearbeiten oder die PDF-Datei austauschen. Wenn „Text eingeben“ gewählt ist, wird dieser Text im Formular angezeigt.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       80,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
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
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalDSGVOEnabled,
		Label:           "Datenschutzinformation im Anmeldeformular anzeigen",
		Description:     "Blendet im Anmeldeformular eine verpflichtende Kenntnisnahme der Datenschutzinformation ein.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       81,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:   config.KeyEnrollmentLegalDSGVOText,
		Label: "Datenschutzinformation (Anmeldeformular)",
		// Wording note: enrollment data is processed on a contractual /
		// legal-obligation basis, NOT on consent. The form therefore asks
		// parents to ACKNOWLEDGE (Kenntnisnahme) this information, it does
		// not collect a DSGVO "Einwilligung". Keep this description aligned
		// with the acknowledgement label rendered on the public form.
		Description:     "Datenschutzinformation gemäß Art. 13 DSGVO, die Eltern bei der Anmeldung zur Kenntnis nehmen. Markdown wird unterstützt (Überschriften, Fettdruck, Listen, Links). Wird nur angezeigt, wenn der Schalter aktiv ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       82,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalPhotoEnabled,
		Label:           "Fotoeinwilligung im Anmeldeformular anzeigen",
		Description:     "Blendet im Anmeldeformular eine freiwillige Fotoeinwilligung ein.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       83,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalPhotoText,
		Label:           "Hinweis zur Fotoeinwilligung (Anmeldeformular)",
		Description:     "Erläuterung zur optionalen und jederzeit widerrufbaren Fotoeinwilligung, zum Beispiel wo und wie Fotos verwendet werden. Markdown wird unterstützt. Wird nur angezeigt, wenn der Schalter aktiv ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       84,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalEmailContactEnabled,
		Label:           "E-Mail-Kontakt im Anmeldeformular anzeigen",
		Description:     "Blendet im Anmeldeformular einen Hinweis zur Nutzung der E-Mail-Adresse ein.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       85,
		DependsOn:       dependsOnEnabled,
	})

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentLegalEmailContactText,
		Label:           "Hinweis zum E-Mail-Kontakt (Anmeldeformular)",
		Description:     "Erläuterung, wozu die Schule die E-Mail-Adresse nutzt, zum Beispiel Rückfragen und Status-Benachrichtigungen. Markdown wird unterstützt. Wird nur angezeigt, wenn der Schalter aktiv ist und hier ein Text steht.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "rechtstexte",
		SortOrder:       86,
		DependsOn:       dependsOnEnabled,
	})
}
