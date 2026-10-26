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
		assert.Equal(t, want, formatEuro(cents), "%d cents", cents)
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
	assert.Equal(t, "Ferienbetreuung: Winterferien 2027", table.Subtitle)
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
}
