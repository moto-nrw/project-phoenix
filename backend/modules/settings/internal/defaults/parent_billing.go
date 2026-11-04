package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Holiday billing of the Träger (#5621): whether its office enters the
// debtor number of the family when it confirms a holiday registration. Only
// the Träger sets it; Bissendorf switches it on, other Träger leave it off.
// The payment sentence of the confirmation letter is a text of the Träger
// with fixed placeholders (#5622, ADR 0121 addendum); empty keeps the
// built-in sentence.
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
	},
}
