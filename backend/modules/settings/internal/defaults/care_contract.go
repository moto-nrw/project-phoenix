package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// How the contracts of a school come about (#4680, ADR 0088 section 6). The
// Träger sets the way for all its schools, a school may take the other one.
// The values are the constants of the Betreuungsvertrag
// (carecontract.ModeSendLater, carecontract.ModeOnAcceptance).
func init() {
	registerDefinitions(careContractDefinitions)
	registerDefinitions(careContractNoticeDefinitions)
}

// noticeConfirmationPattern admits the placeholders the Betreuungsvertrag
// fills in (carecontract application, notice_text.go) and nothing else in
// curly braces.
var noticeConfirmationPattern = `^(?:[^{}]|\{(?:Kind|Module|Schule|Vertragsende|Eingang|Grund|Träger)\})+$`

// noticeConfirmationMaxLength keeps the confirmation a short letter; the
// table refuses more than 8000 characters.
var noticeConfirmationMaxLength = 4000

// The confirmations of a termination and a withdrawal (#4682). The parents
// get them by mail and see them in the parents portal; the text is kept
// with the declaration as sent.
var careContractNoticeDefinitions = []config.Definition{
	{
		Key:             config.KeyCareContractTerminationConfirmation,
		Levels:          config.LevelTraeger,
		Label:           "Bestätigung einer Kündigung",
		Description:     "Diesen Text bekommen Eltern per E-Mail, wenn sie einen Betreuungsvertrag kündigen. Er steht auch in der Eltern-App. Erlaubte Platzhalter: {Kind}, {Module}, {Schule}, {Vertragsende}, {Eingang}, {Träger}.",
		Type:            config.FieldTextarea,
		Default:         "wir bestätigen Ihre Kündigung des Betreuungsvertrags für {Kind} ({Module}, {Schule}).\n\nIhre Kündigung ist am {Eingang} bei uns eingegangen. Der Vertrag endet mit Ablauf des {Vertragsende}. Bis dahin findet die Betreuung wie gebucht statt.\n\nMit freundlichen Grüßen\n{Träger}",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "vertraege",
		Category:        "vertrag-bestaetigungen",
		SortOrder:       10,
		Validation:      &config.ValidationRules{Required: true, Pattern: &noticeConfirmationPattern, MaxLength: &noticeConfirmationMaxLength},
	}, {
		Key:             config.KeyCareContractWithdrawalConfirmation,
		Levels:          config.LevelTraeger,
		Label:           "Bestätigung eines Widerrufs",
		Description:     "Diesen Text bekommen Eltern per E-Mail, wenn sie einen Betreuungsvertrag widerrufen. Er steht auch in der Eltern-App. Erlaubte Platzhalter: {Kind}, {Module}, {Schule}, {Eingang}, {Träger}.",
		Type:            config.FieldTextarea,
		Default:         "wir bestätigen Ihren Widerruf des Betreuungsvertrags für {Kind} ({Module}, {Schule}).\n\nIhr Widerruf ist am {Eingang} bei uns eingegangen. Der Vertrag ist damit aufgehoben. Haben Sie schon etwas bezahlt, erstatten wir es Ihnen innerhalb von 14 Tagen.\n\nMit freundlichen Grüßen\n{Träger}",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "vertraege",
		Category:        "vertrag-bestaetigungen",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Required: true, Pattern: &noticeConfirmationPattern, MaxLength: &noticeConfirmationMaxLength},
	}, {
		Key:             config.KeyCareContractExtraordinaryConfirmation,
		Levels:          config.LevelTraeger,
		Label:           "Bestätigung einer außerordentlichen Kündigung",
		Description:     "Diesen Text bekommen Eltern per E-Mail, wenn sie einen Betreuungsvertrag außerordentlich kündigen. Das Vertragsende legen Sie erst nach der Prüfung fest. Erlaubte Platzhalter: {Kind}, {Module}, {Schule}, {Eingang}, {Grund}, {Träger}.",
		Type:            config.FieldTextarea,
		Default:         "wir bestätigen den Eingang Ihrer außerordentlichen Kündigung des Betreuungsvertrags für {Kind} ({Module}, {Schule}).\n\nIhre Kündigung ist am {Eingang} bei uns eingegangen. Ihr Grund: {Grund}\n\nWir prüfen Ihren Grund. Danach legen wir fest, wann der Vertrag endet. Das Datum sehen Sie dann in der Eltern-App. Bis dahin findet die Betreuung wie gebucht statt.\n\nMit freundlichen Grüßen\n{Träger}",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "vertraege",
		Category:        "vertrag-bestaetigungen",
		SortOrder:       15,
		Validation:      &config.ValidationRules{Required: true, Pattern: &noticeConfirmationPattern, MaxLength: &noticeConfirmationMaxLength},
	},
}

var careContractDefinitions = []config.Definition{
	{
		Key:             config.KeyCareContractConclusionMode,
		Levels:          config.LevelBoth,
		Label:           "Wann gilt der Betreuungsvertrag?",
		Description:     "Bei der Annahme: Nimmt die Schule eine Anmeldung an, gilt der Vertrag sofort. Die Eltern bekommen ihn per E-Mail und in der Eltern-App und müssen nichts unterschreiben. Voraussetzung: Die Eltern stimmen im Anmeldeformular den Vertragsbedingungen zu (AGB / Teilnahmebedingungen). Später verschicken: Das Träger-Büro erstellt den Vertrag und schickt ihn den Eltern.",
		Type:            config.FieldSelect,
		Default:         "nachtraeglich",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "enrollment",
		Category:        "vertragsabschluss",
		SortOrder:       90,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Später verschicken", Value: "nachtraeglich"},
				{Label: "Bei der Annahme", Value: "bei_annahme"},
			},
		},
	},
}
