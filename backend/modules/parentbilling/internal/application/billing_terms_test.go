package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A holiday invoice takes the debtor number of its registration; without
// one it takes the number of the family, as the monthly invoice does (#6677).
// The family key is the normalized e-mail of the registration.
func TestHolidayInvoiceFallsBackToTheDebtorNumberOfTheFamily(t *testing.T) {
	t.Parallel()
	payments := invoicePayments{terms: parentbilling.TermsSet{}, run: parentbilling.RunDueDates{},
		families: map[string]*parentbilling.FamilyBilling{
			"eva.muster@example.org": {ID: 7, FamilyEmail: "eva.muster@example.org", RecipientKind: parentbilling.RecipientApplicant, DebtorNumber: "20 815"},
		},
		used: map[int64]parentbilling.FamilyBilling{}, mandates: map[string]*parentbilling.SEPAMandate{}, usedMandates: map[int64]parentbilling.SEPAMandate{}}
	issuedOn := calendar.NewDate(2026, 11, 2)

	for name, tc := range map[string]struct{ email, registration, want string }{
		"registration without number": {"EVA.Muster@example.org", "", "20 815"},
		"registration number wins":    {"eva.muster@example.org", "10 4711", "10 4711"},
		"family without deviation":    {"jan@example.org", "", ""},
	} {
		document := parentbilling.BillingDocument{Kind: parentbilling.DocumentInvoice, RecipientFirstName: "Eva", RecipientLastName: "Muster",
			RecipientEmail: tc.email, DebtorNumber: tc.registration}
		payments.holidayDocument(&document, issuedOn)
		assert.Equal(t, tc.want, document.DebtorNumber, name)
	}
}
