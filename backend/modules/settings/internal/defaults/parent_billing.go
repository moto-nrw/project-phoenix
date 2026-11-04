package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Holiday billing of the Träger (#5621): whether its office enters the
// debtor number of the family when it confirms a holiday registration. Only
// the Träger sets it; Bissendorf switches it on, other Träger leave it off.
// The payment sentence of the confirmation letter is a text of the Träger
// with fixed placeholders (#5622, ADR 0121 addendum); empty keeps the
// built-in sentence. How the holiday contribution ranks siblings and when a
// cost bearer takes over are choices of the Träger too (#5664, ADR 0115
// addendum); the defaults keep the fixed chain from before. The values are
// the constants of the Elternabrechnung, which refuses any other.
func init() {
	registerDefinitions(parentBillingDefinitions)
}

// paymentTextPattern admits text with the four placeholders of the payment
// sentence and nothing else in curly braces: {Kind}, {betrag} and an open
// brace are refused. The Elternabrechnung replaces exactly these four
// (letter_text.go).
var paymentTextPattern = `^(?:[^{}]|\{(?:Betrag|Zahlungsfrist|Rechnungsnummer|Debitorennummer)\})*$`

// paymentTextMaxLength keeps the sentence a sentence on the letter.
var paymentTextMaxLength = 600

var parentBillingDefinitions = []config.Definition{
	{
		Key:             config.KeyParentBillingDebtorNumberOnConfirm,
		Levels:          config.LevelTraeger,
		Label:           "Debitorennummer beim Bestätigen abfragen",
		Description:     "Ist das an, tragen Sie beim Bestätigen einer Ferienanmeldung die Debitorennummer der Familie ein. Sie steht dann im Schreiben als Verwendungszweck.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-bestaetigung",
		SortOrder:       10,
	}, {
		Key:             config.KeyParentBillingPaymentText,
		Levels:          config.LevelTraeger,
		Label:           "Zahlungstext im Schreiben",
		Description:     "Dieser Text bittet im Bestätigungsschreiben um die Zahlung. Erlaubte Platzhalter: {Betrag}, {Zahlungsfrist}, {Rechnungsnummer}, {Debitorennummer}. {Debitorennummer} geht nur, wenn jede Rechnung eine Debitorennummer hat. Ist das Feld leer, steht dort der eingebaute Satz.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-schreiben",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Pattern: &paymentTextPattern, AllowEmpty: true, MaxLength: &paymentTextMaxLength},
	}, {
		Key:             config.KeyParentBillingSiblingRank,
		Levels:          config.LevelTraeger,
		Label:           "Welches Kind ist das erste Kind?",
		Description:     "Das erste Kind zahlt den vollen Preis, für die Geschwister gilt die Ermäßigung. Haben zwei Kinder denselben Rang, zählt die früher eingegangene Anmeldung. Bestätigte Beiträge bleiben, wie sie sind.",
		Type:            config.FieldSelect,
		Default:         "highest_contribution",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-beitrag",
		SortOrder:       30,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Das Kind mit dem höchsten Beitrag", Value: "highest_contribution"},
				{Label: "Das älteste Kind", Value: "oldest_child"},
			},
		},
	}, {
		Key:             config.KeyParentBillingCostTakeoverOrder,
		Levels:          config.LevelTraeger,
		Label:           "Was bezahlt ein Kostenträger?",
		Description:     "Gilt, wenn ein Kostenträger den Beitrag eines Kindes übernimmt. Beim vollen Preis gilt die Ermäßigung nur für die anderen Kinder. Bestätigte Beiträge bleiben, wie sie sind.",
		Type:            config.FieldSelect,
		Default:         "after_discount",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-beitrag",
		SortOrder:       40,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Betrag nach der Ermäßigung", Value: "after_discount"},
				{Label: "Vollen Preis", Value: "before_discount"},
			},
		},
	},
}
