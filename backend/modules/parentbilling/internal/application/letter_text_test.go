package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
//	«Anschrift», «PLZ_Ort»               → none: a holiday registration has no
//	                                         postal address (ADR 0121); the
//	                                         person who registered instead
//	Infoblock (Sachbearbeiterin, phone)  → Facts Ansprechperson, Telefon, E-Mail
//	Datum                                → Facts Datum (issue date of the invoice)
//	Betreff                              → Subject with the period
//	"Sehr geehrte Familie «Familienname»"→ Salutation
//	«Vorname_Kind»                       → the first names in the confirmation
//	«Gesamtbeitrag»                      → Gesamtbetrag, after every position
//	«Adressnummer» as payment reference  → the invoice number
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
		{Text: "Sie müssen für diese Betreuung nichts bezahlen."},
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
