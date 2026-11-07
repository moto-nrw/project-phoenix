package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the monthly contribution (#4685, ADR 0164), computed by
// hand. Prices of the table (placeholders, not the SJR's):
//
//	Ispringen  (11): until 14:00 (101) Mon 40,00 €, Tue 40,00 €
//	                 until 16:30 (102) Mon 60,00 €
//	                 early care  (103) Mon 33,33 €
//	Birkenfeld (12): until 14:00 (101) Mon 45,00 €

const (
	ruleIncome int64 = 701 + iota
	ruleSiblingIspringen
	ruleSiblingBirkenfeld
	ruleSingleParent
	ruleCap
	contractLea  int64 = 503
	familyEva          = "eva@example.org"
	familyOther        = "jonas@example.org"
	billingMonth       = "2027-01-01"
)

func chainTariffs(t *testing.T) []parentbilling.CareTariff {
	t.Helper()
	return []parentbilling.CareTariff{
		price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2026-09-01", 4000),
		price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "tue", "2026-09-01", 4000),
		price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", "mon", "2026-09-01", 6000),
		price(t, schoolIspringen, moduleEarly, "Frühbetreuung", "mon", "2026-09-01", 3333),
		price(t, schoolBirkenfeld, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2026-09-01", 4500),
	}
}

// careChainChild is one contract of the table: its child, school, family,
// date of birth and booked modules.
type careChainChild struct {
	contract int64
	school   int64
	family   string
	born     string
	booked   []parentbilling.CareContractService
}

var (
	mon1400  = booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	tue1400  = booked("tue", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	mon1630  = booked("mon", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
	monEarly = booked("mon", moduleEarly, "Frühbetreuung", "08:00")
)

func ptr[T any](value T) *T { return &value }

func careRule(t *testing.T, id int64, kind string, schools []int64, set func(*parentbilling.ContributionRule)) parentbilling.ContributionRule {
	t.Helper()
	rule := parentbilling.ContributionRule{ID: id, Service: parentbilling.ServiceCare, Kind: kind, Name: kind,
		ValidFrom: day(t, "2026-09-01"), SchoolIDs: schools, CareOfferingIDs: []int64{}, CountSchoolIDs: []int64{}}
	set(&rule)
	return rule
}

func halfFromSecond(rule *parentbilling.ContributionRule) {
	rule.SiblingFromChild, rule.DiscountBasisPoints = ptr(2), ptr(5000)
}

func evidence(t *testing.T, contract int64, kind string, rule *int64, from string, until *calendar.Date) parentbilling.CareEvidence {
	t.Helper()
	return parentbilling.CareEvidence{ContractID: contract, Kind: kind, RuleID: rule, ValidFrom: day(t, from), ValidUntil: until,
		Evidence: "Einkommensteuerbescheid 2025", CheckedOn: day(t, "2026-12-01")}
}

// reductionOf is a reduction as the table compares it.
type reductionOf struct {
	kind   string
	amount int64
	rank   int
}

func TestCareMonthRuleChainTable(t *testing.T) {
	t.Parallel()
	until := day(t, "2026-12-31")
	cases := []struct {
		name      string
		children  []careChainChild
		invoiced  []int64
		rules     func(t *testing.T) []parentbilling.ContributionRule
		evidences func(t *testing.T) []parentbilling.CareEvidence
		totals    map[int64]int64
		lines     map[int64][]reductionOf
	}{
		{
			// Mia 40 + 40 = 80,00 €, the first child; Noah 60,00 € the
			// second: 60,00 − 50 % = 30,00 €.
			name: "half from the second child",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			totals: map[int64]int64{contractMia: 8000, contractNoah: 3000},
			lines:  map[int64][]reductionOf{contractNoah: {{parentbilling.RuleSibling, -3000, 2}}},
		},
		{
			// The order follows the base price, not the age (ADR 0115):
			// Noah, the younger, books 60,00 € and is first; Mia, the
			// older, books 40,00 € and is second: 40,00 − 50 % = 20,00 €.
			name: "price order before age",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolIspringen, familyEva, "2018-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			totals: map[int64]int64{contractNoah: 6000, contractMia: 2000},
			lines:  map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -2000, 2}}},
		},
		{
			// Equal base prices: the older child is first. Mia and Noah
			// both 40,00 €; Noah is older, Mia second: 20,00 €.
			name: "a tie goes to the older child",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2018-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolIspringen, familyEva, "2016-03-01", []parentbilling.CareContractService{tue1400}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			totals: map[int64]int64{contractNoah: 4000, contractMia: 2000},
		},
		{
			// Fixed end price 25,00 € from the second child: Mia pays
			// 25,00 € (reduction 40,00 − 25,00 = 15,00 €).
			name: "fixed end price from the second child",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) {
						rule.SiblingFromChild, rule.PriceCents = ptr(2), ptr(int64(2500))
					})}
			},
			totals: map[int64]int64{contractNoah: 6000, contractMia: 2500},
			lines:  map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -1500, 2}}},
		},
		{
			// The same 25,00 € as a fixed discount: Mia pays 40,00 − 25,00
			// = 15,00 €, not 25,00 €.
			name: "fixed discount from the second child",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) {
						rule.SiblingFromChild, rule.DiscountCents = ptr(2), ptr(int64(2500))
					})}
			},
			totals: map[int64]int64{contractNoah: 6000, contractMia: 1500},
			lines:  map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -2500, 2}}},
		},
		{
			// Ispringen and Birkenfeld each count their own children (the
			// default): Mia in Ispringen (40,00 €) and Noah in Birkenfeld
			// (45,00 €) are each a first child. Nobody gets a reduction.
			name: "separate scopes Ispringen and Birkenfeld",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{
					careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond),
					careRule(t, ruleSiblingBirkenfeld, parentbilling.RuleSibling, []int64{schoolBirkenfeld},
						func(rule *parentbilling.ContributionRule) {
							rule.SiblingFromChild, rule.PriceCents = ptr(2), ptr(int64(2000))
						}),
				}
			},
			totals: map[int64]int64{contractMia: 4000, contractNoah: 4500},
		},
		{
			// Ispringen counts the children of Birkenfeld too: Noah
			// (45,00 € in Birkenfeld) is first, Mia (40,00 € in Ispringen)
			// second: 20,00 €. Birkenfeld's own rule still counts only
			// Birkenfeld, where Noah is alone.
			name: "Ispringen counts Birkenfeld",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{
					careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
						halfFromSecond(rule)
						rule.CountSchoolIDs = []int64{schoolIspringen, schoolBirkenfeld}
					}),
					careRule(t, ruleSiblingBirkenfeld, parentbilling.RuleSibling, []int64{schoolBirkenfeld},
						func(rule *parentbilling.ContributionRule) {
							rule.SiblingFromChild, rule.PriceCents = ptr(2), ptr(int64(2000))
						}),
				}
			},
			totals: map[int64]int64{contractMia: 2000, contractNoah: 4500},
			lines:  map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -2000, 2}}},
		},
		{
			// Different families never count together.
			name: "another family is no sibling",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolIspringen, familyOther, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			totals: map[int64]int64{contractMia: 4000, contractNoah: 6000},
		},
		{
			// Only the afternoon module (102) counts and is reduced. Mia
			// books no 102 and does not count; Noah (102 60,00 € and Tue
			// 40,00 €) and Lea (102 60,00 €) tie on 60,00 €, Noah is older
			// and first. Lea: 60,00 − 50 % = 30,00 €.
			name: "only certain modules",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2014-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2016-03-01", []parentbilling.CareContractService{mon1630, tue1400}},
				{contractLea, schoolIspringen, familyEva, "2018-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) {
						halfFromSecond(rule)
						rule.CareOfferingIDs = []int64{moduleUntil1630}
					})}
			},
			totals: map[int64]int64{contractMia: 8000, contractNoah: 10000, contractLea: 3000},
			lines:  map[int64][]reductionOf{contractLea: {{parentbilling.RuleSibling, -3000, 2}}},
		},
		{
			// Cap 100,00 € per family: Mia 80,00 € + Noah 60,00 € = 140,00 €;
			// the excess 40,00 € comes off the last child, Noah: 20,00 €.
			name: "cap of the family",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.CapCents = ptr(int64(10000)) })}
			},
			totals: map[int64]int64{contractMia: 8000, contractNoah: 2000},
			lines:  map[int64][]reductionOf{contractNoah: {{parentbilling.RuleCap, -4000, 0}}},
		},
		{
			// The whole chain in its order. Mia 80,00 €, income level
			// 25 % off: −20,00 € → 60,00 €. Noah 60,00 €, second child
			// (ranked by base price): −30,00 € → 30,00 €; single parent
			// −10,00 € → 20,00 €. Cap 70,00 €: 60,00 + 20,00 = 80,00 €, the
			// excess 10,00 € comes off Noah → 10,00 €.
			name: "combination of every rule",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{
					careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen},
						func(rule *parentbilling.ContributionRule) { rule.DiscountBasisPoints = ptr(2500) }),
					careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond),
					careRule(t, ruleSingleParent, parentbilling.RuleSingleParent, []int64{schoolIspringen},
						func(rule *parentbilling.ContributionRule) { rule.DiscountCents = ptr(int64(1000)) }),
					careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen},
						func(rule *parentbilling.ContributionRule) { rule.CapCents = ptr(int64(7000)) }),
				}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{
					evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2026-09-01", nil),
					evidence(t, contractNoah, parentbilling.EvidenceSingleParent, nil, "2026-09-01", nil),
				}
			},
			totals: map[int64]int64{contractMia: 6000, contractNoah: 1000},
			lines: map[int64][]reductionOf{
				contractMia: {{parentbilling.RuleIncomeLevel, -2000, 0}},
				contractNoah: {{parentbilling.RuleSibling, -3000, 2}, {parentbilling.RuleSingleParent, -1000, 0},
					{parentbilling.RuleCap, -1000, 0}},
			},
		},
		{
			// Cent rounding half up: Noah's early care 33,33 € − 50 % =
			// 16,665 € → reduction 16,67 €, Noah pays 16,66 €. A share
			// over two positions adds up to the cent: Mia 80,00 € with
			// 33,33 % off = 26,664 € → 26,66 €, Mia pays 53,34 €.
			name: "cent rounding",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{monEarly}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{
					careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen},
						func(rule *parentbilling.ContributionRule) { rule.DiscountBasisPoints = ptr(3333) }),
					careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond),
				}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2026-09-01", nil)}
			},
			totals: map[int64]int64{contractMia: 5334, contractNoah: 1666},
			lines: map[int64][]reductionOf{
				contractMia:  {{parentbilling.RuleIncomeLevel, -2666, 0}},
				contractNoah: {{parentbilling.RuleSibling, -1667, 2}},
			},
		},
		{
			// An earlier run of the month issued Mia's invoice. Noah, billed
			// now, is still the second child: 30,00 €.
			name: "a child an earlier run billed still counts",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			invoiced: []int64{contractMia},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			totals: map[int64]int64{contractNoah: 3000},
		},
		{
			// The evidence starts on the 16th: it holds for the month
			// (like a version that starts within it). One that ended in
			// December holds for nothing in January.
			name: "evidence of the month",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyOther, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.PriceCents = ptr(int64(5000)) })}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{
					evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2027-01-16", nil),
					evidence(t, contractNoah, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2026-09-01", &until),
				}
			},
			totals: map[int64]int64{contractMia: 5000, contractNoah: 6000},
		},
		{
			// A rule that ends before the month does not apply.
			name: "rule of another month",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { halfFromSecond(rule); rule.ValidUntil = &until })}
			},
			totals: map[int64]int64{contractMia: 8000, contractNoah: 6000},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst, tariffs: chainTariffs(t),
				invoiced: map[int64]bool{}, recipients: map[int64]parentbilling.CareRecipient{}, rules: tc.rules(t)}
			for _, child := range tc.children {
				version := sjrVersion(t, child.contract, child.school, 1, billingMonth, "2027-01-31", child.booked...)
				born := day(t, child.born)
				version.ChildBirthDate = &born
				in.versions = append(in.versions, version)
				in.recipients[child.contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: child.family}
			}
			for _, contract := range tc.invoiced {
				in.invoiced[contract] = true
			}
			if tc.evidences != nil {
				in.evidences = tc.evidences(t)
			}
			month, err := buildCareMonth(in)
			require.NoError(t, err)
			totals := map[int64]int64{}
			for _, invoice := range month.invoices {
				totals[invoice.ContractID] = invoice.TotalCents
				var sum int64
				for _, position := range invoice.Positions {
					sum += position.AmountCents
				}
				var lines []reductionOf
				for _, reduction := range invoice.Reductions {
					rank := 0
					if reduction.ChildRank != nil {
						rank = *reduction.ChildRank
					}
					lines = append(lines, reductionOf{reduction.Kind, reduction.AmountCents, rank})
					sum += reduction.AmountCents
				}
				assert.Equal(t, invoice.TotalCents, sum, "the total is the positions less the reductions")
				if want, ok := tc.lines[invoice.ContractID]; ok {
					assert.Equal(t, want, lines, "reductions of contract %d", invoice.ContractID)
				}
			}
			assert.Equal(t, tc.totals, totals)
		})
	}
}

func TestCareSiblingReductionNamesRankAndFamily(t *testing.T) {
	t.Parallel()
	in := careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst, tariffs: chainTariffs(t),
		recipients: map[int64]parentbilling.CareRecipient{
			contractMia: {Email: "Eva@Example.org"}, contractNoah: {Email: " eva@example.org "}, contractLea: {Email: familyEva},
		},
		rules: []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
			func(rule *parentbilling.ContributionRule) {
				rule.SiblingFromChild, rule.DiscountBasisPoints = ptr(3), ptr(10000)
			})},
	}
	for _, contract := range []int64{contractMia, contractNoah, contractLea} {
		in.versions = append(in.versions, sjrVersion(t, contract, schoolIspringen, 1, billingMonth, "2027-01-31", mon1400))
	}
	month, err := buildCareMonth(in)
	require.NoError(t, err)
	var reduced []parentbilling.CareReduction
	for _, invoice := range month.invoices {
		reduced = append(reduced, invoice.Reductions...)
	}
	// The address counts without case and spaces: three children, the
	// third free; equal prices and no date of birth keep the order of the
	// contracts, so the third contract is the third child.
	require.Len(t, reduced, 1)
	assert.Equal(t, 3, *reduced[0].ChildRank)
	assert.Equal(t, 3, *reduced[0].FamilyChildren)
	assert.Equal(t, int64(-4000), reduced[0].AmountCents)
	for _, invoice := range month.invoices {
		if invoice.ContractID == contractLea {
			assert.Zero(t, invoice.TotalCents)
		}
	}
}
