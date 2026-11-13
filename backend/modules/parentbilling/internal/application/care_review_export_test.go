package application

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

func TestCareReviewTableNamesMissingScaleSteps(t *testing.T) {
	t.Parallel()
	table := careReviewTable(parentbilling.CareRunDetail{Unpriced: []parentbilling.CareUnpriced{
		{ModuleName: "Modul 4", DaysPerWeek: ptr(3)},
		{ModuleName: "Modul 1", Weekday: "mon"},
	}})
	assert.Contains(t, table.Rows[0]["position"], "3 Tage je Woche")
	assert.Equal(t, "Mo", table.Rows[1]["weekdays"])
}

func TestCareReviewTableKeepsSignedReductions(t *testing.T) {
	t.Parallel()
	table := careReviewTable(parentbilling.CareRunDetail{
		Invoices: []parentbilling.CareInvoice{{
			Positions:  []parentbilling.CarePosition{{AmountCents: 10000}},
			Reductions: []parentbilling.CareReduction{{Name: "Ermäßigung", AmountCents: -2000}},
			TotalCents: 8000,
		}},
	})
	require.Len(t, table.Rows, 3)
	assert.Equal(t, "100,00", table.Rows[0]["amount"])
	assert.Equal(t, "-20,00", table.Rows[1]["amount"], "both export formats use the signed reduction")
	assert.Equal(t, "80,00", table.Rows[2]["amount"])
}

// The review list names how and by when each invoice is paid, from the same
// rule the release and the letter use, and both parents (#4689).
func TestCareReviewTableNamesThePayment(t *testing.T) {
	t.Parallel()
	table := careReviewTable(parentbilling.CareRunDetail{
		Invoices: []parentbilling.CareInvoice{{
			Recipient: parentbilling.CareRecipient{FirstName: "Eva", LastName: "Muster", Kind: parentbilling.RecipientBoth, SecondName: "Jan Muster"},
			Payment: &parentbilling.InvoicePayment{Method: parentbilling.PaymentDirectDebit, DueOn: calendar.NewDate(2026, 11, 4),
				DueSource: parentbilling.DueFromRun},
			Positions:  []parentbilling.CarePosition{{ModuleName: "OGS Ganztag", AmountCents: 4550}},
			TotalCents: 4550,
		}},
	})
	require.NotEmpty(t, table.Rows)
	assert.Equal(t, "Lastschrift", table.Rows[0]["payment_method"])
	assert.Equal(t, "04.11.2026", table.Rows[0]["due_on"])
	assert.Equal(t, "Eva Muster und Jan Muster", table.Rows[0]["recipient"])
	ids := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		ids[i] = column.ID
	}
	assert.Contains(t, ids, "payment_method")
	assert.Contains(t, ids, "due_on")
}

// The review list carries the Mensa months, so a Mensa-only invoice has its
// rows and the rows of an invoice add up to its total (#4686).
func TestCareReviewTableListsTheMensaMonths(t *testing.T) {
	t.Parallel()
	table := careReviewTable(parentbilling.CareRunDetail{
		Invoices: []parentbilling.CareInvoice{{
			Positions: []parentbilling.CarePosition{{ModuleName: "OGS Ganztag", AmountCents: 7750}},
			MealPositions: []parentbilling.CareMealPosition{{MealMonth: calendar.NewDate(2026, 12, 1), Mode: parentbilling.MealModePerMeal,
				Quantity: 2, UnitCents: 480, AmountCents: 960}},
			TotalCents: 7750 + 960,
		}},
	})
	require.Len(t, table.Rows, 3)
	assert.Equal(t, "2 Mittagessen im Dezember 2026 à 4,80 €", table.Rows[1]["position"])
	assert.Equal(t, "9,60", table.Rows[1]["amount"])
	assert.Equal(t, "87,10", table.Rows[2]["amount"])
}
