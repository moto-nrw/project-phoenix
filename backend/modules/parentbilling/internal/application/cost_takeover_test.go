package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The cost takeover (#5627): the chain lets the cost bearer pay what is
// left after the sibling discount, the letter of 0 € names the cost bearer,
// the overview for the Kämmerei sums what each cost bearer pays and the
// list for the reimbursement holds exactly that cost bearer's children.

func youthOfficeBearer() *parentbilling.CostBearer {
	return &parentbilling.CostBearer{ID: 7, Name: "Fachdienst Jugend des Landkreises Musterstadt", Active: true}
}

func TestCostTakeoverPaysTheRestAfterTheSiblingDiscount(t *testing.T) {
	t.Parallel()
	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	children := []parentbilling.HolidayChild{chainChild(1, 1, 10000), chainChild(2, 2, 10000)}
	bearer := youthOfficeBearer()
	result := computeHolidayContribution(children, holidayRules{sibling: half}, false, bearer)

	assert.Equal(t, int64(0), result.TotalCents, "the family pays nothing")
	require.NotNil(t, result.CostBearerID)
	assert.Equal(t, bearer.ID, *result.CostBearerID)
	var claim int64
	for _, child := range result.Children {
		assert.Equal(t, int64(0), child.TotalCents)
		last := child.Lines[len(child.Lines)-1]
		assert.Equal(t, parentbilling.LineCostTakeover, last.Kind)
		assert.Equal(t, "Fachdienst Jugend des Landkreises Musterstadt", last.CostBearerName)
		claim -= last.AmountCents
	}
	assert.Equal(t, int64(15000), claim, "the cost bearer pays 100 € for the first and 50 € for the second child")
	assert.Equal(t, []string{"base", "sibling", "cost_takeover"}, []string{
		result.Children[1].Lines[0].Kind, result.Children[1].Lines[1].Kind, result.Children[1].Lines[2].Kind,
	}, "the sibling discount comes first")

	exempt := computeHolidayContribution(children, holidayRules{exemption: &parentbilling.ContributionRule{Note: "BuT"}}, true, nil)
	assert.Nil(t, exempt.CostBearerID, "the exemption stays a waiver without a payer")
	assert.True(t, exempt.BenefitsExemption)
}

func TestLetterOfACostTakeoverNamesTheCostBearer(t *testing.T) {
	t.Parallel()
	content := goldenLetterContent(t)
	bearerID := youthOfficeBearer().ID
	content.document.TotalCents = 0
	content.letterhead.PaymentText = "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist}."
	content.document.Children = []parentbilling.BillingChild{{FirstName: "Lina", Positions: []parentbilling.BillingPosition{
		{Kind: parentbilling.LineBase, AmountCents: 5000, TimeModelName: "Vormittag"},
		{Kind: parentbilling.LineCostTakeover, AmountCents: -5000, CostBearerID: &bearerID, CostBearerName: "Fachdienst Jugend des Landkreises Musterstadt"},
	}}}

	letter := confirmationLetter(content)
	require.Len(t, letter.Blocks, 6)
	assert.Equal(t, []LetterFact{
		{Label: "Lina: Vormittag", Value: "50,00 €"},
		{Label: "Lina: Kosten übernimmt: Fachdienst Jugend des Landkreises Musterstadt", Value: "-50,00 €"},
		{Label: "Gesamtbetrag", Value: "0,00 €", Bold: true},
	}, letter.Blocks[1].Facts)
	assert.Equal(t, LetterBlock{Text: "Sie müssen nichts bezahlen. Die Kosten übernimmt: Fachdienst Jugend des Landkreises Musterstadt."},
		letter.Blocks[2], "an invoice of 0 € names the cost bearer instead of the payment text")
	for _, fact := range letter.Facts {
		assert.NotEqual(t, "Zahlen bis", fact.Label)
	}
}

func socialOfficeBearer() *parentbilling.CostBearer {
	return &parentbilling.CostBearer{ID: 8, Name: "Sozialamt", Active: true}
}

func july(day int) *calendar.Date {
	date := calendar.NewDate(2027, 7, day)
	return &date
}

func takeoverDocuments() []parentbilling.BillingDocument {
	office, other := youthOfficeBearer().ID, socialOfficeBearer().ID
	return []parentbilling.BillingDocument{
		{Number: "FE-0001", RegistrationID: 11, RecipientFirstName: "Eva", RecipientLastName: "Muster", Children: []parentbilling.BillingChild{
			{FirstName: "Lia", LastName: "Muster", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 10000, WeekStartsOn: july(5), WeekEndsOn: july(9), TimeModelName: "bis 16 Uhr"},
				{Kind: parentbilling.LineCostTakeover, AmountCents: -10000, CostBearerID: &office, CostBearerName: "Jugendamt"},
			}},
			{FirstName: "Ben", LastName: "Muster", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 10000, WeekStartsOn: july(5), WeekEndsOn: july(9), TimeModelName: "bis 16 Uhr"},
				{Kind: parentbilling.LineSibling, AmountCents: -5000},
				{Kind: parentbilling.LineCostTakeover, AmountCents: -5000, CostBearerID: &office, CostBearerName: "Jugendamt"},
			}},
		}},
		{Number: "FE-0002", RegistrationID: 12, RecipientFirstName: "Tom", RecipientLastName: "Beispiel", TotalCents: 4500, Children: []parentbilling.BillingChild{
			{FirstName: "Mia", LastName: "Beispiel", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 4500, TimeModelName: "bis 13 Uhr"},
			}},
		}},
		{Number: "FE-0003", RegistrationID: 13, RecipientFirstName: "Ali", RecipientLastName: "Probe", Children: []parentbilling.BillingChild{
			{FirstName: "Noa", LastName: "Probe", Positions: []parentbilling.BillingPosition{
				{Kind: parentbilling.LineBase, AmountCents: 4500, TimeModelName: "bis 13 Uhr"},
				{Kind: parentbilling.LineCostTakeover, AmountCents: -4500, CostBearerID: &other, CostBearerName: "Sozialamt"},
			}},
		}},
	}
}

func TestOverviewSumsWhatEachCostBearerPaysUnderTheTotal(t *testing.T) {
	t.Parallel()
	run := parentbilling.BillingRun{PeriodName: "Sommerferien 2027", ServiceFrom: *july(5), ServiceUntil: *july(9)}
	table := billingRunOverview(run, takeoverDocuments())

	positions := []string{}
	for _, row := range table.Rows {
		positions = append(positions, row["position"]+"="+row["amount"])
	}
	assert.Contains(t, positions, "Kostenübernahme: Jugendamt=-50,00")
	assert.Equal(t, []string{
		"Gesamtsumme des Laufs=45,00",
		"Davon übernimmt Jugendamt (1 Rechnung)=150,00",
		"Davon übernimmt Sozialamt (1 Rechnung)=45,00",
	}, positions[len(positions)-3:], "the total counts what the families pay; each cost bearer follows with its sum")
	assert.Contains(t, table.Lines, "Davon übernimmt Jugendamt: 150,00 Euro (1 Rechnung)")
}

func TestReimbursementListHoldsOnlyTheChildrenOfItsCostBearer(t *testing.T) {
	t.Parallel()
	bearer := parentbilling.CostBearer{ID: youthOfficeBearer().ID, Name: "Jugendamt", Address: "Am Kreishaus 1\n12345 Musterstadt", FileReference: "FJ 51"}
	table, documents := costTakeoverList(costTakeoverInput{
		run: parentbilling.BillingRun{PeriodName: "Sommerferien 2027"}, bearer: bearer, documents: takeoverDocuments(),
		proofs: map[int64]bool{11: true},
	})

	assert.Equal(t, 1, documents)
	require.Len(t, table.Rows, 6, "two children, the sum and three rows naming the cost bearer")
	assert.Equal(t, map[string]string{
		"number": "FE-0001", "child": "Ben Muster", "weeks": "05.07. bis 09.07.2027, bis 16 Uhr",
		"base": "100,00", "sibling": "-50,00", "takeover": "50,00", "proof": "Ja",
	}, table.Rows[1], "a registration gone after 92 days leaves the confirmation day empty")
	assert.Equal(t, map[string]string{"weeks": "Summe", "takeover": "150,00"}, table.Rows[2])
	assert.Equal(t, map[string]string{"child": "Anschrift", "weeks": "Am Kreishaus 1, 12345 Musterstadt"}, table.Rows[4])
	assert.Contains(t, table.Lines, "2 Kinder, 1 Rechnung")
	assert.Contains(t, table.Lines, "Summe: 150,00 Euro")
}
