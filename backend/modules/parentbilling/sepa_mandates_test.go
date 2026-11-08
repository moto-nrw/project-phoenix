package parentbilling_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	sjrCreditor = parentbilling.SEPACreditor{CreditorID: "DE98ZZZ09999999999", PrenotificationDays: 3,
		MissingMandate: parentbilling.MissingMandateTransfer}
	sjrMandate = &parentbilling.SEPAMandate{ID: 4, Reference: "SJR-1", SignedOn: calendar.NewDate(2026, 9, 1), CreditorID: "DE98ZZZ09999999999",
		Kind: parentbilling.MandateRecurring, Sequence: parentbilling.SequenceRecurring, IBANMasked: "DE•• •••• •••• •••• ••30 00"}
	sjrDebitTerms = parentbilling.ServiceTerms{Service: parentbilling.ServiceCare, PaymentMethod: parentbilling.PaymentDirectDebit, DueDays: 14}
)

func debitOf(creditor parentbilling.SEPACreditor, mandate *parentbilling.SEPAMandate, issuedOn calendar.Date) parentbilling.InvoicePayment {
	payment := parentbilling.ResolvePayment(sjrDebitTerms, sjrRun, nil, issuedOn)
	return parentbilling.ResolveDebit(payment, sjrDebitTerms, sjrRun, nil, issuedOn, creditor, mandate)
}

func TestResolveDebitCarriesTheMandateInForce(t *testing.T) {
	t.Parallel()
	got := debitOf(sjrCreditor, sjrMandate, sjrIssued)
	assert.Equal(t, parentbilling.PaymentDirectDebit, got.Method)
	assert.Equal(t, sjrDebit, got.DueOn, "the debit date stays the run's date for direct debits")
	assert.Equal(t, &parentbilling.InvoiceMandate{MandateID: 4, Reference: "SJR-1", SignedOn: calendar.NewDate(2026, 9, 1),
		CreditorID: "DE98ZZZ09999999999", IBANMasked: "DE•• •••• •••• •••• ••30 00", SequenceType: parentbilling.SequenceTypeRecurring}, got.Mandate)
	assert.Empty(t, got.DebitGap)
}

// Without a mandate the invoice falls back to a transfer with the run's
// transfer date and names the gap; under "block" it stays a direct debit
// the release refuses.
func TestResolveDebitWithoutMandateFallsBackOrBlocks(t *testing.T) {
	t.Parallel()
	revoked := *sjrMandate
	revokedOn := calendar.NewDate(2026, 10, 1)
	revoked.RevokedOn = &revokedOn
	signedLater := *sjrMandate
	signedLater.SignedOn = calendar.NewDate(2026, 11, 1)
	for name, mandate := range map[string]*parentbilling.SEPAMandate{"none": nil, "revoked": &revoked, "signed after the invoice": &signedLater} {
		got := debitOf(sjrCreditor, mandate, sjrIssued)
		assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: sjrTransfer, DueSource: parentbilling.DueFromRun,
			DebitGap: parentbilling.DebitGapMandate}, got, name)
	}
	blocking := sjrCreditor
	blocking.MissingMandate = parentbilling.MissingMandateBlock
	got := debitOf(blocking, nil, sjrIssued)
	assert.Equal(t, parentbilling.PaymentDirectDebit, got.Method)
	assert.Equal(t, parentbilling.DebitGapMandate, got.DebitGap)
	assert.Nil(t, got.Mandate)

	noCreditor := sjrCreditor
	noCreditor.CreditorID = ""
	assert.Equal(t, parentbilling.DebitGapCreditor, debitOf(noCreditor, sjrMandate, sjrIssued).DebitGap,
		"without the creditor identifier no mandate can be used")
}

func TestResolveDebitFlagsAShortPrenotification(t *testing.T) {
	t.Parallel()
	assert.False(t, debitOf(sjrCreditor, sjrMandate, sjrIssued).Mandate.NoticeShort, "30.10. to 04.11. is five days, three are asked")
	long := sjrCreditor
	long.PrenotificationDays = parentbilling.DefaultPrenotificationDays
	assert.True(t, debitOf(long, sjrMandate, sjrIssued).Mandate.NoticeShort, "five days are less than fourteen")
}

func TestResolveDebitLeavesATransferAlone(t *testing.T) {
	t.Parallel()
	transfer := parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: sjrTransfer, DueSource: parentbilling.DueFromRun}
	assert.Equal(t, transfer, parentbilling.ResolveDebit(transfer, sjrDebitTerms, sjrRun, nil, sjrIssued, sjrCreditor, nil))
}

func TestMandateSequenceType(t *testing.T) {
	t.Parallel()
	for want, mandate := range map[string]parentbilling.SEPAMandate{
		parentbilling.SequenceTypeFirst:     {Kind: parentbilling.MandateRecurring, Sequence: parentbilling.SequenceFirst},
		parentbilling.SequenceTypeRecurring: {Kind: parentbilling.MandateRecurring, Sequence: parentbilling.SequenceRecurring},
		parentbilling.SequenceTypeOneOff:    {Kind: parentbilling.MandateOneOff, Sequence: parentbilling.SequenceFirst},
	} {
		assert.Equal(t, want, mandate.SequenceType())
	}
}

// A mandate given to an earlier creditor identifier is no proof for the
// current one: the invoice falls back like one without mandate.
func TestResolveDebitNeedsTheMandatesCreditor(t *testing.T) {
	t.Parallel()
	changed := sjrCreditor
	changed.CreditorID = "DE98ABC09999999999"
	got := debitOf(changed, sjrMandate, sjrIssued)
	assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: sjrTransfer, DueSource: parentbilling.DueFromRun,
		DebitGap: parentbilling.DebitGapCreditorChanged}, got)
	assert.Equal(t, "DE98ZZZ09999999999", debitOf(sjrCreditor, sjrMandate, sjrIssued).Mandate.CreditorID,
		"the invoice carries the creditor the mandate was given to")
}
