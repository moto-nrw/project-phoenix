package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The layout of the overview for the Kämmerei is pure; its download,
// right, Träger boundary and trail are tested through the route in
// api/parent_billing_run_export_api_test.go.

func TestEuroAmountsAndSharesAreWrittenTheGermanWay(t *testing.T) {
	t.Parallel()
	for cents, want := range map[int64]string{0: "0,00", 5: "0,05", 4500: "45,00", 123456: "1.234,56", 100000000: "1.000.000,00", -500: "-5,00"} {
		assert.Equal(t, want, formatAmount(cents), "%d cents", cents)
	}
	for basisPoints, want := range map[int]string{5000: "50 %", 1250: "12,5 %", 10000: "100 %", 5: "0,05 %"} {
		assert.Equal(t, want, formatShare(basisPoints), "%d basis points", basisPoints)
	}
}

func TestOverviewListsEveryPositionPerFamilyWithTheSums(t *testing.T) {
	t.Parallel()
	monday, friday := calendar.NewDate(2027, 1, 4), calendar.NewDate(2027, 1, 8)
	issuedOn := calendar.NewDate(2026, 12, 1)
	rank, half := 2, 5000
	run := parentbilling.BillingRun{
		PeriodName: "Winterferien 2027", ServiceFrom: monday, ServiceUntil: calendar.NewDate(2027, 1, 15),
		Status: parentbilling.RunReleased, IssuedOn: &issuedOn,
	}
	documents := []parentbilling.BillingDocument{
		{Number: "FE-0001", RecipientFirstName: "Eva", RecipientLastName: "Muster", RecipientEmail: "eva@example.org", TotalCents: 6250,
			Children: []parentbilling.BillingChild{
				{FirstName: "Lia", LastName: "Muster", Positions: []parentbilling.BillingPosition{
					{Kind: parentbilling.LineBase, AmountCents: 4500, WeekStartsOn: &monday, WeekEndsOn: &friday, TimeModelName: "bis 16 Uhr"},
					{Kind: parentbilling.LineCorrection, AmountCents: -500, Note: "Erste Woche krank"},
				}},
				{FirstName: "Ben", LastName: "Muster", Positions: []parentbilling.BillingPosition{
					{Kind: parentbilling.LineBase, AmountCents: 4500, WeekStartsOn: &monday, WeekEndsOn: &friday, TimeModelName: "bis 16 Uhr"},
					{Kind: parentbilling.LineSibling, AmountCents: -2250, ChildRank: &rank, DiscountBasisPoints: &half},
				}},
			}},
		{Number: "FE-0002", RecipientFirstName: "Tom", RecipientLastName: "Beispiel", TotalCents: 0,
			Children: []parentbilling.BillingChild{{FirstName: "Mia", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBenefitsExemption, AmountCents: 0, Note: "BuT"},
			}}}},
	}

	table := billingRunOverview(run, documents)
	assert.Equal(t, "Abrechnungsanordnung", table.Title)
	assert.Equal(t, "Winterferien 2027", table.Subtitle)
	assert.Equal(t, []string{
		"Leistungszeitraum: 04.01.2027 bis 15.01.2027", "Rechnungsdatum: 01.12.2026",
		"2 Rechnungen, Nummern FE-0001 bis FE-0002", "Gesamtsumme: 62,50 Euro",
	}, table.Lines)
	require.Len(t, table.Rows, 8, "five positions, two invoice sums and the sum of the run")
	assert.Equal(t, map[string]string{
		"number": "FE-0001", "recipient": "Muster, Eva", "child": "Lia Muster",
		"position": "Woche 04.01.2027 bis 08.01.2027, bis 16 Uhr", "amount": "45,00",
	}, table.Rows[0])
	assert.Equal(t, "Korrektur: Erste Woche krank", table.Rows[1]["position"])
	assert.Equal(t, "-5,00", table.Rows[1]["amount"])
	assert.Equal(t, "Geschwister: 2. Kind, 50 % weniger", table.Rows[3]["position"])
	assert.Equal(t, map[string]string{"number": "FE-0001", "recipient": "Muster, Eva", "position": "Summe der Rechnung", "amount": "62,50"}, table.Rows[4])
	assert.Equal(t, "Befreiung bei Leistungsbezug: BuT", table.Rows[5]["position"])
	assert.Equal(t, map[string]string{"position": "Gesamtsumme des Laufs", "amount": "62,50"}, table.Rows[7])
	for _, row := range table.Rows {
		for _, value := range row {
			assert.NotContains(t, value, "@", "the overview carries no mail address")
		}
	}
	assert.Contains(t, billingRunOverview(run, documents[:1]).Lines, "1 Rechnung: FE-0001")
}

// TestOverviewNamesThePostalAddressOnlyWhenAnInvoiceHasOne pins the ADR 0122
// addendum (#5625): the three address columns follow the recipient when an
// invoice of the run copied a postal address; otherwise the columns of
// before stay.
func TestOverviewNamesThePostalAddressOnlyWhenAnInvoiceHasOne(t *testing.T) {
	t.Parallel()
	monday, friday := calendar.NewDate(2027, 1, 4), calendar.NewDate(2027, 1, 8)
	run := parentbilling.BillingRun{PeriodName: "Winterferien 2027", ServiceFrom: monday, ServiceUntil: friday, Status: parentbilling.RunReleased}
	position := []parentbilling.BillingPosition{{Kind: parentbilling.LineBase, AmountCents: 4500, TimeModelName: "bis 16 Uhr"}}
	documents := []parentbilling.BillingDocument{
		{Number: "FE-0001", RecipientFirstName: "Eva", RecipientLastName: "Muster", TotalCents: 4500,
			RecipientAddress: &parentbilling.PostalAddress{Street: "Musterweg 12", PostalCode: "01234", City: "Bissendorf"},
			Children:         []parentbilling.BillingChild{{FirstName: "Lia", LastName: "Muster", Positions: position}}},
		{Number: "FE-0002", RecipientFirstName: "Tom", RecipientLastName: "Beispiel", TotalCents: 4500,
			Children: []parentbilling.BillingChild{{FirstName: "Mia", LastName: "Beispiel", Positions: position}}},
	}
	columnIDs := func(table ExportTable) []string {
		ids := make([]string, len(table.Columns))
		for i, column := range table.Columns {
			ids[i] = column.ID
		}
		return ids
	}

	before := []string{"number", "recipient", "child", "position", "amount"}
	assert.Equal(t, before, columnIDs(billingRunOverview(run, documents[1:])), "no address, the columns of before")

	table := billingRunOverview(run, documents)
	assert.Equal(t, []string{"number", "recipient", "street", "postal_code", "city", "child", "position", "amount"}, columnIDs(table))
	assert.Equal(t, []ExportColumn{{ID: "street", Label: "Straße und Hausnummer"}, {ID: "postal_code", Label: "PLZ"}, {ID: "city", Label: "Ort"}},
		table.Columns[2:5])
	assert.Equal(t, map[string]string{
		"number": "FE-0001", "recipient": "Muster, Eva", "street": "Musterweg 12", "postal_code": "01234", "city": "Bissendorf",
		"child": "Lia Muster", "position": "bis 16 Uhr", "amount": "45,00",
	}, table.Rows[0])
	assert.Equal(t, "Bissendorf", table.Rows[1]["city"], "the sum of the invoice names the address too")
	assert.NotContains(t, table.Rows[2], "street", "an invoice without an address keeps the cells empty")
	assert.Equal(t, before, columnIDs(billingRunOverview(run, documents[1:])), "the shared columns stay unchanged")
}

// TestOverviewNamesTheDebtorNumberOnlyWhenAnInvoiceHasOne pins the ADR 0122
// addendum (#5621): the column "Debitorennummer" follows the recipient, on
// the rows of the positions and on the sum of each invoice, when an invoice
// of the run copied a debtor number; an invoice without one keeps the cell
// empty, and before the address columns when both are there.
func TestOverviewNamesTheDebtorNumberOnlyWhenAnInvoiceHasOne(t *testing.T) {
	t.Parallel()
	monday, friday := calendar.NewDate(2027, 1, 4), calendar.NewDate(2027, 1, 8)
	run := parentbilling.BillingRun{PeriodName: "Winterferien 2027", ServiceFrom: monday, ServiceUntil: friday, Status: parentbilling.RunReleased}
	position := []parentbilling.BillingPosition{{Kind: parentbilling.LineBase, AmountCents: 4500, TimeModelName: "bis 16 Uhr"}}
	documents := []parentbilling.BillingDocument{
		{Number: "FE-0001", RecipientFirstName: "Eva", RecipientLastName: "Muster", TotalCents: 4500, DebtorNumber: "10 4711",
			Children: []parentbilling.BillingChild{{FirstName: "Lia", LastName: "Muster", Positions: position}}},
		{Number: "FE-0002", RecipientFirstName: "Tom", RecipientLastName: "Beispiel", TotalCents: 4500,
			Children: []parentbilling.BillingChild{{FirstName: "Mia", LastName: "Beispiel", Positions: position}}},
	}
	columnIDs := func(table ExportTable) []string {
		ids := make([]string, len(table.Columns))
		for i, column := range table.Columns {
			ids[i] = column.ID
		}
		return ids
	}

	assert.Equal(t, []string{"number", "recipient", "child", "position", "amount"}, columnIDs(billingRunOverview(run, documents[1:])),
		"no debtor number, the columns of before")
	table := billingRunOverview(run, documents)
	assert.Equal(t, []string{"number", "recipient", "debtor_number", "child", "position", "amount"}, columnIDs(table))
	assert.Equal(t, ExportColumn{ID: "debtor_number", Label: "Debitorennummer"}, table.Columns[2])
	assert.Equal(t, "10 4711", table.Rows[0]["debtor_number"])
	assert.Equal(t, "10 4711", table.Rows[1]["debtor_number"], "the sum of the invoice names the number too")
	assert.NotContains(t, table.Rows[2], "debtor_number", "an invoice without a number keeps the cell empty")

	documents[1].RecipientAddress = &parentbilling.PostalAddress{Street: "Musterweg 12", PostalCode: "01234", City: "Bissendorf"}
	assert.Equal(t, []string{"number", "recipient", "debtor_number", "street", "postal_code", "city", "child", "position", "amount"},
		columnIDs(billingRunOverview(run, documents)), "the debtor number comes right after the recipient, before the address")
}
