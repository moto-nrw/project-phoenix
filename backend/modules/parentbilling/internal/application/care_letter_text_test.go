package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careLetterSJR is the SJR invoice of ADR 0158: Monday until 14:00 and
// Tuesday until 16:30 for the whole month, Wednesday free.
func careLetterSJR() careLetterContent {
	issued := calendar.NewDate(2027, 1, 4)
	return careLetterContent{
		run: parentbilling.CareRun{BillingMonth: calendar.NewDate(2027, 1, 1)},
		invoice: parentbilling.CareInvoice{
			ID: 7, Number: "MB-2027-0001", IssuedOn: &issued, ContractID: 3, SchoolName: "Grundschule Ispringen",
			ChildFirstName: "Mia", ChildLastName: "Beispiel",
			Recipient: parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: "eva@example.org"},
			Positions: []parentbilling.CarePosition{
				{ModuleName: "Betreuung bis 14:00 Uhr", ServiceFrom: calendar.NewDate(2027, 1, 1), ServiceUntil: calendar.NewDate(2027, 1, 31),
					Weekdays: []parentbilling.CareWeekdayPrice{{Weekday: "mon", MonthlyCents: 3200, AmountCents: 3200}}, AmountCents: 3200},
				{ModuleName: "Betreuung bis 16:30 Uhr", ServiceFrom: calendar.NewDate(2027, 1, 1), ServiceUntil: calendar.NewDate(2027, 1, 15),
					Weekdays: []parentbilling.CareWeekdayPrice{{Weekday: "tue", MonthlyCents: 4550, AmountCents: 2201}}, AmountCents: 2201},
			},
			TotalCents: 5401,
		},
		input: parentbilling.CareLetterInput{PaymentDueOn: calendar.NewDate(2027, 1, 15), Notes: "Die Betreuung ist am 2. Januar geschlossen."},
		letterhead: parentbilling.Letterhead{
			SenderText: "SJR Pforzheim\nWestliche 1\n75172 Pforzheim", ContactName: "Ute Muster", ContactPhone: "07231 1234",
			BankAccount: &parentbilling.LetterheadBankAccount{AccountHolder: "SJR Pforzheim", IBAN: "DE02120300000000202051"},
		},
	}
}

func careLetterText(letter Letter) string {
	var parts []string
	parts = append(parts, letter.Title, letter.Subject, letter.Salutation)
	for _, fact := range letter.Facts {
		parts = append(parts, fact.Label+": "+fact.Value)
	}
	for _, block := range letter.Blocks {
		parts = append(parts, block.Heading, block.Text)
		for _, fact := range block.Facts {
			parts = append(parts, fact.Label+" = "+fact.Value)
		}
	}
	return strings.Join(append(parts, letter.Footer...), "\n")
}

func TestCareInvoiceLetterNamesOneLinePerModuleAndAsksForPayment(t *testing.T) {
	t.Parallel()
	text := careLetterText(careInvoiceLetter(careLetterSJR()))
	for _, want := range []string{
		"Rechnung MB-2027-0001: Monatsbeitrag Januar 2027",
		"Sehr geehrte Familie Beispiel,",
		"Kind: Mia Beispiel", "Monat: Januar 2027", "Datum: 04.01.2027", "Zahlen bis: 15.01.2027", "Ansprechperson: Ute Muster",
		"für die Betreuung von Mia Beispiel an der Grundschule Ispringen",
		"Betreuung bis 14:00 Uhr (Mo) = 32,00 €",
		"Betreuung bis 16:30 Uhr (Di), 01.01. bis 15.01.2027 = 22,01 €",
		"Summe = 54,01 €",
		"Ihre Betreuung", "Abgerechnet wird nach Ihrem Vertrag, nicht nach Anwesenheit.",
		"Bitte überweisen Sie 54,01 € bis zum 15.01.2027 auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer MB-2027-0001 an.",
		"Hinweise", "Die Betreuung ist am 2. Januar geschlossen.",
		"Bankverbindung: SJR Pforzheim · IBAN DE02 1203 0000 0000 2020 51",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "02.01.", "no line per calendar day")
	for _, block := range careInvoiceLetter(careLetterSJR()).Blocks {
		if block.Heading != "" {
			assert.NotEmpty(t, block.Text, "a heading needs its paragraph, or the layout lays the table over it")
		}
	}
}

func TestCarePaymentTextUsesTheTraegersTextUnlessItNeedsADebtorNumber(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.letterhead.PaymentText = "Bitte zahlen Sie {Betrag} bis {Zahlungsfrist}, Zweck {Rechnungsnummer}."
	assert.Equal(t, "Bitte zahlen Sie 54,01 € bis 15.01.2027, Zweck MB-2027-0001.", carePaymentText(content))
	content.letterhead.PaymentText = "Zweck: {Debitorennummer}"
	assert.Contains(t, carePaymentText(content), "die Rechnungsnummer MB-2027-0001", "no gap where the debtor number belongs")
	content.invoice.TotalCents = 0
	assert.Equal(t, "Sie müssen für diesen Monat nichts bezahlen.", carePaymentText(content))
}

func TestCareCancellationLetterAsksForNothing(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.cancellation = &parentbilling.CareCancellation{
		Number: "MB-2027-0009", IssuedOn: calendar.NewDate(2027, 1, 10), Reason: "Falscher Empfänger", TotalCents: -5401,
	}
	text := careLetterText(careCancellationLetter(content))
	for _, want := range []string{
		"Storno MB-2027-0009 zur Rechnung MB-2027-0001",
		"Storno zur Rechnung MB-2027-0001: Monatsbeitrag Januar 2027",
		"Storno-Nummer: MB-2027-0009", "Zur Rechnung: MB-2027-0001", "Datum: 10.01.2027",
		"wir stornieren die Rechnung MB-2027-0001 vom 04.01.2027 für Monatsbeitrag Januar 2027. Grund: Falscher Empfänger.",
		"Betreuung bis 14:00 Uhr (Mo) = -32,00 €", "Storno gesamt = -54,01 €",
		"Diese Rechnung gilt nicht mehr. Sie müssen sie nicht bezahlen.",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "Zahlen bis")
	assert.NotContains(t, text, "Bitte überweisen")
}

func TestCareMailOfAMonthlyLetter(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string][2]string{
		parentbilling.MailDocumentCareInvoice:      {"Ihre Rechnung: Monatsbeitrag Januar 2027", "billing-care-letter.html"},
		parentbilling.MailDocumentCareCancellation: {"Storno Ihrer Rechnung: Monatsbeitrag Januar 2027", "billing-correction-letter.html"},
	} {
		payload := `{"document_kind":"` + kind + `","period_name":"Monatsbeitrag Januar 2027","number":"MB-2027-0001",` +
			`"traeger_name":"SJR Pforzheim","filename":"rechnung.pdf","pdf":"JVBERi0="}`
		mail, err := RenderLetterMail(t.Context(), []byte(payload))
		require.NoError(t, err)
		assert.Equal(t, want[0], mail.Subject)
		assert.Equal(t, want[1], mail.Template)
		assert.Equal(t, "SJR Pforzheim", mail.SenderName)
	}
}

func TestCareLetterSummaryCountsWhatTheRunMayStillSend(t *testing.T) {
	t.Parallel()
	summary := careSummary([]parentbilling.CareLetterMailing{
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Status: parentbilling.LetterMailSent, Sends: 1},
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Status: parentbilling.LetterMailQueued, Sends: 1},
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Status: parentbilling.LetterMailFailed, Sends: 1},
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Status: parentbilling.LetterMailNotSent},
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Status: parentbilling.LetterMailNotSent, RecipientEmail: "a@example.org"},
		// A cancelled invoice never sent is void; its Storno has no letter.
		{Kind: parentbilling.CareLetterInvoice, HasLetter: true, Cancelled: true, Status: parentbilling.LetterMailNotSent},
		{Kind: parentbilling.CareLetterCancellation, Cancelled: true},
	})
	assert.Equal(t, parentbilling.CareLetterSummary{Letters: 5, Sent: 1, Queued: 1, Failed: 1, NotSent: 2, WithoutEmail: 1}, summary)
}
