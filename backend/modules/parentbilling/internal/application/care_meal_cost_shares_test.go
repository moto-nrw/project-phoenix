package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// mealShareMonth is Mia's April with one module, Monday until 14:00 (32,00 €),
// and 2 × 4,80 € = 9,60 € Mensa of March, under the shares; ownInvoice puts
// the Mensa on an invoice of its own (ADR 0167). It runs what monthWith runs.
func mealShareMonth(t *testing.T, ownInvoice bool, shares ...parentbilling.CareCostShare) careMonth {
	t.Helper()
	versions := []parentbilling.CareContractVersion{
		sjrVersion(t, contractMia, schoolIspringen, 1, mealRunMonth, "2026-04-30", booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")),
	}
	recipients := map[int64]parentbilling.CareRecipient{contractMia: {FirstName: "Eva", LastName: "Beispiel", Email: familyEva}}
	invoiced := map[int64]parentbilling.CareInvoice{}
	month, err := buildCareMonth(careMonthInput{month: day(t, mealRunMonth), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: versions, invoiced: invoiced, recipients: recipients, costShares: shares,
		tariffs: []parentbilling.CareTariff{price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2026-01-01", 3200)}})
	require.NoError(t, err)
	billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday), schools: map[int64]string{schoolIspringen: "Ispringen"},
		tariffs:  []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)},
		releases: []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(2))}, billed: map[CareMealKey]bool{},
		versions: versions, invoiced: invoiced, recipients: recipients, ownInvoice: ownInvoice})
	if ownInvoice {
		splitMealInvoices(&month)
	}
	applyMealCostShares(&month, shares, day(t, mealRunMonth), invoiced)
	return month
}

func scoped(scope string) func(*parentbilling.CareCostShare) {
	return func(share *parentbilling.CareCostShare) { share.Scope = scope }
}

func withAll(sets ...func(*parentbilling.CareCostShare)) func(*parentbilling.CareCostShare) {
	return func(share *parentbilling.CareCostShare) {
		for _, set := range sets {
			set(share)
		}
	}
}

// invoiceSplit is what a test compares of an invoice: its service, what
// each cost bearer pays and the family's rest.
type invoiceSplit struct {
	service string
	shares  []shareOf
	family  int64
}

func splitsOf(month careMonth) []invoiceSplit {
	splits := []invoiceSplit{}
	for _, invoice := range month.invoices {
		split := invoiceSplit{service: invoice.Service, shares: []shareOf{}, family: invoice.TotalCents}
		for _, share := range invoice.CostShares {
			split.shares = append(split.shares, shareOf{bearer: share.CostBearerID, amount: share.AmountCents})
		}
		splits = append(splits, split)
	}
	return splits
}

// A cost bearer share covers the modules, the Mensa or both (#4686, ADR 0168
// point 12). Hand-computed on 32,00 € of modules and 9,60 € of Mensa:
func TestCareCostSharesCoverTheModulesTheMensaOrBoth(t *testing.T) {
	t.Parallel()
	but := func(set func(*parentbilling.CareCostShare)) parentbilling.CareCostShare {
		return costShare(t, contractMia, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-01-01", "2026-12-31", set)
	}
	youth := func(set func(*parentbilling.CareCostShare)) parentbilling.CareCostShare {
		return costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-01-01", "2026-12-31", set)
	}
	for _, tc := range []struct {
		name   string
		own    bool
		shares []parentbilling.CareCostShare
		want   []invoiceSplit
	}{{
		// Before the Mensa every share covered the care: without a scope
		// 50 % of 32,00 € = 16,00 €; the Mensa stays with the family:
		// 16,00 + 9,60 = 25,60 €.
		name:   "no scope covers the modules",
		shares: []parentbilling.CareCostShare{but(basisPoints(5000))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 1600}}, 1600 + 960}},
	}, {
		// BuT takes the lunch, 100 % of 9,60 €; the family pays the modules.
		name:   "Bildung und Teilhabe takes the Mensa",
		shares: []parentbilling.CareCostShare{but(withAll(basisPoints(10000), scoped(parentbilling.CostShareScopeMeal)))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 960}}, 3200}},
	}, {
		// BuT the Mensa (9,60 €), the Jugendamt half the care (16,00 €):
		// the family pays 32,00 − 16,00 = 16,00 €; each bearer once, in the
		// order of the kinds.
		name: "the Mensa and the care to two bearers",
		shares: []parentbilling.CareCostShare{
			but(withAll(basisPoints(10000), scoped(parentbilling.CostShareScopeMeal))),
			youth(withAll(basisPoints(5000), scoped(parentbilling.CostShareScopeCare))),
		},
		want: []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 960}, {bearerYouthOffice, 1600}}, 1600}},
	}, {
		// 25 % of both, each part rounded on its own: 8,00 € of the modules
		// and 2,40 € of the Mensa, 10,40 € on one line; the family pays
		// 41,60 − 10,40 = 31,20 €.
		name:   "a part of both",
		shares: []parentbilling.CareCostShare{but(withAll(basisPoints(2500), scoped(parentbilling.CostShareScopeBoth)))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 800 + 240}}, 3120}},
	}, {
		// 20,00 € a month for both: the modules take all of it, nothing is
		// left for the Mensa: 12,00 + 9,60 = 21,60 €.
		name:   "a fixed amount for both within the modules",
		shares: []parentbilling.CareCostShare{but(withAll(monthly(2000), scoped(parentbilling.CostShareScopeBoth)))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 2000}}, 1200 + 960}},
	}, {
		// 35,00 € a month for both: 32,00 € of the modules, the rest 3,00 €
		// of the Mensa; the family pays 9,60 − 3,00 = 6,60 €.
		name:   "a fixed amount for both beyond the modules",
		shares: []parentbilling.CareCostShare{but(withAll(monthly(3500), scoped(parentbilling.CostShareScopeBoth)))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 3500}}, 660}},
	}, {
		// 15,00 € a month for the Mensa takes at most the 9,60 €.
		name:   "a fixed amount for the Mensa takes at most the Mensa",
		shares: []parentbilling.CareCostShare{but(withAll(monthly(1500), scoped(parentbilling.CostShareScopeMeal)))},
		want:   []invoiceSplit{{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 960}}, 3200}},
	}, {
		// On an invoice of its own the Mensa carries only the shares of the
		// Mensa: the Jugendamt's half of the care stays on the care invoice
		// (16,00 €), BuT takes the 9,60 € of the Mensa invoice.
		name: "the Mensa invoice carries only the Mensa shares",
		own:  true,
		shares: []parentbilling.CareCostShare{
			but(withAll(basisPoints(10000), scoped(parentbilling.CostShareScopeMeal))),
			youth(withAll(basisPoints(5000), scoped(parentbilling.CostShareScopeCare))),
		},
		want: []invoiceSplit{
			{parentbilling.ServiceCare, []shareOf{{bearerYouthOffice, 1600}}, 1600},
			{parentbilling.ServiceMeal, []shareOf{{bearerJobcenter, 960}}, 0},
		},
	}, {
		// A fixed 35,00 € for both across the two invoices: 32,00 € of the
		// care invoice, the rest 3,00 € of the Mensa invoice (9,60 − 3,00 =
		// 6,60 €).
		name:   "a fixed amount for both across two invoices",
		own:    true,
		shares: []parentbilling.CareCostShare{but(withAll(monthly(3500), scoped(parentbilling.CostShareScopeBoth)))},
		want: []invoiceSplit{
			{parentbilling.ServiceCare, []shareOf{{bearerJobcenter, 3200}}, 0},
			{parentbilling.ServiceMeal, []shareOf{{bearerJobcenter, 300}}, 660},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			month := mealShareMonth(t, tc.own, tc.shares...)
			assert.Equal(t, tc.want, splitsOf(month))
			for _, invoice := range month.invoices {
				var billed int64
				for _, position := range invoice.Positions {
					billed += position.PayableCents
				}
				for _, position := range invoice.MealPositions {
					assert.Equal(t, int64(960), position.AmountCents, "the Mensa position keeps its full amount")
					billed += position.AmountCents
				}
				assert.Equal(t, billed, invoice.TotalCents+sumShares(invoice), "modules and Mensa = shares + rest")
			}
		})
	}
}

func sumShares(invoice parentbilling.CareInvoice) int64 {
	var sum int64
	for _, share := range invoice.CostShares {
		sum += share.AmountCents
	}
	return sum
}

// The list of a cost bearer shows the base of its share: the Mensa for a
// share of the Mensa, the modules for one of the care, the whole for both.
func TestCareCostShareListShowsTheBaseOfTheScope(t *testing.T) {
	t.Parallel()
	invoice := parentbilling.CareInvoice{TotalCents: 1600,
		MealPositions: []parentbilling.CareMealPosition{{AmountCents: 960}},
		CostShares: []parentbilling.CareInvoiceCostShare{
			{Kind: parentbilling.CostShareEducationParticipation, Scope: parentbilling.CostShareScopeMeal, AmountCents: 960, ShareBasisPoints: ptr(10000)},
			{Kind: parentbilling.CostShareYouthOffice, Scope: parentbilling.CostShareScopeCare, AmountCents: 1600, ShareBasisPoints: ptr(5000)},
		}}
	assert.Equal(t, int64(960), costShareBaseCents(invoice, parentbilling.CostShareScopeMeal))
	assert.Equal(t, int64(3200), costShareBaseCents(invoice, parentbilling.CostShareScopeCare))
	assert.Equal(t, int64(4160), costShareBaseCents(invoice, parentbilling.CostShareScopeBoth))
	assert.Equal(t, "100 % für die Mensa", costShareText(invoice.CostShares[0]))
	assert.Equal(t, "50 %", costShareText(invoice.CostShares[1]))
}
