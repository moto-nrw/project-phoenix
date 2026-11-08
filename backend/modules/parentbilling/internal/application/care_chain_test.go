package application

import (
	"slices"
	"testing"
	"time"

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
		price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", "tue", "2026-09-01", 4000),
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
	tue1630  = booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
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
		name     string
		children []careChainChild
		invoiced []int64
		// sameChild gives a contract the child of another one.
		sameChild map[int64]int64
		// sameFamily moves a contract into a family after the earlier run.
		sameFamily map[int64]string
		rules      func(t *testing.T) []parentbilling.ContributionRule
		evidences  func(t *testing.T) []parentbilling.CareEvidence
		// families are the numbers of the families (#5918); afterRelease
		// gives them only to the run after the earlier one issued invoiced.
		families     func(t *testing.T) []parentbilling.FamilyChildCount
		afterRelease bool
		uncounted    int
		totals       map[int64]int64
		lines        map[int64][]reductionOf
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
		{
			// One child, two contracts. 501 (Mon until 14:00 = A 40,00 €,
			// Mon until 16:30 = B 60,00 €) was issued by an earlier run:
			// the income level holds only for A and sets it to 0,00 €; the
			// sibling price of 25,00 € holds only for B (Noah, B Mon and Tue
			// = 100,00 €, ties with Mia's 60 + 40 and is older: first child). The first run shared B's discount of
			// 100,00 − 25,00 = 75,00 € by base: 501 B 60 → 45,00 € off, B
			// pays 15,00 €; 503 B 40 → 30,00 € off. Now 503 (Tue until 16:30
			// = B 40,00 €) is billed alone: 501 holds 15,00 € of the price
			// at the sibling stage (the income level took nothing from B),
			// so 503 pays 25,00 − 15,00 = 10,00 €. Spreading the income
			// reduction over A and B would count 501 at 0,00 € and bill
			// 503 at 25,00 €.
			name: "an earlier income reduction stays on its module",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, mon1630}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630, tue1630}},
				{contractLea, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{tue1630}},
			},
			invoiced:  []int64{contractMia},
			sameChild: map[int64]int64{contractLea: contractMia},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{
					careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
						rule.PriceCents, rule.CareOfferingIDs = ptr(int64(0)), []int64{moduleUntil1400}
					}),
					careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
						rule.SiblingFromChild, rule.PriceCents, rule.CareOfferingIDs = ptr(2), ptr(int64(2500)), []int64{moduleUntil1630}
					}),
				}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2026-09-01", nil)}
			},
			totals: map[int64]int64{contractNoah: 10000, contractLea: 1000},
			lines:  map[int64][]reductionOf{contractLea: {{parentbilling.RuleSibling, -3000, 2}}},
		},
		{
			// The family states two children and the Abrechnung confirmed
			// it; only Mia has a contract. Her sibling without one comes
			// first: Mia is the 2nd child, 80,00 − 50 % = 40,00 €.
			name: "the family's number makes the only contract child the second",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 2, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 4000},
			lines:    map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -4000, 2}}},
		},
		{
			// Three children, two of them with a contract: Mia (80,00 €)
			// is the 2nd, 40,00 €; Noah (60,00 €) the 3rd, 30,00 €.
			name: "the family's number makes the contract children the second and third",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 3, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 4000, contractNoah: 3000},
			lines: map[int64][]reductionOf{
				contractMia:  {{parentbilling.RuleSibling, -4000, 2}},
				contractNoah: {{parentbilling.RuleSibling, -3000, 3}},
			},
		},
		{
			// The family states one child, but two have a contract: the
			// rule counts at least those two. Noah is the 2nd: 30,00 €.
			name: "a number below the contract children counts the contracts",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 1, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 8000, contractNoah: 3000},
			lines:    map[int64][]reductionOf{contractNoah: {{parentbilling.RuleSibling, -3000, 2}}},
		},
		{
			// A rule that does not count the family keeps counting the
			// contracts: Mia alone pays 80,00 €.
			name: "a rule without the family's number counts the contracts",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules:    familySiblingRule(false),
			families: confirmedFamily(familyEva, 3, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 8000},
		},
		{
			// The confirmation ended in December: in January the rule
			// counts Mia alone and the run names the family as uncounted.
			name: "an expired number counts the contracts",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules:     familySiblingRule(true),
			families:  confirmedFamily(familyEva, 3, "2026-09-01", ptr(calendar.NewDate(2026, 12, 31))),
			uncounted: 1,
			totals:    map[int64]int64{contractMia: 8000},
		},
		{
			// A confirmation from the 16th holds for the month, like an
			// evidence that starts within it.
			name: "a number confirmed within the month holds for it",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 2, "2027-01-16", nil),
			totals:   map[int64]int64{contractMia: 4000},
		},
		{
			// The parents stated three, the Abrechnung has not confirmed
			// it: Mia counts alone and the family is uncounted.
			name: "a number the parents only stated does not count",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules: familySiblingRule(true),
			families: func(*testing.T) []parentbilling.FamilyChildCount {
				return []parentbilling.FamilyChildCount{{FamilyEmail: familyEva, StatedChildren: ptr(3), StatedAt: ptr(time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC))}}
			},
			uncounted: 1,
			totals:    map[int64]int64{contractMia: 8000},
		},
		{
			// Another family's number never reaches Eva's family.
			name: "another family's number does not count",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
			},
			rules:     familySiblingRule(true),
			families:  confirmedFamily(familyOther, 3, "2026-09-01", nil),
			uncounted: 1,
			totals:    map[int64]int64{contractMia: 8000},
		},
		{
			// Each location apart (SJR 08.11.2026): the rule counts only
			// Ispringen. The family states two children, Mia at Ispringen
			// and Noah at Birkenfeld. Noah is no sibling at Ispringen, so
			// Mia stays the 1st child: 40,00 €. Noah pays 45,00 €, no rule
			// at Birkenfeld.
			name: "a sibling at the other location does not count for the family's number",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 2, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 4000, contractNoah: 4500},
		},
		{
			// Noah books Tuesday at Birkenfeld, which has no price there: his
			// contract gets no draft but still places him at Birkenfeld. Two
			// stated less Noah: Mia stays the 1st at 40,00 €.
			name: "a sibling at the other location without a price still does not count",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{tue1400}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 2, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 4000},
		},
		{
			// Noah has two contracts at Birkenfeld for the same Monday: both
			// are held, yet he has a contract there. Mia stays the 1st.
			name: "a sibling at the other location held for a double booking still does not count",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
				{contractLea, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			sameChild: map[int64]int64{contractLea: contractNoah},
			rules:     familySiblingRule(true),
			families:  confirmedFamily(familyEva, 2, "2026-09-01", nil),
			totals:    map[int64]int64{contractMia: 4000},
		},
		{
			// Three children stated: Mia, Noah at Birkenfeld and one sibling
			// without a contract. At Ispringen 3 − 1 = 2 count, the sibling
			// without a contract first: Mia is the 2nd, 40,00 − 50 % =
			// 20,00 €.
			name: "a sibling without a contract counts at each location",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			rules:    familySiblingRule(true),
			families: confirmedFamily(familyEva, 3, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 2000, contractNoah: 4500},
			lines:    map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -2000, 2}}},
		},
		{
			// The rule's setting „Geschwister zählen an“ names both schools:
			// Noah counts. Two stated, two with a contract; Noah (45,00 €)
			// is the 1st, Mia (40,00 €) the 2nd: 20,00 €.
			name: "a rule that counts both schools counts the sibling at the other location",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
				{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
			},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) {
						halfFromSecond(rule)
						rule.CountsFamilyChildren = true
						rule.CountSchoolIDs = []int64{schoolIspringen, schoolBirkenfeld}
					})}
			},
			families: confirmedFamily(familyEva, 2, "2026-09-01", nil),
			totals:   map[int64]int64{contractMia: 2000, contractNoah: 4500},
			lines:    map[int64][]reductionOf{contractMia: {{parentbilling.RuleSibling, -2000, 2}}},
		},
		{
			// An earlier run issued Mia's invoice at 80,00 € before the
			// family's number of three was confirmed. The issued invoice
			// stays as it is; Noah, billed now, is the 3rd: 30,00 €.
			name: "an issued invoice stays when the number comes later",
			children: []careChainChild{
				{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}},
				{contractNoah, schoolIspringen, familyOther, "2015-03-01", []parentbilling.CareContractService{mon1630}},
			},
			invoiced:     []int64{contractMia},
			sameFamily:   map[int64]string{contractNoah: familyEva},
			rules:        familySiblingRule(true),
			families:     confirmedFamily(familyEva, 3, "2026-09-01", nil),
			afterRelease: true,
			totals:       map[int64]int64{contractNoah: 3000},
			lines:        map[int64][]reductionOf{contractNoah: {{parentbilling.RuleSibling, -3000, 3}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst, tariffs: chainTariffs(t),
				invoiced: map[int64]parentbilling.CareInvoice{}, recipients: map[int64]parentbilling.CareRecipient{}, rules: tc.rules(t)}
			for _, child := range tc.children {
				version := sjrVersion(t, child.contract, child.school, 1, billingMonth, "2027-01-31", child.booked...)
				born := day(t, child.born)
				version.ChildBirthDate = &born
				in.versions = append(in.versions, version)
				in.recipients[child.contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: child.family}
			}
			for contract, like := range tc.sameChild {
				for i := range in.versions {
					if in.versions[i].ContractID == contract {
						student := like + 9000
						in.versions[i].StudentID = &student
					}
				}
			}
			if tc.evidences != nil {
				in.evidences = tc.evidences(t)
			}
			if tc.families != nil && !tc.afterRelease {
				in.familyCounts = tc.families(t)
			}
			if len(tc.invoiced) > 0 {
				prior, err := buildCareMonth(in)
				require.NoError(t, err)
				for _, invoice := range prior.invoices {
					for _, contract := range tc.invoiced {
						if invoice.ContractID == contract {
							in.invoiced[contract] = invoice
						}
					}
				}
			}
			if tc.families != nil && tc.afterRelease {
				in.familyCounts = tc.families(t)
			}
			for contract, family := range tc.sameFamily {
				in.recipients[contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: family}
			}
			issued := deepCopyInvoices(in.invoiced)
			month, err := buildCareMonth(in)
			require.NoError(t, err)
			assert.Equal(t, issued, in.invoiced, "an issued invoice never changes")
			assert.Equal(t, tc.uncounted, month.uncounted, "families a rule counts without a confirmed number")
			for family, schools := range month.uncountedSchools {
				assert.NotEmpty(t, schools, "family %s names the schools the rule reduces", family)
			}
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

func TestCareCapKeepsIssuedAmounts(t *testing.T) {
	t.Parallel()
	issued := parentbilling.CareInvoice{ContractID: contractMia, SchoolID: schoolIspringen,
		Recipient: parentbilling.CareRecipient{Email: familyEva}, TotalCents: 8000,
		Positions: []parentbilling.CarePosition{{CareOfferingID: moduleUntil1400, AmountCents: 8000, PayableCents: 8000}}}
	draft := parentbilling.CareInvoice{ContractID: contractNoah, SchoolID: schoolIspringen,
		Recipient: issued.Recipient, TotalCents: 6000,
		Positions: []parentbilling.CarePosition{{CareOfferingID: moduleUntil1630, AmountCents: 6000}}}
	rules := []parentbilling.ContributionRule{
		careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
			rule.DiscountBasisPoints = ptr(5000)
		}),
		careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
			rule.CapCents = ptr(int64(10000))
		}),
	}
	applyCareChain([]*careChainEntry{{invoice: &issued, child: "mia", issued: true}, {invoice: &draft, child: "noah"}},
		careChainInput{month: day(t, billingMonth), rules: rules,
			evidences: []parentbilling.CareEvidence{evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), billingMonth, nil)}})
	assert.Equal(t, int64(8000), issued.TotalCents)
	assert.Empty(t, issued.Reductions)
	assert.Equal(t, int64(2000), draft.TotalCents)
}

func TestCareSiblingFixedAmountsApplyOncePerChild(t *testing.T) {
	t.Parallel()
	for _, effect := range []string{"price", "amount"} {
		t.Run(effect, func(t *testing.T) {
			t.Parallel()
			rule := careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
				rule.SiblingFromChild = ptr(2)
				if effect == "price" {
					rule.PriceCents = ptr(int64(2500))
				} else {
					rule.DiscountCents = ptr(int64(2500))
				}
			})
			invoices := []parentbilling.CareInvoice{
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 10000}}},
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 4000}}},
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 4001}}},
			}
			entries := []*careChainEntry{{invoice: &invoices[0], child: "first"}, {invoice: &invoices[1], child: "second"}, {invoice: &invoices[2], child: "second"}}
			applyCareChain(entries, careChainInput{month: day(t, billingMonth), rules: []parentbilling.ContributionRule{rule}})
			want := int64(2500)
			if effect == "amount" {
				want = 5501
			}
			assert.Equal(t, want, invoices[1].TotalCents+invoices[2].TotalCents)
			assert.Equal(t, int64(10000), invoices[0].TotalCents)
		})
	}
}

func TestCareMonthUsesIssuedScopeWithoutCurrentPrices(t *testing.T) {
	t.Parallel()
	capRule := careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
		rule.CapCents = ptr(int64(5000))
		rule.CareOfferingIDs = []int64{moduleUntil1400}
	})
	issued := parentbilling.CareInvoice{ContractID: contractMia, SchoolID: schoolIspringen,
		Recipient: parentbilling.CareRecipient{Email: familyEva}, TotalCents: 8000,
		Positions: []parentbilling.CarePosition{
			{CareOfferingID: moduleUntil1400, AmountCents: 8000, PayableCents: 4000},
			{CareOfferingID: moduleUntil1630, AmountCents: 4000, PayableCents: 4000},
		}}
	month, err := buildCareMonth(careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{sjrVersion(t, contractNoah, schoolIspringen, 1, billingMonth, "2027-01-31", mon1400)},
		tariffs:  chainTariffs(t), invoiced: map[int64]parentbilling.CareInvoice{contractMia: issued},
		recipients: map[int64]parentbilling.CareRecipient{contractNoah: {Email: familyEva}}, rules: []parentbilling.ContributionRule{capRule},
	})
	require.NoError(t, err)
	require.Len(t, month.invoices, 1)
	assert.Equal(t, int64(1000), month.invoices[0].TotalCents)
	assert.Equal(t, 1, month.billed)
	assert.Equal(t, int64(4000), issued.Positions[0].PayableCents)
}

func TestCareSiblingFixedAmountsCountEarlierContractsOfTheChild(t *testing.T) {
	t.Parallel()
	for _, effect := range []string{"price", "amount"} {
		t.Run(effect, func(t *testing.T) {
			t.Parallel()
			rule := careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
				rule.SiblingFromChild = ptr(2)
				if effect == "price" {
					rule.PriceCents = ptr(int64(2500))
				} else {
					rule.DiscountCents = ptr(int64(2500))
				}
			})
			issuedNet, want := int64(2500), int64(0)
			if effect == "amount" {
				issuedNet, want = 1500, 4000
			}
			invoices := []parentbilling.CareInvoice{
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 10000}}},
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, TotalCents: issuedNet,
					Positions:  []parentbilling.CarePosition{{AmountCents: 4000, PayableCents: issuedNet}},
					Reductions: []parentbilling.CareReduction{{Kind: parentbilling.RuleSibling, RuleID: rule.ID, AmountCents: issuedNet - 4000}}},
				{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 4000}}},
			}
			applyCareChain([]*careChainEntry{{invoice: &invoices[0], child: "first"}, {invoice: &invoices[1], child: "second", issued: true}, {invoice: &invoices[2], child: "second"}},
				careChainInput{month: day(t, billingMonth), rules: []parentbilling.ContributionRule{rule}})
			assert.Equal(t, want, invoices[2].TotalCents)
			assert.Equal(t, issuedNet, invoices[1].TotalCents)
		})
	}
}

// A later rule of the chain lowers an issued invoice below its amount at the
// sibling stage. The fixed price still counts that amount: the child paid
// 25,00 € at the sibling stage (12,50 € after 50 % for single parents), so a
// later contract of the same child pays nothing more under the price.
func TestCareSiblingFixedPriceCountsTheIssuedAmountAtTheSiblingStage(t *testing.T) {
	t.Parallel()
	rule := careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
		rule.SiblingFromChild, rule.PriceCents = ptr(2), ptr(int64(2500))
	})
	invoices := []parentbilling.CareInvoice{
		{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 10000}}},
		{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, TotalCents: 1250,
			Positions: []parentbilling.CarePosition{{AmountCents: 4000, PayableCents: 1250}},
			Reductions: []parentbilling.CareReduction{
				{Kind: parentbilling.RuleSibling, RuleID: rule.ID, AmountCents: -1500},
				{Kind: parentbilling.RuleSingleParent, RuleID: ruleSingleParent, AmountCents: -1250},
			}},
		{SchoolID: schoolIspringen, Recipient: parentbilling.CareRecipient{Email: familyEva}, Positions: []parentbilling.CarePosition{{AmountCents: 4000}}},
	}
	applyCareChain([]*careChainEntry{{invoice: &invoices[0], child: "first"}, {invoice: &invoices[1], child: "second", issued: true}, {invoice: &invoices[2], child: "second"}},
		careChainInput{month: day(t, billingMonth), rules: []parentbilling.ContributionRule{rule}})
	assert.Equal(t, int64(0), invoices[2].TotalCents)
	assert.Equal(t, int64(1250), invoices[1].TotalCents)
}

// familySiblingRule is half from the second child at Ispringen that counts
// the family's number or not (#5918).
func familySiblingRule(counts bool) func(t *testing.T) []parentbilling.ContributionRule {
	return func(t *testing.T) []parentbilling.ContributionRule {
		return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
			func(rule *parentbilling.ContributionRule) { halfFromSecond(rule); rule.CountsFamilyChildren = counts })}
	}
}

// confirmedFamily is a number the Abrechnung confirmed from from on.
func confirmedFamily(family string, children int, from string, until *calendar.Date) func(t *testing.T) []parentbilling.FamilyChildCount {
	return func(t *testing.T) []parentbilling.FamilyChildCount {
		return []parentbilling.FamilyChildCount{{FamilyEmail: family, ConfirmedChildren: ptr(children),
			ValidFrom: ptr(day(t, from)), ValidUntil: until, ConfirmedAt: ptr(time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC))}}
	}
}

// deepCopyInvoices copies the issued invoices with their positions and
// reductions, so a change by the run shows.
func deepCopyInvoices(invoices map[int64]parentbilling.CareInvoice) map[int64]parentbilling.CareInvoice {
	copied := make(map[int64]parentbilling.CareInvoice, len(invoices))
	for id, invoice := range invoices {
		invoice.Positions = slices.Clone(invoice.Positions)
		invoice.Reductions = slices.Clone(invoice.Reductions)
		copied[id] = invoice
	}
	return copied
}
