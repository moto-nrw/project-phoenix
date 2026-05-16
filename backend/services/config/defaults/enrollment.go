package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
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
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}

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
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}
	dependsOnCareEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentCareOfferingsEnabled,
		Condition: "eq",
		Value:     true,
	}

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

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentCareOfferingsRequired,
		Label:           "Betreuungsauswahl verpflichtend",
		Description:     "Eltern müssen mindestens ein Betreuungsangebot auswählen, bevor sie die Anmeldung absenden können.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "betreuungsangebote",
		SortOrder:       31,
		DependsOn:       dependsOnCareEnabled,
	})
}

func registerEnrollmentNotifications() {
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}

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

	// enrollment.show_status_reason_to_parent moved to per-phase column -
	// each phase decides whether parents see admin-provided reason
	// strings on the status page.
}

func registerEnrollmentSafety() {
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}

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

	minRetention := float64(7)
	maxRetention := float64(730)
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
		Validation:      &config.ValidationRules{Min: &minRetention, Max: &maxRetention},
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
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}

	config.Register(config.Definition{
		Key:             config.KeyEnrollmentDefaultActivationMode,
		Label:           "Aktivierung neuer Schüler",
		Description:     "Genehmigte Schüler werden sofort aktiv geschaltet oder erst zum eingetragenen Anmeldedatum (z. B. Schuljahresbeginn).",
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
	minAttempts := float64(1)
	maxAttempts := float64(20)
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
		Validation:      &config.ValidationRules{Min: &minAttempts, Max: &maxAttempts},
		AccessPolicy:    config.AccessOperatorOnly,
	})

	minWorkerInterval := float64(10)
	maxWorkerInterval := float64(600)
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
		Validation:      &config.ValidationRules{Min: &minWorkerInterval, Max: &maxWorkerInterval},
		AccessPolicy:    config.AccessOperatorOnly,
	})

	minTokenTTL := float64(7)
	maxTokenTTL := float64(1825)
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
		Validation:      &config.ValidationRules{Min: &minTokenTTL, Max: &maxTokenTTL},
		AccessPolicy:    config.AccessOperatorOnly,
	})
}

// registerEnrollmentPublicForm wires PR 7's settings - captcha keys,
// grade-level cap, care-offering overflow mode. These are consumed by
// the public submission flow.
func registerEnrollmentPublicForm() {
	dependsOnEnabled := &config.Dependency{
		Key:       config.KeyEnrollmentEnabled,
		Condition: "eq",
		Value:     true,
	}

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
	minGrade := float64(1)
	maxGrade := float64(13)
	config.Register(config.Definition{
		Key:             config.KeyEnrollmentGradeLevelMax,
		Label:           "Höchste Klassenstufe im Formular",
		Description:     "Eltern können Klassenstufen 1 bis zu diesem Wert auswählen. Standard ist 4 (OGS-Schuljahre 1-4); Schulen mit weiterführenden Stufen erhöhen den Wert entsprechend.",
		Type:            config.FieldNumber,
		Default:         4,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       22,
		Validation:      &config.ValidationRules{Min: &minGrade, Max: &maxGrade},
		DependsOn: &config.Dependency{
			Key:       config.KeyEnrollmentCollectGradeLevel,
			Condition: "eq",
			Value:     true,
		},
	})

	// enrollment.care_overflow_mode moved to per-phase column - each
	// phase carries its own waitlist/reject/allow setting now. The
	// constants live on enrollmentModels.PhaseCareOverflow*; the
	// service reads phase.CareOverflowMode at submit time.
}
