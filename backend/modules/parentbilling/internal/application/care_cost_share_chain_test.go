package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The cost bearer shares after the rule chain (#4687, ADR 0169), computed by
// hand on the prices of TestCareMonthRuleChainTable: Mia books Mon and Tue
// until 14:00 (80,00 €), Noah Mon until 16:30 (60,00 €), Lea the early care
// on Monday (33,33 €), all at Ispringen in January 2027.

const (
	bearerJobcenter   int64 = 801
	bearerYouthOffice int64 = 802
	bearerFoundation  int64 = 803
)

func costShare(t *testing.T, contract int64, kind string, bearer int64, from, until string, set func(*parentbilling.CareCostShare)) parentbilling.CareCostShare {
	t.Helper()
	share := parentbilling.CareCostShare{ContractID: contract, Kind: kind, CostBearerID: bearer,
		CostBearerName: map[int64]string{bearerJobcenter: "Jobcenter Enzkreis", bearerYouthOffice: "Jugendamt Enzkreis", bearerFoundation: "Stiftung Kinderglück"}[bearer],
		ValidFrom:      day(t, from), ValidUntil: day(t, until), Reference: "Bescheid 2026/17", CheckedOn: day(t, "2026-12-01")}
	set(&share)
	return share
}

func basisPoints(points int) func(*parentbilling.CareCostShare) {
	return func(share *parentbilling.CareCostShare) { share.ShareBasisPoints = ptr(points) }
}

func monthly(cents int64) func(*parentbilling.CareCostShare) {
	return func(share *parentbilling.CareCostShare) { share.ShareCents = ptr(cents) }
}

// shareOf is a share of an invoice as the table compares it.
type shareOf struct {
	bearer int64
	amount int64
}

// splitOf is what the table expects of one invoice: the contribution after
// the rules, the shares in the order of the chain and the family's rest.
type splitOf struct {
	contribution int64
	shares       []shareOf
	family       int64
}

func TestCareCostSharesFollowTheRuleChain(t *testing.T) {
	t.Parallel()
	mia := careChainChild{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400, tue1400}}
	noah := careChainChild{contractNoah, schoolIspringen, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1630}}
	lea := careChainChild{contractLea, schoolIspringen, familyOther, "2018-03-01", []parentbilling.CareContractService{monEarly}}
	noRules := func(*testing.T) []parentbilling.ContributionRule { return nil }
	cases := []struct {
		name      string
		children  []careChainChild
		invoiced  []int64
		rules     func(t *testing.T) []parentbilling.ContributionRule
		evidences func(t *testing.T) []parentbilling.CareEvidence
		shares    func(t *testing.T) []parentbilling.CareCostShare
		want      map[int64]splitOf
	}{
		{
			// No rule: 80,00 €, Bildung und Teilhabe takes 100 %, the family pays nothing.
			name: "full takeover without rules", children: []careChainChild{mia}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractMia, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-07-31", basisPoints(10000))}
			},
			want: map[int64]splitOf{contractMia: {8000, []shareOf{{bearerJobcenter, 8000}}, 0}},
		},
		{
			// Income level 20 %: 80,00 − 16,00 = 64,00 €. The Jugendamt pays 50 % of
			// that, 32,00 €, the family the other 32,00 €.
			name: "income level, then half by the Jugendamt", children: []careChainChild{mia},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleIncome, parentbilling.RuleIncomeLevel, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.DiscountBasisPoints = ptr(2000) })}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{evidence(t, contractMia, parentbilling.EvidenceIncomeLevel, ptr(ruleIncome), "2026-09-01", nil)}
			},
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-07-31", basisPoints(5000))}
			},
			want: map[int64]splitOf{contractMia: {6400, []shareOf{{bearerYouthOffice, 3200}}, 3200}},
		},
		{
			// Mia 80,00 € is the first child, Noah 60,00 − 50 % = 30,00 € the second.
			// The Jugendamt takes Noah's 30,00 €; the share changes neither the rank
			// nor Mia's 80,00 €.
			name: "sibling discount, then the share of the second child", children: []careChainChild{mia, noah},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, halfFromSecond)}
			},
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractNoah, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-07-31", basisPoints(10000))}
			},
			want: map[int64]splitOf{
				contractMia:  {8000, []shareOf{}, 8000},
				contractNoah: {3000, []shareOf{{bearerYouthOffice, 3000}}, 0},
			},
		},
		{
			// Single parent 10,00 € off: 70,00 €. A foundation pays 25,00 € a month,
			// the family 45,00 €.
			name: "single parent, then a fixed subsidy", children: []careChainChild{mia},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleSingleParent, parentbilling.RuleSingleParent, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.DiscountCents = ptr(int64(1000)) })}
			},
			evidences: func(t *testing.T) []parentbilling.CareEvidence {
				return []parentbilling.CareEvidence{evidence(t, contractMia, parentbilling.EvidenceSingleParent, nil, "2026-09-01", nil)}
			},
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractMia, parentbilling.CostShareOtherSubsidy, bearerFoundation, "2026-08-01", "2027-07-31", monthly(2500))}
			},
			want: map[int64]splitOf{contractMia: {7000, []shareOf{{bearerFoundation, 2500}}, 4500}},
		},
		{
			// 80,00 + 60,00 = 140,00 € against a cap of 100,00 €: the 40,00 € excess
			// comes off Noah, the last child: 20,00 €. The cap counts the whole
			// contribution, not what the family pays after a share. Bildung und
			// Teilhabe pays half of Noah's 20,00 €.
			name: "cap, then half of the rest", children: []careChainChild{mia, noah},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.CapCents = ptr(int64(10000)) })}
			},
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractNoah, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-07-31", basisPoints(5000))}
			},
			want: map[int64]splitOf{
				contractMia:  {8000, []shareOf{}, 8000},
				contractNoah: {2000, []shareOf{{bearerJobcenter, 1000}}, 1000},
			},
		},
		{
			// Mia's earlier invoice of the month is issued: Bildung und Teilhabe paid
			// all 80,00 €, the family nothing. Noah's draft still meets the cap of
			// 100,00 € with Mia's whole 80,00 €: 60,00 − 40,00 = 20,00 €. A cap on
			// the family's rest would have left Noah at 60,00 €.
			name: "an issued share still counts toward the cap", children: []careChainChild{mia, noah}, invoiced: []int64{contractMia},
			rules: func(t *testing.T) []parentbilling.ContributionRule {
				return []parentbilling.ContributionRule{careRule(t, ruleCap, parentbilling.RuleCap, []int64{schoolIspringen},
					func(rule *parentbilling.ContributionRule) { rule.CapCents = ptr(int64(10000)) })}
			},
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractMia, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-07-31", basisPoints(10000))}
			},
			want: map[int64]splitOf{contractNoah: {2000, []shareOf{}, 2000}},
		},
		{
			// 33,33 € and 50 %: 1666,5 cent rounds half up to 16,67 €; the family pays
			// 16,66 €. The parts add up to the cent.
			name: "half of an odd cent amount", children: []careChainChild{lea}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractLea, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-07-31", basisPoints(5000))}
			},
			want: map[int64]splitOf{contractLea: {3333, []shareOf{{bearerYouthOffice, 1667}}, 1666}},
		},
		{
			// Two halves of 33,33 €: Bildung und Teilhabe 16,67 €, the Jugendamt only
			// the 16,66 € that are left. The family pays nothing, never less.
			name: "two halves never take more than the contribution", children: []careChainChild{lea}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{
					costShare(t, contractLea, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-07-31", basisPoints(5000)),
					costShare(t, contractLea, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-07-31", basisPoints(5000)),
				}
			},
			want: map[int64]splitOf{contractLea: {3333, []shareOf{{bearerJobcenter, 1667}, {bearerYouthOffice, 1666}}, 0}},
		},
		{
			// A subsidy of 100,00 € a month on 80,00 €: it pays 80,00 €.
			name: "a fixed share above the contribution", children: []careChainChild{mia}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{costShare(t, contractMia, parentbilling.CostShareOtherSubsidy, bearerFoundation, "2026-08-01", "2027-07-31", monthly(10000))}
			},
			want: map[int64]splitOf{contractMia: {8000, []shareOf{{bearerFoundation, 8000}}, 0}},
		},
		{
			// All three kinds in the order of the chain on 80,00 €: Bildung und
			// Teilhabe 20,00 €, the Jugendamt 50 % = 40,00 €, the foundation 10 % =
			// 8,00 €. The family pays 12,00 €.
			name: "three kinds in their order", children: []careChainChild{mia}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{
					costShare(t, contractMia, parentbilling.CostShareOtherSubsidy, bearerFoundation, "2026-08-01", "2027-07-31", basisPoints(1000)),
					costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-07-31", basisPoints(5000)),
					costShare(t, contractMia, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-07-31", monthly(2000)),
				}
			},
			want: map[int64]splitOf{contractMia: {8000, []shareOf{{bearerJobcenter, 2000}, {bearerYouthOffice, 4000}, {bearerFoundation, 800}}, 1200}},
		},
		{
			// A share that ended in December does not reach January; one that
			// starts on 15 January holds for the month, like an evidence.
			name: "validity like an evidence", children: []careChainChild{mia, lea}, rules: noRules,
			shares: func(t *testing.T) []parentbilling.CareCostShare {
				return []parentbilling.CareCostShare{
					costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2026-12-31", basisPoints(10000)),
					costShare(t, contractLea, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2027-01-15", "2027-07-31", basisPoints(10000)),
				}
			},
			want: map[int64]splitOf{
				contractMia: {8000, []shareOf{}, 8000},
				contractLea: {3333, []shareOf{{bearerYouthOffice, 3333}}, 0},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst, tariffs: chainTariffs(t),
				invoiced: map[int64]parentbilling.CareInvoice{}, recipients: map[int64]parentbilling.CareRecipient{},
				rules: tc.rules(t), costShares: tc.shares(t)}
			for _, child := range tc.children {
				version := sjrVersion(t, child.contract, child.school, 1, billingMonth, "2027-01-31", child.booked...)
				born := day(t, child.born)
				version.ChildBirthDate = &born
				in.versions = append(in.versions, version)
				in.recipients[child.contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: child.family}
			}
			if tc.evidences != nil {
				in.evidences = tc.evidences(t)
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
			month, err := buildCareMonth(in)
			require.NoError(t, err)
			got := map[int64]splitOf{}
			for _, invoice := range month.invoices {
				var payable int64
				for _, position := range invoice.Positions {
					payable += position.PayableCents
				}
				shares := []shareOf{}
				for _, share := range invoice.CostShares {
					shares = append(shares, shareOf{share.CostBearerID, share.AmountCents})
				}
				got[invoice.ContractID] = splitOf{invoice.ContributionCents(), shares, invoice.TotalCents}
				assert.Equal(t, payable, invoice.ContributionCents(), "the shares leave the payable cents of the positions alone")
				assert.GreaterOrEqual(t, invoice.TotalCents, int64(0), "the family never pays less than nothing")
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCareCostShareStatusWarnsBeforeTheEnd(t *testing.T) {
	t.Parallel()
	today := day(t, "2027-01-10")
	ending := costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-02-09", basisPoints(10000))
	later := costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2026-08-01", "2027-02-10", basisPoints(10000))
	followUp := costShare(t, contractMia, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2027-02-10", "2027-07-31", basisPoints(10000))
	otherKind := costShare(t, contractMia, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2027-02-10", "2027-07-31", basisPoints(10000))
	ahead := costShare(t, contractNoah, parentbilling.CostShareYouthOffice, bearerYouthOffice, "2027-01-11", "2027-07-31", basisPoints(10000))
	over := costShare(t, contractNoah, parentbilling.CostShareEducationParticipation, bearerJobcenter, "2026-08-01", "2027-01-09", basisPoints(10000))

	assert.Equal(t, parentbilling.CostShareExpiring, costShareStatus(ending, []parentbilling.CareCostShare{ending, otherKind}, today),
		"ends within 30 days; a share of another kind is no follow-up")
	assert.Equal(t, parentbilling.CostShareActive, costShareStatus(ending, []parentbilling.CareCostShare{ending, followUp}, today),
		"a follow-up of the same kind ends the warning")
	assert.Equal(t, parentbilling.CostShareActive, costShareStatus(later, nil, today), "31 days ahead does not warn yet")
	assert.Equal(t, parentbilling.CostShareUpcoming, costShareStatus(ahead, nil, today))
	assert.Equal(t, parentbilling.CostShareExpired, costShareStatus(over, nil, today))
}
