package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A direct debit never prints the payment text, so a payment text naming
// the debtor number does not hold its letter back (#4689); a transfer still
// needs the number.
func TestDirectDebitNeedsNoDebtorNumberForThePaymentText(t *testing.T) {
	t.Parallel()
	text := "Bitte überweisen Sie {Betrag} mit Ihrer Debitorennummer {Debitorennummer}."
	due := calendar.NewDate(2026, 12, 4)
	debit := parentbilling.BillingDocument{TotalCents: 4500,
		Payment: &parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit, DueOn: due, DueSource: parentbilling.DueFromRun}}
	transfer := parentbilling.BillingDocument{TotalCents: 4500,
		Payment: &parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: due, DueSource: parentbilling.DueFromRun}}
	assert.False(t, lacksDebtorNumberForPaymentText(text, debit))
	assert.True(t, lacksDebtorNumberForPaymentText(text, transfer))
	assert.True(t, lacksDebtorNumberForPaymentText(text, parentbilling.BillingDocument{TotalCents: 4500}),
		"an invoice issued before #4689 keeps the check")
}
