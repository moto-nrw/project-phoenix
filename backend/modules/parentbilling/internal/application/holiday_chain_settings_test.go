package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The choices of a Träger in the chain (#5664, ADR 0115 addendum): which child
// is the first child and whether a cost bearer pays before the sibling
// discount. Their defaults are pinned by the tests of the chain, which name no
// choice.

func halfFromSecondChild() *parentbilling.ContributionRule {
	return &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
}

func bornOn(child parentbilling.HolidayChild, birthDate calendar.Date) parentbilling.HolidayChild {
	child.BirthDate = birthDate
	return child
}

func oldestFirstRules() holidayRules {
	return holidayRules{sibling: halfFromSecondChild(), chain: parentbilling.HolidayChainSettings{SiblingRank: parentbilling.SiblingRankOldestChild}}
}

func beforeDiscountRules() holidayRules {
	return holidayRules{sibling: halfFromSecondChild(), chain: parentbilling.HolidayChainSettings{CostTakeoverOrder: parentbilling.CostTakeoverBeforeDiscount}}
}

func TestOldestChildPaysInFullEvenWhenTheYoungerIsDearer(t *testing.T) {
	t.Parallel()
	younger := bornOn(chainChild(1, 1, 6750), calendar.NewDate(2018, 3, 1))
	older := bornOn(chainChild(2, 2, 4500), calendar.NewDate(2016, 9, 12))

	result := computeHolidayContribution([]parentbilling.HolidayChild{younger, older}, oldestFirstRules(), false, nil)
	assert.Equal(t, int64(4500), result.Children[1].TotalCents, "the older child is the first child")
	require.Len(t, result.Children[0].Lines, 2)
	assert.Equal(t, int64(-3375), result.Children[0].Lines[1].AmountCents)
	assert.Equal(t, 2, *result.Children[0].Lines[1].ChildRank)

	standard := computeHolidayContribution([]parentbilling.HolidayChild{younger, older}, holidayRules{sibling: halfFromSecondChild()}, false, nil)
	assert.Equal(t, int64(6750), standard.Children[0].TotalCents, "without a choice the dearer child stays the first child")
	assert.Equal(t, int64(2250), standard.Children[1].TotalCents)
	assert.Equal(t, parentbilling.SiblingRankHighestContribution, standard.Chain.SiblingRank)
	assert.Equal(t, parentbilling.CostTakeoverAfterDiscount, standard.Chain.CostTakeoverOrder)
}

func TestSiblingsOfTheSameAgeRankByRegistrationThenForm(t *testing.T) {
	t.Parallel()
	twin := calendar.NewDate(2017, 5, 5)
	earlier := holidayMember{children: []parentbilling.HolidayChild{bornOn(chainChild(3, 2, 4500), twin), bornOn(chainChild(2, 1, 4500), twin)}}
	later := holidayMember{children: []parentbilling.HolidayChild{bornOn(chainChild(1, 1, 9000), twin)}}

	results := computeHolidaySiblings([]holidayMember{earlier, later}, oldestFirstRules())
	assert.Equal(t, int64(4500), results[0].Children[1].TotalCents, "the first child of the form of the earlier registration pays in full")
	assert.Equal(t, int64(2250), results[0].Children[0].TotalCents)
	assert.Equal(t, int64(4500), results[1].Children[0].TotalCents, "the later registration comes after it, however dear")
}

func TestQuoteRanksByTheOrderTheFormSends(t *testing.T) {
	t.Parallel()
	younger, older := chainChild(0, 1, 6750), chainChild(0, 2, 4500)
	younger.AgeRank, older.AgeRank = 2, 1
	unknown := chainChild(0, 3, 9000)

	result := computeHolidayContribution([]parentbilling.HolidayChild{younger, older, unknown}, oldestFirstRules(), false, nil)
	assert.Equal(t, int64(4500), result.Children[1].TotalCents)
	assert.Equal(t, int64(3375), result.Children[0].TotalCents)
	assert.Equal(t, 3, *result.Children[2].Lines[1].ChildRank, "a child without a date comes last")

	weeks := []parentbilling.HolidayWeek{{ID: 1, TimeModels: []parentbilling.HolidayTimeModel{{ID: 2, PriceCents: 4500}}}}
	priced, err := quotedChildren(weeks, []parentbilling.HolidayQuoteChild{{AgeRank: 1}, {AgeRank: 1}})
	require.NoError(t, err)
	assert.Equal(t, 1, priced[1].AgeRank, "twins share a rank")
	for _, rank := range []int{-1, 3} {
		_, err = quotedChildren(weeks, []parentbilling.HolidayQuoteChild{{AgeRank: rank}, {AgeRank: 1}})
		assert.ErrorIs(t, err, parentbilling.ErrInvalidHolidayQuote, "age rank %d", rank)
	}
}

func TestCostBearerPaysTheFullPriceBeforeTheDiscount(t *testing.T) {
	t.Parallel()
	bearer := youthOfficeBearer()
	first := chainChild(1, 1, 10000)
	second := chainChild(2, 2, 10000)

	result := computeHolidayContribution([]parentbilling.HolidayChild{first, second}, beforeDiscountRules(), false, bearer)
	assert.Equal(t, int64(0), result.TotalCents)
	for _, child := range result.Children {
		require.Len(t, child.Lines, 2, "no child of the registration gets a discount")
		assert.Equal(t, parentbilling.LineCostTakeover, child.Lines[1].Kind)
		assert.Equal(t, int64(-10000), child.Lines[1].AmountCents)
	}

	after := computeHolidayContribution([]parentbilling.HolidayChild{first, second}, holidayRules{sibling: halfFromSecondChild()}, false, bearer)
	assert.Equal(t, int64(-5000), after.Children[1].Lines[2].AmountCents, "by default the cost bearer pays what the discount leaves")
}

func TestBeforeTheDiscountTheChildOfTheCostBearerKeepsItsRank(t *testing.T) {
	t.Parallel()
	bearer := youthOfficeBearer()
	dearer := holidayMember{children: []parentbilling.HolidayChild{chainChild(1, 1, 6750)}, bearer: bearer}
	cheaper := holidayMember{children: []parentbilling.HolidayChild{chainChild(2, 1, 4500)}}

	results := computeHolidaySiblings([]holidayMember{dearer, cheaper}, beforeDiscountRules())
	assert.Equal(t, int64(-6750), results[0].Children[0].Lines[1].AmountCents, "the cost bearer pays the full price")
	assert.Equal(t, int64(2250), results[1].TotalCents, "the other family keeps the discount of the second child")

	dearer.bearer, cheaper.bearer = nil, bearer
	results = computeHolidaySiblings([]holidayMember{dearer, cheaper}, beforeDiscountRules())
	assert.Equal(t, int64(6750), results[0].TotalCents, "the first child still pays in full")
	require.Len(t, results[1].Children[0].Lines, 2, "the second child of the cost bearer gets no discount")
	assert.Equal(t, int64(-4500), results[1].Children[0].Lines[1].AmountCents)
}

func TestBeforeTheDiscountAnExemptionStillFollowsTheDiscount(t *testing.T) {
	t.Parallel()
	rules := beforeDiscountRules()
	rules.exemption = &parentbilling.ContributionRule{Kind: parentbilling.RuleBenefitsExemption, Note: "Befreit"}
	result := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 10000), chainChild(2, 2, 10000)}, rules, true, nil)
	require.Len(t, result.Children[1].Lines, 3)
	assert.Equal(t, parentbilling.LineSibling, result.Children[1].Lines[1].Kind)
	assert.Equal(t, int64(-5000), result.Children[1].Lines[2].AmountCents)
}

func TestGroupKeepsTheChoicesItFroze(t *testing.T) {
	t.Parallel()
	old := parentbilling.HolidayChainSettings{SiblingRank: parentbilling.SiblingRankHighestContribution, CostTakeoverOrder: parentbilling.CostTakeoverAfterDiscount}
	current := parentbilling.HolidayChainSettings{SiblingRank: parentbilling.SiblingRankOldestChild, CostTakeoverOrder: parentbilling.CostTakeoverBeforeDiscount}
	frozen := func(chain parentbilling.HolidayChainSettings) siblingMember {
		return siblingMember{frozen: &parentbilling.HolidayContribution{Chain: chain}}
	}

	assert.Equal(t, current, groupChain([]siblingMember{{}, {}}, current), "nothing frozen takes the current choices")
	assert.Equal(t, old, groupChain([]siblingMember{{}, frozen(old), frozen(old)}, current), "a frozen contribution keeps its choices")
	assert.Equal(t, current, groupChain([]siblingMember{frozen(old), frozen(current)}, current), "different choices merged take the current ones")
}

func TestFrozenSiblingRanksWithTheBirthDateOfItsRegistration(t *testing.T) {
	t.Parallel()
	born := calendar.NewDate(2016, 9, 12)
	member := siblingMember{
		registration: parentbilling.HolidayRegistration{Children: []parentbilling.HolidayChild{{ID: 4, BirthDate: born}}},
		frozen:       &parentbilling.HolidayContribution{Children: []parentbilling.HolidayChildContribution{{ChildID: 4, Position: 1}}},
	}
	assert.Equal(t, born, member.chainMember().children[0].BirthDate)
}

type chainSettingsDouble struct {
	parentbilling.HolidayBillingSettings
	chain parentbilling.HolidayChainSettings
	err   error
}

func (d chainSettingsDouble) HolidayChain(context.Context, int64) (parentbilling.HolidayChainSettings, error) {
	return d.chain, d.err
}

func TestUnreadableOrUnknownChoicesRefuseInsteadOfBillingAnotherWay(t *testing.T) {
	t.Parallel()
	known := parentbilling.HolidayChainSettings{SiblingRank: parentbilling.SiblingRankOldestChild, CostTakeoverOrder: parentbilling.CostTakeoverBeforeDiscount}
	failure := errors.New("settings down")
	for name, double := range map[string]chainSettingsDouble{
		"read error":    {err: failure},
		"unknown rank":  {chain: parentbilling.HolidayChainSettings{SiblingRank: "youngest", CostTakeoverOrder: parentbilling.CostTakeoverAfterDiscount}},
		"unknown order": {chain: parentbilling.HolidayChainSettings{SiblingRank: parentbilling.SiblingRankOldestChild, CostTakeoverOrder: "never"}},
	} {
		_, err := (&HolidayContributions{settings: double}).chainSettings(context.Background(), 1)
		assert.Error(t, err, name)
	}
	chain, err := (&HolidayContributions{settings: chainSettingsDouble{chain: known}}).chainSettings(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, known, chain)
}
