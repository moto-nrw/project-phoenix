package templates

// The placeholders of every kind of document, their sample values and the
// standard templates (#4677, user story 92). A kind lists only what its
// documents can fill; the confirmation letter fills every placeholder of its
// kind (parent-billing, ADR 0121).

type kindCatalog struct {
	placeholders []Placeholder
	standard     Template
}

var (
	pFamilyName    = Placeholder{Name: "familienname", Label: "Familienname"}
	pRegistrant    = Placeholder{Name: "anmelder", Label: "Name der Person, die angemeldet hat"}
	pChildren      = Placeholder{Name: "kinder", Label: "Vornamen der Kinder"}
	pDate          = Placeholder{Name: "datum", Label: "Datum des Schreibens"}
	pSender        = Placeholder{Name: "traeger", Label: "Name des Trägers"}
	pContact       = Placeholder{Name: "ansprechperson", Label: "Ansprechperson"}
	pQuestions     = Placeholder{Name: "rueckfragen", Label: "Satz zu Rückfragen mit Ansprechperson"}
	pPeriod        = Placeholder{Name: "zeitraum", Label: "Zeitraum, zum Beispiel die Ferien"}
	pNumber        = Placeholder{Name: "rechnungsnummer", Label: "Rechnungsnummer"}
	pNoticeNumber  = Placeholder{Name: "bescheidnummer", Label: "Nummer des Bescheids"}
	pDebtorNumber  = Placeholder{Name: "debitorennummer", Label: "Debitorennummer der Familie"}
	pPositions     = Placeholder{Name: "positionen", Label: "Tabelle der Beträge", Table: true}
	pTotal         = Placeholder{Name: "summe", Label: "Summe"}
	pDue           = Placeholder{Name: "zahlen_bis", Label: "Fälligkeit (zahlen bis)"}
	pPaymentNote   = Placeholder{Name: "zahlungshinweis", Label: "Satz zur Zahlung mit Betrag und Frist"}
	pNotes         = Placeholder{Name: "hinweise", Label: "Hinweise für die Eltern"}
	pEmergency     = Placeholder{Name: "notfallnummern", Label: "Notfallnummern der ausrichtenden OGS", Table: true}
	pOffer         = Placeholder{Name: "angebot", Label: "Betreuungsangebot"}
	pPlace         = Placeholder{Name: "betreuungsort", Label: "OGS, in der das Kind betreut wird"}
	pContractStart = Placeholder{Name: "vertragsbeginn", Label: "Beginn des Vertrags"}
	pEndsOn        = Placeholder{Name: "kuendigung_zum", Label: "Vertrag endet am"}
)

// tableNames are the placeholders that print a table on a line of their own.
var tableNames = map[string]bool{pPositions.Name: true, pEmergency.Name: true}

var samples = map[string]Value{
	pFamilyName.Name:    {Text: "Muster"},
	pRegistrant.Name:    {Text: "Erika Muster"},
	pChildren.Name:      {Text: "Lina und Ben"},
	pDate.Name:          {Text: "02.11.2026"},
	pSender.Name:        {Text: "Ihr Träger"},
	pContact.Name:       {Text: "Paula Kern"},
	pQuestions.Name:     {Text: "Bei Fragen erreichen Sie Paula Kern unter 05402 1234-0."},
	pPeriod.Name:        {Text: "Herbstferien 2026"},
	pNumber.Name:        {Text: "FE-2026-0001"},
	pNoticeNumber.Name:  {Text: "GB-2026-0001"},
	pDebtorNumber.Name:  {Text: sampleDebtorNumber},
	pTotal.Name:         {Text: "92,50 €"},
	pDue.Name:           {Text: "16.11.2026"},
	pPaymentNote.Name:   {Text: "Bitte überweisen Sie 92,50 € bis zum 16.11.2026 auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer FE-2026-0001 an."},
	pNotes.Name:         {Text: "Bitte denken Sie an: Frühstück, Wechselkleidung, Hausschuhe."},
	pOffer.Name:         {Text: "Offene Ganztagsschule"},
	pPlace.Name:         {Text: "OGS Am Lindenhof"},
	pContractStart.Name: {Text: "01.08.2026"},
	pEndsOn.Name:        {Text: "31.07.2027"},
	pEmergency.Name: {Rows: []Row{
		{Label: "OGS Am Lindenhof", Value: "0170 0000001"},
	}},
}

// common are the placeholders every kind knows.
var common = []Placeholder{pFamilyName, pChildren, pDate, pSender, pContact, pQuestions}

func with(extra ...Placeholder) []Placeholder {
	return append(append([]Placeholder{}, common...), extra...)
}

const (
	standardSalutation = "Sehr geehrte Familie {{familienname}},"
	standardClosing    = "Mit freundlichen Grüßen\n{{traeger}}"
)

var catalog = map[Kind]kindCatalog{
	// The standard confirmation letter is the letter of ADR 0121, field by
	// field like the Bissendorf letter (GH-003).
	KindConfirmation: {
		placeholders: with(pRegistrant, pPeriod, pNumber, pDebtorNumber, pPositions, pTotal, pDue, pPaymentNote, pNotes, pEmergency),
		standard: Template{
			Subject:    "Bestätigung und Rechnung: {{zeitraum}}",
			Salutation: standardSalutation,
			Body: "wir bestätigen die Anmeldung von {{kinder}} für „{{zeitraum}}“. Sie haben gebucht:\n\n" +
				"{{positionen}}\n\n" +
				"**{{zahlungshinweis}}**\n\n" +
				"# Bitte beachten Sie\n{{hinweise}}\n\n" +
				"# Notfallnummern\nIn dringenden Fällen und wenn Ihr Kind krank ist, erreichen Sie die Betreuung hier:\n{{notfallnummern}}\n\n" +
				"{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindInvoice: {
		placeholders: with(pPeriod, pNumber, pDebtorNumber, pPositions, pTotal, pDue, pPaymentNote),
		standard: Template{
			Subject:    "Rechnung {{rechnungsnummer}}: {{zeitraum}}",
			Salutation: standardSalutation,
			Body: "für {{kinder}} berechnen wir Ihnen:\n\n{{positionen}}\n\n" +
				"**{{zahlungshinweis}}**\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindFeeNotice: {
		placeholders: with(pNoticeNumber, pPeriod, pPositions, pTotal, pDue, pPaymentNote),
		standard: Template{
			Subject:    "Bescheid über den Elternbeitrag {{bescheidnummer}}",
			Salutation: standardSalutation,
			Body: "für die Betreuung von {{kinder}} setzen wir für {{zeitraum}} diesen Elternbeitrag fest:\n\n{{positionen}}\n\n" +
				"**{{zahlungshinweis}}**\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindContract: {
		placeholders: with(pOffer, pPlace, pContractStart, pTotal),
		standard: Template{
			Subject:    "Betreuungsvertrag für {{kinder}}",
			Salutation: standardSalutation,
			Body: "hiermit schließen wir den Vertrag über die Betreuung von {{kinder}} im Angebot „{{angebot}}“ in der {{betreuungsort}}.\n\n" +
				"Der Vertrag beginnt am {{vertragsbeginn}}. Der Beitrag beträgt {{summe}} im Monat.\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindCancellation: {
		placeholders: with(pOffer, pPlace, pEndsOn),
		standard: Template{
			Subject:    "Bestätigung Ihrer Kündigung",
			Salutation: standardSalutation,
			Body: "wir haben Ihre Kündigung für {{kinder}} erhalten.\n\n" +
				"Die Betreuung im Angebot „{{angebot}}“ in der {{betreuungsort}} endet am {{kuendigung_zum}}.\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindReminder: {
		placeholders: with(pNumber, pTotal, pDue),
		standard: Template{
			Subject:    "Zahlungserinnerung zur Rechnung {{rechnungsnummer}}",
			Salutation: standardSalutation,
			Body: "für die Rechnung {{rechnungsnummer}} haben wir noch keine Zahlung erhalten.\n\n" +
				"**Bitte überweisen Sie {{summe}} bis zum {{zahlen_bis}}.**\n\n" +
				"Haben Sie schon bezahlt? Dann ist dieses Schreiben erledigt.\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
	KindCertificate: {
		placeholders: with(pOffer, pPlace, pPeriod, pTotal),
		standard: Template{
			Subject:    "Bescheinigung über Betreuungskosten",
			Salutation: standardSalutation,
			Body: "hiermit bescheinigen wir: {{kinder}} wurde im Zeitraum {{zeitraum}} im Angebot „{{angebot}}“ in der {{betreuungsort}} betreut.\n\n" +
				"Sie haben dafür {{summe}} bezahlt.\n\n{{rueckfragen}}",
			Closing: standardClosing,
		},
	},
}
