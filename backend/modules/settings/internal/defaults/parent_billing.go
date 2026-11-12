package defaults

import (
	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// Holiday billing of the Träger (#5621): whether its office enters the
// debtor number of the family when it confirms a holiday registration. Only
// the Träger sets it; Bissendorf switches it on, other Träger leave it off.
// The payment sentence of the confirmation letter is a text of the Träger
// with fixed placeholders (#5622, ADR 0121 addendum); empty keeps the
// built-in sentence. How the holiday contribution ranks siblings and when a
// cost bearer takes over are choices of the Träger too (#5664, ADR 0115
// addendum); the defaults keep the fixed chain from before. How a monthly
// run bills a month in which a contract changes is open with the customer
// (#4684); the default bills the full month by the version on the first.
// Which changes of a single invoice the review of a monthly run allows is
// open with the customer too (#6197, SJR item 2); all four are on by
// default. Whether the parents give the SEPA mandate in the parents portal
// is the Träger's choice after it agreed it with its bank (#4696); off by
// default. The values are
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
		Key:             config.KeyParentBillingDebtorNumberDigitsOnly,
		Levels:          config.LevelTraeger,
		Label:           "Debitorennummer: nur Ziffern",
		Description:     "Ist das an, nimmt moto nur Debitorennummern aus Ziffern mit der Länge darunter an. Das gilt für jede Nummer, die Sie eintragen: bei Ferienanmeldungen und bei Familien. Nummern, die schon gespeichert sind, bleiben, wie sie sind.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-bestaetigung",
		SortOrder:       11,
	},
	debtorNumberDigits(config.KeyParentBillingDebtorNumberMinDigits, 12, "Debitorennummer: Ziffern von", 1),
	debtorNumberDigits(config.KeyParentBillingDebtorNumberMaxDigits, 13, "Debitorennummer: Ziffern bis", 35),
	{
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
	}, {
		Key:             config.KeyParentBillingCareMonthChange,
		Levels:          config.LevelTraeger,
		Label:           "Wenn sich der Vertrag im Monat ändert",
		Description:     "Gilt für den Monatsbeitrag, wenn ein Vertrag im Monat beginnt, endet oder sich ändert. Voller Monat: Es zählt der Vertrag am 1. des Monats. Beginnt die Betreuung erst im Monat, zählt der erste Vertrag den ganzen Monat. Anteilig: Jede Fassung zählt für ihre Tage. Ausgestellte Rechnungen bleiben, wie sie sind.",
		Type:            config.FieldSelect,
		Default:         "version_on_first",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "monatsbeitrag",
		Category:        "monatsbeitrag-lauf",
		SortOrder:       50,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Voller Monat nach dem Vertrag am 1.", Value: "version_on_first"},
				{Label: "Anteilig nach Kalendertagen", Value: "pro_rata"},
			},
		},
	},
	{
		Key:             config.KeyParentBillingOnlineMandate,
		Levels:          config.LevelTraeger,
		Label:           "Lastschrift-Mandat in der Eltern-App",
		Description:     "Ist das an, können Eltern das SEPA-Lastschrift-Mandat in der Eltern-App erteilen und widerrufen. Schalten Sie das erst ein, wenn Ihre Bank Mandate annimmt, die online bestätigt wurden.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "monatsbeitrag",
		Category:        "monatsbeitrag-lastschrift",
		SortOrder:       100,
	},
	careReviewSwitch(config.KeyParentBillingCareReviewAmount, 60, "Betrag einer Rechnung ändern",
		"Ist das an, können Sie vor dem Ausstellen den Betrag einer einzelnen Rechnung ändern. Die Rechnung bekommt dann eine Korrektur-Zeile mit Ihrem Grund."),
	careReviewSwitch(config.KeyParentBillingCareReviewRecipient, 70, "Empfänger einer Rechnung ändern",
		"Ist das an, können Sie vor dem Ausstellen Name und E-Mail des Empfängers einer einzelnen Rechnung ändern. Das gilt nur für diese Rechnung, nicht für die Familie."),
	careReviewSwitch(config.KeyParentBillingCareReviewAddress, 80, "Anschrift einer Rechnung ändern",
		"Ist das an, können Sie vor dem Ausstellen die Anschrift einer einzelnen Rechnung ändern. Das gilt nur für diese Rechnung, nicht für die Familie."),
	careReviewSwitch(config.KeyParentBillingCareReviewPositions, 90, "Positionen einer Rechnung ändern",
		"Ist das an, können Sie vor dem Ausstellen Positionen einer einzelnen Rechnung hinzufügen oder entfernen, jeweils mit Grund."),
}

// debtorNumberDigits is one end of the length of a debtor number (#6675):
// 1 to 35 digits, shown while the Träger allows only digits. The two ends
// are independent values; the Elternabrechnung reads them in either order.
func debtorNumberDigits(key string, order int, label string, def int) config.Definition {
	return config.Definition{
		Key:             key,
		Levels:          config.LevelTraeger,
		Label:           label,
		Description:     "Wie viele Ziffern eine Debitorennummer hat, von 1 bis 35.",
		Type:            config.FieldNumber,
		Default:         def,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "ferien",
		Category:        "ferien-bestaetigung",
		SortOrder:       order,
		Validation:      config.Range(1, 35),
		DependsOn:       config.DependsOnEq(config.KeyParentBillingDebtorNumberDigitsOnly, true),
	}
}

// careReviewSwitch is one change of a single invoice the review of a
// monthly run allows (#6197); every switch is on until the Träger turns it
// off.
func careReviewSwitch(key string, order int, label, description string) config.Definition {
	return config.Definition{
		Key:             key,
		Levels:          config.LevelTraeger,
		Label:           label,
		Description:     description,
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "monatsbeitrag",
		Category:        "monatsbeitrag-pruefung",
		SortOrder:       order,
	}
}
