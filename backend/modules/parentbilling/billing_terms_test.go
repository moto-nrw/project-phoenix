package parentbilling_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The SJR run of October 2026 (Punkt 5): invoices dated 30.10., transfers
// due on 02.11., direct debits collected on 04.11.
var (
	sjrIssued   = calendar.NewDate(2026, 10, 30)
	sjrTransfer = calendar.NewDate(2026, 11, 2)
	sjrDebit    = calendar.NewDate(2026, 11, 4)
	sjrRun      = parentbilling.RunDueDates{parentbilling.PaymentTransfer: sjrTransfer, parentbilling.PaymentDirectDebit: sjrDebit}
)

func TestResolvePaymentTakesTheRunDateOfThePaymentMethod(t *testing.T) {
	t.Parallel()
	care := parentbilling.ServiceTerms{Service: parentbilling.ServiceCare, PaymentMethod: parentbilling.PaymentDirectDebit, DueDays: 14}
	holiday := parentbilling.ServiceTerms{Service: parentbilling.ServiceHoliday, PaymentMethod: parentbilling.PaymentTransfer, DueDays: 14}

	assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit, DueOn: sjrDebit, DueSource: parentbilling.DueFromRun},
		parentbilling.ResolvePayment(care, sjrRun, nil, sjrIssued))
	assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: sjrTransfer, DueSource: parentbilling.DueFromRun},
		parentbilling.ResolvePayment(holiday, sjrRun, nil, sjrIssued))
}

func TestResolvePaymentFallsBackToThePaymentTerm(t *testing.T) {
	t.Parallel()
	terms := parentbilling.ServiceTerms{PaymentMethod: parentbilling.PaymentDirectDebit, DueDays: 10}
	onlyTransfer := parentbilling.RunDueDates{parentbilling.PaymentTransfer: sjrTransfer}

	got := parentbilling.ResolvePayment(terms, onlyTransfer, nil, sjrIssued)
	assert.Equal(t, calendar.NewDate(2026, 11, 9), got.DueOn, "no run date for direct debit: issue date plus ten days")
	assert.Equal(t, parentbilling.DueFromTerm, got.DueSource)
	assert.Equal(t, parentbilling.PaymentTransfer, parentbilling.ResolvePayment(parentbilling.ServiceTerms{}, nil, nil, sjrIssued).Method,
		"without a method the invoice is paid by transfer")
}

func TestResolvePaymentFamilyDeviationWins(t *testing.T) {
	t.Parallel()
	terms := parentbilling.ServiceTerms{PaymentMethod: parentbilling.PaymentDirectDebit, DueDays: 14}
	day := 15
	method := &parentbilling.FamilyBilling{PaymentMethod: parentbilling.PaymentTransfer}
	dueDay := &parentbilling.FamilyBilling{DueDay: &day}

	got := parentbilling.ResolvePayment(terms, sjrRun, method, sjrIssued)
	assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentTransfer, DueOn: sjrTransfer, DueSource: parentbilling.DueFromRun}, got,
		"the family pays by transfer, so the run's transfer date holds")
	got = parentbilling.ResolvePayment(terms, sjrRun, dueDay, sjrIssued)
	assert.Equal(t, parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit, DueOn: calendar.NewDate(2026, 11, 15),
		DueSource: parentbilling.DueFromFamily}, got, "the 15th after the issue date, not the run date")
	got = parentbilling.ResolvePayment(terms, sjrRun, dueDay, calendar.NewDate(2026, 12, 10))
	assert.Equal(t, calendar.NewDate(2026, 12, 15), got.DueOn, "the 15th of the same month when it is still ahead")
	got = parentbilling.ResolvePayment(terms, nil, dueDay, calendar.NewDate(2026, 12, 20))
	assert.Equal(t, calendar.NewDate(2027, 1, 15), got.DueOn, "across the year")
}

func TestResolveRecipientKeepsTheRegistrationUnlessTheFamilyDeviates(t *testing.T) {
	t.Parallel()
	address := &parentbilling.PostalAddress{Street: "Musterweg 1", PostalCode: "75228", City: "Ispringen"}
	applicant := parentbilling.BillingRecipient{FirstName: "Eva", LastName: "Beispiel", Email: "eva@example.org", Address: address}

	got := parentbilling.ResolveRecipient(applicant, nil)
	assert.Equal(t, parentbilling.RecipientApplicant, got.Kind)
	assert.Equal(t, "eva@example.org", got.Email)

	got = parentbilling.ResolveRecipient(applicant, &parentbilling.FamilyBilling{RecipientKind: parentbilling.RecipientBoth,
		SecondFirstName: "Jan", SecondLastName: "Beispiel"})
	assert.Equal(t, parentbilling.RecipientBoth, got.Kind)
	assert.Equal(t, "Jan Beispiel", got.SecondName)
	assert.Equal(t, address, got.Address, "both parents keep the address of the registration")

	payerAddress := &parentbilling.PostalAddress{Street: "Amtsweg 2", PostalCode: "75175", City: "Pforzheim"}
	got = parentbilling.ResolveRecipient(applicant, &parentbilling.FamilyBilling{RecipientKind: parentbilling.RecipientPayer,
		PayerFirstName: "Oma", PayerLastName: "Muster", PayerEmail: "oma@example.org", PayerAddress: payerAddress})
	assert.Equal(t, parentbilling.BillingRecipient{Kind: parentbilling.RecipientPayer, FirstName: "Oma", LastName: "Muster",
		Email: "oma@example.org", Address: payerAddress}, got)
}

func TestTermsSetBundlesTheLunchWithTheCareByDefault(t *testing.T) {
	t.Parallel()
	defaults := parentbilling.TermsSet{}
	assert.Equal(t, parentbilling.ServiceCare, defaults.InvoiceService(parentbilling.ServiceMeal), "bundled: lunch on the care invoice")
	assert.Equal(t, parentbilling.ServiceHoliday, defaults.InvoiceService(parentbilling.ServiceHoliday), "the holiday care stays apart")
	assert.Equal(t, parentbilling.ServiceCare, defaults.InvoiceService(parentbilling.ServiceCare))

	separate := parentbilling.TermsSet{parentbilling.ServiceMeal: {Service: parentbilling.ServiceMeal, PaymentMethod: parentbilling.PaymentTransfer}}
	assert.Equal(t, parentbilling.ServiceMeal, separate.InvoiceService(parentbilling.ServiceMeal), "separate: an invoice of its own")
	assert.Equal(t, parentbilling.DefaultDueDays, defaults.Of(parentbilling.ServiceCare).DueDays)
}

func TestFamilyKeyIsTheTrimmedLowerCaseAddress(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "eva@example.org", parentbilling.FamilyKey("  Eva@Example.org "))
}
