package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func familyDocument(id int64, key, kind string, total int64, reference int64) parentbilling.BillingFamilyDocument {
	document := parentbilling.BillingFamilyDocument{
		ID: id, RunID: 1, Kind: kind, FamilyKey: key, TotalCents: total, RecipientEmail: key,
		IssuedOn: calendar.NewDate(2027, 1, int(id)),
	}
	if reference > 0 {
		document.ReferenceDocumentID = &reference
	}
	return document
}

// TestFamiliesOfSumEveryStateOfAnInvoice covers each state: open, partly
// credited, credited in full, cancelled, and nothing due because a cost
// bearer pays or the exemption freed it; and that open is issued less
// cancelled less credited.
func TestFamiliesOfSumEveryStateOfAnInvoice(t *testing.T) {
	t.Parallel()
	bearer := familyDocument(6, "a", parentbilling.DocumentInvoice, 0, 0)
	bearer.CostBearerNames = []string{"Jugendamt Enzkreis"}
	exempt := familyDocument(7, "b", parentbilling.DocumentInvoice, 0, 0)
	exempt.Exempt = true
	exempt.DebtorNumber = "D-2"
	documents := []parentbilling.BillingFamilyDocument{
		familyDocument(1, "a", parentbilling.DocumentInvoice, 10000, 0),
		familyDocument(2, "a", parentbilling.DocumentInvoice, 8000, 0),
		familyDocument(3, "a", parentbilling.DocumentInvoice, 5000, 0),
		familyDocument(4, "b", parentbilling.DocumentInvoice, 4000, 0),
		familyDocument(5, "a", parentbilling.DocumentCredit, -3000, 2),
		bearer,
		exempt,
		familyDocument(8, "a", parentbilling.DocumentCredit, -5000, 3),
		familyDocument(9, "b", parentbilling.DocumentCancellation, -4000, 4),
	}

	families := familiesOf(documents)
	require.Len(t, families, 2)
	a, b := families[0], families[1]
	assert.EqualValues(t, 1, a.ID, "a family is named by its first document")
	assert.Equal(t, parentbilling.BillingAmounts{IssuedCents: 23000, CreditedCents: 8000, OpenCents: 15000}, a.BillingAmounts)
	assert.Equal(t, a.IssuedCents-a.CancelledCents-a.CreditedCents, a.OpenCents)
	assert.Equal(t, 4, a.InvoiceCount)
	assert.Equal(t, 2, a.OpenInvoiceCount)
	assert.Equal(t, 6, a.DocumentCount)
	assert.Equal(t, calendar.NewDate(2027, 1, 8), a.LastIssuedOn)
	assert.Equal(t, []parentbilling.BillingPayment{
		{Kind: parentbilling.PaymentFamily},
		{Kind: parentbilling.PaymentCostBearer, CostBearerName: "Jugendamt Enzkreis"},
	}, a.Payments)

	states := map[int64]string{}
	open := map[int64]int64{}
	for _, document := range append(a.Documents, b.Documents...) {
		states[document.ID], open[document.ID] = document.Status, document.OpenCents
		if document.Kind != parentbilling.DocumentInvoice {
			assert.Nil(t, document.Payment, "only an invoice has a payer")
		}
	}
	assert.Equal(t, map[int64]string{
		1: parentbilling.InvoiceOpen, 2: parentbilling.InvoicePartlyCredited, 3: parentbilling.InvoiceCredited,
		4: parentbilling.InvoiceCancelled, 5: "", 6: parentbilling.InvoiceNothingDue, 7: parentbilling.InvoiceNothingDue, 8: "", 9: "",
	}, states)
	assert.Equal(t, map[int64]int64{1: 10000, 2: 5000, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0}, open)

	assert.Equal(t, parentbilling.BillingAmounts{IssuedCents: 4000, CancelledCents: 4000}, b.BillingAmounts)
	assert.Equal(t, []string{"D-2"}, b.DebtorNumbers)
	assert.Equal(t, []parentbilling.BillingPayment{{Kind: parentbilling.PaymentExempt}}, b.Payments,
		"a cancelled invoice names no payer")
	assert.Zero(t, b.OpenInvoiceCount)
}

func TestFamiliesOfIsEmptyWithoutDocuments(t *testing.T) {
	t.Parallel()
	assert.Empty(t, familiesOf(nil))
}
