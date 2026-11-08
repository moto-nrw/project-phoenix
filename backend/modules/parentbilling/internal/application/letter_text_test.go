package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func letterDate(t *testing.T, value string) *calendar.Date {
	t.Helper()
	date, err := calendar.ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return &date
}

func goldenLetterContent(t *testing.T) letterContent {
	t.Helper()
	rank := 2
	basisPoints := 5000
	discount := int64(-3250)
	return letterContent{
		run: parentbilling.BillingRun{ID: 3, PeriodName: "Winterferien 2027", Status: parentbilling.RunReleased},
		document: parentbilling.BillingDocument{
			ID: 9, Number: "FE-2026-0007", IssuedOn: letterDate(t, "2026-12-01"), RecipientFirstName: "Erika",
			RecipientLastName: "Beispiel", RecipientEmail: "erika@example.org", TotalCents: 9250,
			Children: []parentbilling.BillingChild{
				{FirstName: "Lina", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
					{Kind: parentbilling.LineBase, AmountCents: 5000, WeekStartsOn: letterDate(t, "2027-01-04"),
						WeekEndsOn: letterDate(t, "2027-01-08"), TimeModelName: "Vormittag"},
				}},
				{FirstName: "Ben", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
					{Kind: parentbilling.LineBase, AmountCents: 6500, WeekStartsOn: letterDate(t, "2027-01-04"),
						WeekEndsOn: letterDate(t, "2027-01-08"), TimeModelName: "Nachmittag mit Mittagessen"},
					{Kind: parentbilling.LineSibling, AmountCents: discount, ChildRank: &rank, DiscountBasisPoints: &basisPoints},
					{Kind: parentbilling.LineCorrection, AmountCents: 2000, Note: "Zusätzlicher Ausflug"},
				}},
			},
		},
		input: parentbilling.BillingLetterInput{
			PaymentDueOn: *letterDate(t, "2026-12-15"),
			Notes:        "Bitte denken Sie an: Frühstück, Wechselkleidung, Hausschuhe.\nKinder mit ansteckenden Krankheiten bleiben zu Hause.",
			EmergencyContacts: []parentbilling.BillingEmergencyContact{
				{Name: "OGS Am Lindenhof", Phone: "0170 0000001"},
				{Name: "OGS Schule am Hang", Phone: "0170 0000002"},
			},
		},
		letterhead: parentbilling.Letterhead{
			Logo: []byte("logo"), AccentColor: "#217A78", SenderText: "Ganztag im Quartier e.V.\nAm Markt 4\n49143 Bissendorf",
			FooterText: "Vorstand: Anna Brandt", ContactName: "Paula Kern", ContactPhone: "05402 1234-0",
			ContactEmail: "ferien@ganztag.example",
			BankAccount: &parentbilling.LetterheadBankAccount{
				AccountHolder: "Ganztag im Quartier e.V.", IBAN: "DE02120300000000202051", BIC: "BYLADEM1001", BankName: "Beispielbank",
			},
		},
	}
}

// TestConfirmationLetterFollowsTheBissendorfTemplate is the golden test of
// the letter against the Bissendorf confirmation letter (GH-003), field by
// field. The template's merge fields and fixed texts map like this:
//
//	«Familienname» in the address field  → Recipient "Familie Beispiel"
//	«Anschrift», «PLZ_Ort»               → the person who registered, then
//	                                         street and postal code with city
//	                                         when the invoice copied an address
//	                                         from the registration (ADR 0121
//	                                         addendum, #5625); without one the
//	                                         two lines of before
//	Infoblock (Sachbearbeiterin, phone)  → Facts Ansprechperson, Telefon, E-Mail
//	Datum                                → Facts Datum (issue date of the invoice)
//	Betreff                              → Subject with the period
//	"Sehr geehrte Familie «Familienname»"→ Salutation
//	«Vorname_Kind»                       → the first names in the confirmation
//	«Gesamtbeitrag»                      → Gesamtbetrag, after every position
//	«Adressnummer» as payment reference  → the debtor number the invoice
//	                                         copied from the registration, in
//	                                         the facts under the invoice number
//	                                         (ADR 0121 addendum, #5621); without
//	                                         one the invoice number
//	Zahlungsfrist                        → the payment date of the run
//	Konto bei der Sparkasse              → Footer Bankverbindung (default account)
//	Packliste, Krankheitsregel           → the free text of the run
//	Notfallnummern der Betreuungsteams   → the emergency numbers of the run
//	Rückfragen an …                      → the question line with the contact
//	"Mit freundlichen Grüßen", Absender  → Closing
//	Briefbogen                           → Logo, colour, letterhead, footer
func TestConfirmationLetterFollowsTheBissendorfTemplate(t *testing.T) {
	t.Parallel()

	want := Letter{
		Title:       "Bestätigung und Rechnung FE-2026-0007",
		AccentColor: "#217A78",
		Logo:        []byte("logo"),
		Sender:      []string{"Ganztag im Quartier e.V.", "Am Markt 4", "49143 Bissendorf"},
		Recipient:   []string{"Familie Beispiel", "Erika Beispiel"},
		Facts: []LetterFact{
			{Label: "Rechnungsnummer", Value: "FE-2026-0007"},
			{Label: "Datum", Value: "01.12.2026"},
			{Label: "Zahlen bis", Value: "15.12.2026"},
			{Label: "Ansprechperson", Value: "Paula Kern"},
			{Label: "Telefon", Value: "05402 1234-0"},
			{Label: "E-Mail", Value: "ferien@ganztag.example"},
		},
		Subject:    "Bestätigung und Rechnung: Winterferien 2027",
		Salutation: "Sehr geehrte Familie Beispiel,",
		Blocks: []LetterBlock{
			{Text: "wir bestätigen die Anmeldung von Lina und Ben für „Winterferien 2027“. Sie haben gebucht:"},
			{Facts: []LetterFact{
				{Label: "Lina: Woche 04.01. bis 08.01.2027, Vormittag", Value: "50,00 €"},
				{Label: "Ben: Woche 04.01. bis 08.01.2027, Nachmittag mit Mittagessen", Value: "65,00 €"},
				{Label: "Ben: Ermäßigung für Geschwister", Value: "-32,50 €"},
				{Label: "Ben: Korrektur: Zusätzlicher Ausflug", Value: "20,00 €"},
				{Label: "Gesamtbetrag", Value: "92,50 €", Bold: true},
			}},
			{Text: "Bitte überweisen Sie 92,50 € bis zum 15.12.2026 auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer FE-2026-0007 an.", Bold: true},
			{Heading: "Bitte beachten Sie", Text: "Bitte denken Sie an: Frühstück, Wechselkleidung, Hausschuhe.\nKinder mit ansteckenden Krankheiten bleiben zu Hause."},
			{
				Heading: "Notfallnummern",
				Text:    "In dringenden Fällen und wenn Ihr Kind krank ist, erreichen Sie die Betreuung hier:",
				Facts: []LetterFact{
					{Label: "OGS Am Lindenhof", Value: "0170 0000001"},
					{Label: "OGS Schule am Hang", Value: "0170 0000002"},
				},
			},
			{Text: "Bei Fragen erreichen Sie Paula Kern unter 05402 1234-0 oder ferien@ganztag.example."},
		},
		Closing: []string{"Mit freundlichen Grüßen", "Ganztag im Quartier e.V."},
		Footer: []string{
			"Vorstand: Anna Brandt",
			"Bankverbindung: Ganztag im Quartier e.V. · IBAN DE02 1203 0000 0000 2020 51 · BIC BYLADEM1001 · Beispielbank",
		},
	}
	assert.Equal(t, want, confirmationLetter(goldenLetterContent(t)))

	withAddress := goldenLetterContent(t)
	withAddress.document.RecipientAddress = &parentbilling.PostalAddress{Street: "Musterweg 12", PostalCode: "49143", City: "Bissendorf"}
	want.Recipient = []string{"Familie Beispiel", "Erika Beispiel", "Musterweg 12", "49143 Bissendorf"}
	assert.Equal(t, want, confirmationLetter(withAddress), "the address field names the postal address in four lines")

	withDebtor := goldenLetterContent(t)
	withDebtor.document.DebtorNumber = "10 4711"
	want.Recipient = []string{"Familie Beispiel", "Erika Beispiel"}
	want.Facts = append([]LetterFact{want.Facts[0], {Label: "Debitorennummer", Value: "10 4711"}}, want.Facts[1:]...)
	want.Blocks[2] = LetterBlock{
		Text: "Bitte überweisen Sie 92,50 € bis zum 15.12.2026 auf das Konto unten. Geben Sie als Verwendungszweck Ihre Debitorennummer 10 4711 an.",
		Bold: true,
	}
	assert.Equal(t, want, confirmationLetter(withDebtor),
		"the debtor number stands under the invoice number and is the reference to pay with")
}

// bissendorfPaymentText is the payment text of the Bissendorf letter with
// the sentence for families with a direct debit mandate (#5622). It stands
// in every letter: moto does not know which family gave a mandate.
const bissendorfPaymentText = "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. " +
	"Geben Sie als Verwendungszweck Ihre Debitorennummer {Debitorennummer} an. " +
	"Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab."

// TestConfirmationLetterWithThePaymentTextOfTheTraeger is the golden test
// of the Träger's own payment text (#5622, ADR 0121 addendum): it replaces
// the built-in sentence, every placeholder is filled in, amount and date in
// the format of the built-in sentence, and the sentence stays bold. The rest
// of the letter does not change.
func TestConfirmationLetterWithThePaymentTextOfTheTraeger(t *testing.T) {
	t.Parallel()

	builtIn := goldenLetterContent(t)
	builtIn.document.DebtorNumber = "10 4711"
	content := goldenLetterContent(t)
	content.document.DebtorNumber = "10 4711"
	content.letterhead.PaymentText = bissendorfPaymentText
	want := confirmationLetter(builtIn)
	want.Blocks[2] = LetterBlock{
		Text: "Bitte überweisen Sie 92,50 € bis zum 15.12.2026 auf das Konto unten. " +
			"Geben Sie als Verwendungszweck Ihre Debitorennummer 10 4711 an. " +
			"Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab.",
		Bold: true,
	}
	assert.Equal(t, want, confirmationLetter(content), "the Bissendorf text with the sentence on the mandate")

	content.letterhead.PaymentText = "  {Rechnungsnummer}, {Betrag}, {Zahlungsfrist}, {Debitorennummer}, {Betrag} again.\n"
	assert.Equal(t, "FE-2026-0007, 92,50 €, 15.12.2026, 10 4711, 92,50 € again.", confirmationLetter(content).Blocks[2].Text,
		"every placeholder is filled in, as often as it stands")

	content.letterhead.PaymentText = "   "
	assert.Equal(t, confirmationLetter(builtIn), confirmationLetter(content), "a blank text keeps the built-in sentence")

	content.letterhead.PaymentText = bissendorfPaymentText
	content.document.TotalCents = 0
	assert.Equal(t, LetterBlock{Text: "Sie müssen für diese Betreuung nichts bezahlen.", Bold: true}, confirmationLetter(content).Blocks[2],
		"an invoice of 0 € keeps the fixed sentence, also with a text of the Träger")
}

// TestPaymentTextRefusesLettersWithoutTheDebtorNumberItNames holds that no
// letter prints a gap where the reference to pay with belongs (#5622): a
// payment text with {Debitorennummer} needs the number on every invoice
// that asks for payment.
func TestPaymentTextRefusesLettersWithoutTheDebtorNumberItNames(t *testing.T) {
	t.Parallel()

	documents := []parentbilling.BillingDocument{
		{Number: "FE-0001", TotalCents: 9250, DebtorNumber: "10 4711"},
		{Number: "FE-0002", TotalCents: 5000},
		{Number: "FE-0003", TotalCents: 0},
		{Number: "FE-0004", TotalCents: 2000, DebtorNumber: "  "},
	}
	err := debtorNumbersForPaymentText(parentbilling.Letterhead{PaymentText: bissendorfPaymentText}, documents)
	var missing *parentbilling.PaymentTextDebtorNumberMissingError
	require.ErrorAs(t, err, &missing)
	assert.ErrorIs(t, err, parentbilling.ErrPaymentTextDebtorNumberMissing)
	assert.Equal(t, []string{"FE-0002", "FE-0004"}, missing.InvoiceNumbers, "an invoice of 0 € needs no reference")

	for name, text := range map[string]string{
		"built-in sentence":       "",
		"text without the number": "Bitte überweisen Sie {Betrag} mit der Rechnungsnummer {Rechnungsnummer}.",
	} {
		assert.NoError(t, debtorNumbersForPaymentText(parentbilling.Letterhead{PaymentText: text}, documents), name)
	}
	assert.NoError(t, debtorNumbersForPaymentText(parentbilling.Letterhead{PaymentText: bissendorfPaymentText}, documents[:1]),
		"every invoice has its number")
}

// A family that pays nothing gets no payment date and no request to pay;
// a run without free text, emergency numbers or contact leaves their parts
// out.
func TestConfirmationLetterWithoutAmountAndOptionalParts(t *testing.T) {
	t.Parallel()

	content := goldenLetterContent(t)
	content.document.TotalCents = 0
	content.document.Children = []parentbilling.BillingChild{{FirstName: "Lina", Positions: []parentbilling.BillingPosition{
		{Kind: parentbilling.LineBase, AmountCents: 5000, TimeModelName: "Vormittag"},
		{Kind: parentbilling.LineBenefitsExemption, AmountCents: -5000, Note: "Bürgergeld"},
	}}}
	content.input = parentbilling.BillingLetterInput{PaymentDueOn: *letterDate(t, "2026-12-15")}
	content.letterhead.ContactName, content.letterhead.ContactPhone, content.letterhead.ContactEmail = "", "", ""
	content.letterhead.BankAccount.BIC = ""

	letter := confirmationLetter(content)
	assert.Equal(t, []LetterFact{{Label: "Rechnungsnummer", Value: "FE-2026-0007"}, {Label: "Datum", Value: "01.12.2026"}}, letter.Facts)
	assert.Equal(t, []LetterBlock{
		{Text: "wir bestätigen die Anmeldung von Lina für „Winterferien 2027“. Sie haben gebucht:"},
		{Facts: []LetterFact{
			{Label: "Lina: Vormittag", Value: "50,00 €"},
			{Label: "Lina: Befreiung bei Leistungsbezug", Value: "-50,00 €"},
			{Label: "Gesamtbetrag", Value: "0,00 €", Bold: true},
		}},
		// The payment paragraph of the template is bold, so the sentence
		// that nothing is to be paid is too (#4677).
		{Text: "Sie müssen für diese Betreuung nichts bezahlen.", Bold: true},
	}, letter.Blocks)
	assert.Equal(t, "Bankverbindung: Ganztag im Quartier e.V. · IBAN DE02 1203 0000 0000 2020 51 · Beispielbank", letter.Footer[1])
}

func TestFormatEuroGroupsThousands(t *testing.T) {
	t.Parallel()
	for cents, want := range map[int64]string{0: "0,00 €", 5: "0,05 €", 123456: "1.234,56 €", -3250: "-32,50 €", 100000000: "1.000.000,00 €"} {
		assert.Equal(t, want, formatEuro(cents))
	}
}

func TestChildNamesJoinWithUnd(t *testing.T) {
	t.Parallel()
	children := func(names ...string) []parentbilling.BillingChild {
		result := make([]parentbilling.BillingChild, len(names))
		for i, name := range names {
			result[i] = parentbilling.BillingChild{FirstName: name}
		}
		return result
	}
	assert.Equal(t, "Lina", childNames(children("Lina")))
	assert.Equal(t, "Lina und Ben", childNames(children("Lina", "Ben")))
	assert.Equal(t, "Lina, Ben und Mia", childNames(children("Lina", "Ben", "Mia")))
}

func TestLetterInputNeedsAPaymentDateAndCompleteEmergencyNumbers(t *testing.T) {
	t.Parallel()
	today := *letterDate(t, "2026-12-01")
	long := make([]rune, parentbilling.MaxLetterNotesLength+1)
	for i := range long {
		long[i] = 'a'
	}
	for name, input := range map[string]parentbilling.BillingLetterInput{
		"no payment date":     {},
		"payment in the past": {PaymentDueOn: *letterDate(t, "2026-11-30")},
		"notes too long":      {PaymentDueOn: today, Notes: string(long)},
		"contact without number": {PaymentDueOn: today, EmergencyContacts: []parentbilling.BillingEmergencyContact{
			{Name: "OGS Am Lindenhof"},
		}},
		"too many contacts": {PaymentDueOn: today, EmergencyContacts: make([]parentbilling.BillingEmergencyContact, parentbilling.MaxLetterEmergencyContacts+1)},
	} {
		_, err := normalizeLetterInput(input, today)
		assert.Error(t, err, name)
	}
	normalized, err := normalizeLetterInput(parentbilling.BillingLetterInput{
		PaymentDueOn: today, Notes: "  Packliste\r\nHausschuhe  ",
		EmergencyContacts: []parentbilling.BillingEmergencyContact{{Name: " OGS ", Phone: " 0170 1 "}},
	}, today)
	assert.NoError(t, err)
	assert.Equal(t, "Packliste\nHausschuhe", normalized.Notes)
	assert.Equal(t, []parentbilling.BillingEmergencyContact{{Name: "OGS", Phone: "0170 1"}}, normalized.EmergencyContacts)
}

// The Träger's own template replaces subject, salutation, body and closing
// and fills its placeholders with the invoice; address field, facts and
// footer stay (#4677).
func TestConfirmationLetterFollowsTheTemplateOfTheTraeger(t *testing.T) {
	t.Parallel()

	content := goldenLetterContent(t)
	content.letterhead.Template = parentbilling.LetterTemplate{
		Version:    3,
		Subject:    "Ihre Anmeldung {{rechnungsnummer}}",
		Salutation: "Liebe Familie {{familienname}},",
		Body: "{{kinder}} ist angemeldet. Summe {{summe}}, zahlbar bis {{zahlen_bis}}.\n\n{{positionen}}\n\n" +
			"# Notfall\n{{notfallnummern}}\n\n# Hinweise\n{{hinweise}}",
		Closing: "Viele Grüße\n{{ansprechperson}}",
	}
	content.input.Notes = ""

	letter := confirmationLetter(content)
	assert.Equal(t, "Ihre Anmeldung FE-2026-0007", letter.Subject)
	assert.Equal(t, "Liebe Familie Beispiel,", letter.Salutation)
	assert.Equal(t, []string{"Viele Grüße", "Paula Kern"}, letter.Closing)
	assert.Equal(t, "Lina und Ben ist angemeldet. Summe 92,50 €, zahlbar bis 15.12.2026.", letter.Blocks[0].Text)
	assert.Len(t, letter.Blocks[1].Facts, 5)
	assert.Equal(t, LetterBlock{Heading: "Notfall", Facts: []LetterFact{
		{Label: "OGS Am Lindenhof", Value: "0170 0000001"},
		{Label: "OGS Schule am Hang", Value: "0170 0000002"},
	}}, letter.Blocks[2])
	// Without free text the paragraph "Hinweise" is left out.
	assert.Len(t, letter.Blocks, 3)
	assert.Equal(t, []string{"Familie Beispiel", "Erika Beispiel"}, letter.Recipient)
	assert.Equal(t, "Rechnungsnummer", letter.Facts[0].Label)
}

// The letter fills every placeholder its kind offers the Träger, so no
// placeholder in a Träger's template stays empty by accident (#4677).
func TestConfirmationLetterFillsEveryPlaceholderOfItsKind(t *testing.T) {
	t.Parallel()
	content := goldenLetterContent(t)
	assert.Empty(t, missingLetterValuesOf(templateKindOf(parentbilling.DocumentKindConfirmation), letterValues(content)))
	// Values the invoice does not have stay present and empty.
	content.document.TotalCents, content.document.IssuedOn = 0, nil
	assert.Empty(t, missingLetterValuesOf(templateKindOf(parentbilling.DocumentKindConfirmation), letterValues(content)))
	// The check itself notices a placeholder without a value.
	assert.Contains(t, missingLetterValuesOf(templateKindOf(parentbilling.DocumentKindConfirmation), nil), "familienname")
}

// TestLetterheadTextsPrintLineByLineAndNothingWhenEmpty holds #5693: every
// entered line of sender and footer is one line of the letter, and an empty
// field prints nothing. The grey example of the settings form is only a
// placeholder; it never reaches the letter.
func TestLetterheadTextsPrintLineByLineAndNothingWhenEmpty(t *testing.T) {
	t.Parallel()

	content := goldenLetterContent(t)
	content.letterhead.SenderText = "Förderverein OGS e. V.\r\n  Schulstraße 5 \n\n49143 Bissendorf\n"
	content.letterhead.FooterText = "Vorstand: Anna Brandt\nAmtsgericht Osnabrück VR 200123"
	letter := confirmationLetter(content)
	assert.Equal(t, []string{"Förderverein OGS e. V.", "Schulstraße 5", "49143 Bissendorf"}, letter.Sender)
	assert.Equal(t, []string{"Vorstand: Anna Brandt", "Amtsgericht Osnabrück VR 200123"}, letter.Footer[:2])

	content.letterhead.SenderText, content.letterhead.FooterText = "", " \n "
	letter = confirmationLetter(content)
	assert.Empty(t, letter.Sender)
	assert.Equal(t, []string{"Mit freundlichen Grüßen"}, letter.Closing)
	require.Len(t, letter.Footer, 1, "only the bank account stays")
	assert.Contains(t, letter.Footer[0], "Bankverbindung")
	for _, line := range append(letter.Footer, letter.Closing...) {
		assert.NotContains(t, line, "Muster")
	}
}

// The layout of the Träger's template decides how the table of amounts lists
// the positions; amounts, discount, correction and total stay (#5691). The
// debtor number fills its placeholder.
func TestConfirmationLetterPrintsTheTableInTheLayoutOfTheTemplate(t *testing.T) {
	t.Parallel()
	content := goldenLetterContent(t)
	content.document.DebtorNumber = "10234"
	content.letterhead.Template = parentbilling.LetterTemplate{
		Version: 4, Subject: "Rechnung {{rechnungsnummer}}", Body: "Debitorennummer {{debitorennummer}}\n\n{{positionen}}",
		GroupByChild: true, HideTimeModels: true,
	}

	letter := confirmationLetter(content)
	require.Len(t, letter.Blocks, 2)
	assert.Equal(t, "Debitorennummer 10234", letter.Blocks[0].Text)
	assert.Equal(t, []LetterFact{
		{Label: "Lina", Bold: true},
		{Label: "Woche 04.01. bis 08.01.2027", Value: "50,00 €"},
		{Label: "Summe für Lina", Value: "50,00 €"},
		{Label: "Ben", Bold: true},
		{Label: "Woche 04.01. bis 08.01.2027", Value: "65,00 €"},
		{Label: "Ermäßigung für Geschwister", Value: "-32,50 €"},
		{Label: "Korrektur: Zusätzlicher Ausflug", Value: "20,00 €"},
		{Label: "Summe für Ben", Value: "52,50 €"},
		{Label: "Gesamtbetrag", Value: "92,50 €", Bold: true},
	}, letter.Blocks[1].Facts)
}
