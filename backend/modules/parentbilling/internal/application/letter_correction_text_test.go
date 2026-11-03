package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// correctionContent is the golden invoice with a document of kind that
// references it.
func correctionContent(t *testing.T, kind string) letterContent {
	t.Helper()
	content := goldenLetterContent(t)
	invoice := content.document
	var document parentbilling.BillingDocument
	switch kind {
	case parentbilling.DocumentCancellation:
		document = cancellationOf(invoice, "Anschrift falsch, neue Rechnung folgt")
	default:
		document = creditOf(invoice, invoice.Children[1], parentbilling.BillingCreditInput{AmountCents: 2000, Reason: "Ausflug fiel aus"})
	}
	document.ID, document.Number, document.IssuedOn = 12, "FE-2026-0012", letterDate(t, "2026-12-20")
	content.document, content.invoice = document, &invoice
	content.leftCents = leftOf(invoice, []parentbilling.BillingDocument{document})
	return content
}

func letterText(letter Letter) string {
	parts := []string{letter.Title, letter.Subject, letter.Salutation}
	for _, fact := range letter.Facts {
		parts = append(parts, fact.Label+": "+fact.Value)
	}
	for _, block := range letter.Blocks {
		parts = append(parts, block.Text)
		for _, fact := range block.Facts {
			parts = append(parts, fact.Label+": "+fact.Value)
		}
	}
	return strings.Join(parts, "\n")
}

// TestCancellationLetterNamesItsInvoiceAndAsksForNoPayment pins the letter
// of a Storno (#4690): its number, the invoice with its date, the reason,
// every position with its minus and no payment request.
func TestCancellationLetterNamesItsInvoiceAndAsksForNoPayment(t *testing.T) {
	t.Parallel()
	content := correctionContent(t, parentbilling.DocumentCancellation)
	letter := cancellationLetter(content)
	text := letterText(letter)
	assert.Equal(t, "Storno FE-2026-0012 zur Rechnung FE-2026-0007", letter.Title)
	for _, want := range []string{
		"Storno-Nummer: FE-2026-0012", "Zur Rechnung: FE-2026-0007", "Datum: 20.12.2026",
		"wir stornieren die Rechnung FE-2026-0007 vom 01.12.2026 für „Winterferien 2027“. Grund: Anschrift falsch, neue Rechnung folgt.",
		"Lina: Woche 04.01. bis 08.01.2027, Vormittag: -50,00 €", "Ben: Ermäßigung für Geschwister: 32,50 €",
		"Storno gesamt: -102,50 €", "Diese Rechnung gilt nicht mehr. Sie müssen sie nicht bezahlen.",
		"Bei Fragen erreichen Sie Paula Kern",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "Zahlen bis")
	assert.NotContains(t, text, "überweisen")
	assert.Equal(t, []string{"Familie Beispiel", "Erika Beispiel"}, letter.Recipient)
}

// TestCreditLetterNamesWhatTheInvoiceStillAsksFor pins the letter of a
// Gutschrift (#4690): the credited amount with its reason and what the
// invoice asks for after it.
func TestCreditLetterNamesWhatTheInvoiceStillAsksFor(t *testing.T) {
	t.Parallel()
	content := correctionContent(t, parentbilling.DocumentCredit)
	letter := creditLetter(content)
	text := letterText(letter)
	assert.Equal(t, "Gutschrift FE-2026-0012 zur Rechnung FE-2026-0007", letter.Title)
	for _, want := range []string{
		"Gutschrift-Nummer: FE-2026-0012",
		"zur Rechnung FE-2026-0007 vom 01.12.2026 für „Winterferien 2027“ schreiben wir Ihnen 20,00 € gut. Grund: Ausflug fiel aus.",
		"Ben: Gutschrift: -20,00 €", "Gutschrift gesamt: -20,00 €",
		"Für die Rechnung FE-2026-0007 müssen Sie jetzt noch 72,50 € bezahlen.",
	} {
		assert.Contains(t, text, want)
	}

	content.leftCents = 0
	assert.Contains(t, letterText(creditLetter(content)), "Für die Rechnung FE-2026-0007 müssen Sie nichts mehr bezahlen.")
}

// TestRunLetterContentFindsTheInvoiceOfACorrection counts what an invoice
// asks for after the credits up to the letter, not after later ones.
func TestRunLetterContentFindsTheInvoiceOfACorrection(t *testing.T) {
	t.Parallel()
	invoice := goldenLetterContent(t).document
	first := creditOf(invoice, invoice.Children[0], parentbilling.BillingCreditInput{AmountCents: 1000, Reason: "a"})
	second := creditOf(invoice, invoice.Children[1], parentbilling.BillingCreditInput{AmountCents: 500, Reason: "b"})
	first.ID, second.ID = 20, 21
	documents := []parentbilling.BillingDocument{invoice, first, second}
	content := runLetterContent(parentbilling.BillingRun{}, documents, first, parentbilling.BillingLetterInput{}, parentbilling.Letterhead{})
	require.NotNil(t, content.invoice)
	assert.Equal(t, invoice.ID, content.invoice.ID)
	assert.Equal(t, int64(8250), content.leftCents)
	assert.Equal(t, int64(7750), runLetterContent(parentbilling.BillingRun{}, documents, second, parentbilling.BillingLetterInput{}, parentbilling.Letterhead{}).leftCents)
	assert.Nil(t, runLetterContent(parentbilling.BillingRun{}, documents, invoice, parentbilling.BillingLetterInput{}, parentbilling.Letterhead{}).invoice)
}

// TestLetterMailOfACorrectionSaysWhatItIs gives the mail of a Storno and a
// Gutschrift their own subject and text (#4690); an invoice keeps its mail.
func TestLetterMailOfACorrectionSaysWhatItIs(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]struct{ subject, template string }{
		"":                                 {"Bestätigung und Rechnung: Winterferien 2027", "billing-letter.html"},
		parentbilling.DocumentInvoice:      {"Bestätigung und Rechnung: Winterferien 2027", "billing-letter.html"},
		parentbilling.DocumentCancellation: {"Storno Ihrer Rechnung: Winterferien 2027", "billing-correction-letter.html"},
		parentbilling.DocumentCredit:       {"Gutschrift zu Ihrer Rechnung: Winterferien 2027", "billing-correction-letter.html"},
	} {
		payload, err := json.Marshal(parentbilling.BillingLetterMail{
			PeriodName: "Winterferien 2027", Number: "FE-0003", DocumentKind: kind, Filename: "storno-fe-0003.pdf", PDF: []byte("%PDF"),
		})
		require.NoError(t, err)
		mail, err := RenderLetterMail(context.Background(), payload)
		require.NoError(t, err)
		assert.Equal(t, want.subject, mail.Subject, kind)
		assert.Equal(t, want.template, mail.Template, kind)
	}
}
