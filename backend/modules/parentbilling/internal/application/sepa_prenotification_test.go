package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// sepaInvoice is the SJR invoice collected by direct debit with its
// mandate (#4695, ADR 0171).
func sepaInvoice() careLetterContent {
	content := careLetterSJR()
	content.invoice.Payment = &parentbilling.InvoicePayment{
		Method: parentbilling.PaymentDirectDebit, DueOn: calendar.NewDate(2027, 1, 18), DueSource: parentbilling.DueFromRun,
		Mandate: &parentbilling.InvoiceMandate{MandateID: 9, Reference: "SJR-2026-0042", SignedOn: calendar.NewDate(2026, 9, 1),
			CreditorID: "DE98ZZZ09999999999", IBANMasked: "DE•• •••• •••• •••• ••30 00", SequenceType: parentbilling.SequenceTypeRecurring},
	}
	return content
}

// The pre-notification names amount, debit date, the masked account, the
// creditor identifier and the mandate reference, beside the address field
// and in the payment sentence.
func TestCareLetterCarriesTheSEPAPrenotification(t *testing.T) {
	t.Parallel()
	text := careLetterText(careInvoiceLetter(sepaInvoice()))
	for _, want := range []string{
		"Zahlungsweg: Lastschrift", "Abbuchung am: 18.01.2027", "Konto: DE•• •••• •••• •••• ••30 00",
		"Mandatsreferenz: SJR-2026-0042", "Gläubiger-ID: DE98ZZZ09999999999",
		"Wir buchen 54,01 € am 18.01.2027 per SEPA-Lastschrift von Ihrem Konto DE•• •••• •••• •••• ••30 00 ab.",
		"Gläubiger-ID: DE98ZZZ09999999999. Mandatsreferenz: SJR-2026-0042. Verwendungszweck: Rechnungsnummer MB-2027-0001.",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "DE89370400440532013000", "the letter never prints the full IBAN")
	assert.NotContains(t, text, "Bitte überweisen Sie", "a direct debit asks for no transfer")
}

// The pre-notification announces what the family pays: the rest after the
// share of a cost bearer (#4687, ADR 0169), never the whole contribution.
func TestSEPAPrenotificationAnnouncesTheFamilysShareAfterTheCostBearer(t *testing.T) {
	t.Parallel()
	content := sepaInvoice()
	basis := 5000
	content.invoice.CostShares = []parentbilling.CareInvoiceCostShare{{Kind: parentbilling.CostShareYouthOffice,
		CostBearerName: "Jugendamt Pforzheim", ShareBasisPoints: &basis, AmountCents: 2700}}
	content.invoice.TotalCents = 2701
	text := careLetterText(careInvoiceLetter(content))
	assert.Contains(t, text, "Wir buchen 27,01 € am 18.01.2027 per SEPA-Lastschrift")
	assert.NotContains(t, text, "Wir buchen 54,01 €", "the cost bearer's share is not collected from the family")
}

// An invoice issued by direct debit before #4695 has no mandate and keeps
// the short sentence.
func TestDirectDebitWithoutMandateKeepsTheShortSentence(t *testing.T) {
	t.Parallel()
	content := sepaInvoice()
	content.invoice.Payment.Mandate = nil
	assert.Equal(t, "Wir buchen 54,01 € am 18.01.2027 von Ihrem Konto ab. Verwendungszweck: Rechnungsnummer MB-2027-0001. Sie müssen nichts überweisen.",
		carePaymentText(content))
}

// Only an invoice that collects money and stays a direct debit without a
// mandate holds the release back (MissingMandateBlock).
func TestDebitBlockedOnlyByAnAmountWithoutMandate(t *testing.T) {
	t.Parallel()
	gap := &parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit, DebitGap: parentbilling.DebitGapMandate}
	assert.True(t, debitBlocked(7800, gap))
	assert.False(t, debitBlocked(0, gap), "a cost bearer pays all: nothing to collect")
	assert.False(t, debitBlocked(7800, &parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer,
		DebitGap: parentbilling.DebitGapMandate}), "the fallback transfer goes out")
	assert.False(t, debitBlocked(7800, &parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit,
		Mandate: &parentbilling.InvoiceMandate{Reference: "SJR-1"}}))
	assert.False(t, debitBlocked(7800, nil))
}
