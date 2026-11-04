package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rounding and the ranking of the chain are not reachable through the
// API with every edge, so they are pinned here; the chain as a whole is
// tested through the routes in api/parent_billing_api_test.go.

func chainChild(id int64, position int, prices ...int64) parentbilling.HolidayChild {
	child := parentbilling.HolidayChild{ID: id, Position: position}
	for i, price := range prices {
		startsOn := calendar.NewDate(2027, 7, 5).AddDays(7 * i)
		child.Bookings = append(child.Bookings, parentbilling.HolidayBooking{
			WeekID: int64(i + 1), WeekStartsOn: startsOn, WeekEndsOn: startsOn.AddDays(4),
			TimeModelID: int64(10 + i), TimeModelName: "bis 16 Uhr", PriceCents: price,
		})
	}
	return child
}

func TestSiblingShareRoundsHalfUpAndRanksByBasePrice(t *testing.T) {
	t.Parallel()
	rule := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(3333)}
	// The first child in the form is the cheaper one; the more expensive
	// second child counts as the first child and pays in full.
	result := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4501), chainChild(2, 2, 6750)}, holidayRules{sibling: rule}, false, nil)

	require.Len(t, result.Children, 2)
	cheaper, dearer := result.Children[0], result.Children[1]
	assert.Equal(t, int64(6750), dearer.TotalCents)
	require.Len(t, cheaper.Lines, 2)
	assert.Equal(t, parentbilling.LineSibling, cheaper.Lines[1].Kind)
	assert.Equal(t, int64(-1500), cheaper.Lines[1].AmountCents, "4501 × 33.33 % = 1500.18 rounds to 1500 cent")
	assert.Equal(t, 2, *cheaper.Lines[1].ChildRank)
	assert.Equal(t, int64(3001+6750), result.TotalCents)

	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	odd := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4500), chainChild(2, 2, 4501)}, holidayRules{sibling: half}, false, nil)
	assert.Equal(t, int64(-2250), odd.Children[0].Lines[1].AmountCents, "equal ranks keep the form order; 4500 / 2 = 2250")
	assert.Len(t, odd.Children[1].Lines, 1, "the dearer child is the first child")
	halfUp := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4501), chainChild(2, 2, 4501)}, holidayRules{sibling: half}, false, nil)
	assert.Equal(t, int64(-2251), halfUp.Children[1].Lines[1].AmountCents, "2250.5 rounds half up to 2251 cent")
}

func TestFixedSiblingDiscountNeverExceedsTheBasePriceAndCountsOnlyBookedChildren(t *testing.T) {
	t.Parallel()
	rule := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(3), DiscountCents: new(int64(5000))}
	children := []parentbilling.HolidayChild{chainChild(1, 1, 6750), chainChild(2, 2), chainChild(3, 3, 4500), chainChild(4, 4, 4500)}
	result := computeHolidayContribution(children, holidayRules{sibling: rule}, false, nil)

	assert.Empty(t, result.Children[1].Lines, "a child without a booking is no sibling")
	assert.Len(t, result.Children[2].Lines, 1, "the third booked child is the first one the rule reaches")
	require.Len(t, result.Children[3].Lines, 2)
	assert.Equal(t, int64(-4500), result.Children[3].Lines[1].AmountCents, "the discount stops at the base price")
	assert.Equal(t, int64(0), result.Children[3].TotalCents)
}

func TestExemptionFreesEveryChildWithItsNoteAndNeedsARule(t *testing.T) {
	t.Parallel()
	exemption := &parentbilling.ContributionRule{Kind: parentbilling.RuleBenefitsExemption, Note: "Beitragsfrei wegen Leistungsbezug"}
	children := []parentbilling.HolidayChild{chainChild(1, 1, 4500, 4500), chainChild(2, 2, 6750)}

	freed := computeHolidayContribution(children, holidayRules{exemption: exemption}, true, nil)
	assert.True(t, freed.ExemptionAvailable)
	assert.True(t, freed.BenefitsExemption)
	assert.Equal(t, int64(0), freed.TotalCents)
	last := freed.Children[0].Lines[len(freed.Children[0].Lines)-1]
	assert.Equal(t, parentbilling.LineBenefitsExemption, last.Kind)
	assert.Equal(t, int64(-9000), last.AmountCents)
	assert.Equal(t, "Beitragsfrei wegen Leistungsbezug", last.Note)

	without := computeHolidayContribution(children, holidayRules{}, true, nil)
	assert.False(t, without.BenefitsExemption, "without a valid rule the statement changes nothing")
	assert.Equal(t, int64(15750), without.TotalCents)
}

func TestRulesApplyOnTheFirstBookedDay(t *testing.T) {
	t.Parallel()
	until := calendar.NewDate(2027, 7, 4)
	rules := []parentbilling.ContributionRule{
		{ID: 1, Service: parentbilling.ServiceHoliday, Kind: parentbilling.RuleSibling, ValidFrom: calendar.NewDate(2027, 1, 1), ValidUntil: &until, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)},
		{ID: 2, Service: parentbilling.ServiceHoliday, Kind: parentbilling.RuleSibling, ValidFrom: calendar.NewDate(2027, 7, 5), SiblingFromChild: new(2), DiscountBasisPoints: new(2500)},
	}
	children := []parentbilling.HolidayChild{chainChild(1, 1, 4500, 4500), chainChild(2, 2, 4500)}
	day := firstHolidayDay(children)
	assert.Equal(t, calendar.NewDate(2027, 7, 5), day)
	valid := validHolidayRules(rules, day)
	require.NotNil(t, valid.sibling)
	assert.Equal(t, rules[1].ID, valid.sibling.ID, "the rule from 5 July applies, the one until 4 July no longer")
	assert.Nil(t, validHolidayRules(rules, calendar.NewDate(2026, 12, 31)).sibling)
}

// TestSiblingRuleRanksAcrossTheRegistrationsOfAGroup pins #5628: the
// registrations of a sibling group count as one family. Two registrations
// with one child of 100 € each and a rule from the second child at 50 %:
// one child pays 100 €, the other 50 €; on a tie the child of the earlier
// registration is the first child; each registration keeps only the lines
// of its own children.
func TestSiblingRuleRanksAcrossTheRegistrationsOfAGroup(t *testing.T) {
	t.Parallel()
	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}

	laterChild := chainChild(2, 1, 10000)
	tie := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000)}},
		{children: []parentbilling.HolidayChild{laterChild}},
	}, holidayRules{sibling: half})
	require.Len(t, tie, 2)
	require.Len(t, tie[0].Children, 1)
	require.Len(t, tie[1].Children, 1)
	assert.Equal(t, int64(10000), tie[0].TotalCents, "the child of the earlier registration is the first child")
	assert.Len(t, tie[0].Children[0].Lines, 1)
	assert.Equal(t, int64(5000), tie[1].TotalCents, "the child of the later registration pays half")
	require.Len(t, tie[1].Children[0].Lines, 2)
	assert.Equal(t, parentbilling.LineSibling, tie[1].Children[0].Lines[1].Kind)
	assert.Equal(t, 2, *tie[1].Children[0].Lines[1].ChildRank)
	assert.Equal(t, laterChild.ID, tie[1].Children[0].ChildID, "a registration holds only its own child")

	dearer := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000)}},
		{children: []parentbilling.HolidayChild{chainChild(2, 1, 20000)}},
	}, holidayRules{sibling: half})
	assert.Equal(t, int64(5000), dearer[0].TotalCents, "the higher contribution of the later registration ranks first")
	assert.Equal(t, int64(20000), dearer[1].TotalCents)

	four := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000), chainChild(2, 2, 10000), chainChild(3, 3, 10000)}},
		{children: []parentbilling.HolidayChild{chainChild(4, 1, 10000)}},
	}, holidayRules{sibling: half})
	halves := 0
	for _, registration := range four {
		for _, child := range registration.Children {
			if child.TotalCents == 5000 {
				halves++
			}
		}
	}
	assert.Equal(t, 3, halves, "four children in two registrations: three pay half")
	assert.Equal(t, int64(10000+5000+5000), four[0].TotalCents)
	assert.Equal(t, int64(5000), four[1].TotalCents)
}

// TestExemptionStaysWithItsRegistrationInAGroup pins that the exemption of
// one registration of a group frees only its children (#5628).
func TestExemptionStaysWithItsRegistrationInAGroup(t *testing.T) {
	t.Parallel()
	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	exemption := &parentbilling.ContributionRule{Kind: parentbilling.RuleBenefitsExemption, Note: "Beitragsfrei wegen Leistungsbezug"}
	results := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000)}, exemption: true},
		{children: []parentbilling.HolidayChild{chainChild(2, 1, 10000)}},
	}, holidayRules{sibling: half, exemption: exemption})
	assert.True(t, results[0].BenefitsExemption)
	assert.Equal(t, int64(0), results[0].TotalCents)
	assert.False(t, results[1].BenefitsExemption)
	assert.Equal(t, int64(5000), results[1].TotalCents, "the exempt child still counts as the first child")
}

// TestCostTakeoverInASiblingGroupTakesOverOnlyItsRegistration pins the
// combination of #5628 and #5627: two registrations of one sibling group,
// one of them with a cost takeover. The sibling rule ranks across the
// group first; the cost bearer then takes over what its registration owes
// after the discount, so the family pays nothing for it, and the discount
// of the other registration stays as it is.
func TestCostTakeoverInASiblingGroupTakesOverOnlyItsRegistration(t *testing.T) {
	t.Parallel()
	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	bearer := youthOfficeBearer()
	rules := holidayRules{sibling: half}

	// The later registration has the cost takeover: its child is the second
	// child and the cost bearer pays the half price.
	later := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000)}},
		{children: []parentbilling.HolidayChild{chainChild(2, 1, 10000)}, bearer: bearer},
	}, rules)
	assert.Equal(t, int64(10000), later[0].TotalCents, "the first child pays in full")
	assert.Nil(t, later[0].CostBearerID, "the cost bearer belongs to the other registration")
	assert.Equal(t, int64(0), later[1].TotalCents, "the family pays nothing for the registration with the cost takeover")
	require.NotNil(t, later[1].CostBearerID)
	assert.Equal(t, bearer.ID, *later[1].CostBearerID)
	assert.Equal(t, []string{"base:10000", "sibling:-5000", "cost_takeover:-5000"}, chainLineKinds(later[1].Children[0].Lines),
		"the cost bearer gets the amount after the sibling discount")

	// The earlier registration has the cost takeover: the cost bearer pays
	// the full price of the first child, the discount of the other
	// registration's child stays.
	earlier := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(1, 1, 10000)}, bearer: bearer},
		{children: []parentbilling.HolidayChild{chainChild(2, 1, 10000)}},
	}, rules)
	assert.Equal(t, []string{"base:10000", "cost_takeover:-10000"}, chainLineKinds(earlier[0].Children[0].Lines))
	assert.Equal(t, int64(0), earlier[0].TotalCents)
	assert.Equal(t, []string{"base:10000", "sibling:-5000"}, chainLineKinds(earlier[1].Children[0].Lines),
		"the other registration keeps its discount")
	assert.Equal(t, int64(5000), earlier[1].TotalCents)
	assert.Nil(t, earlier[1].CostBearerID)
}

func chainLineKinds(lines []parentbilling.HolidayContributionLine) []string {
	kinds := make([]string, len(lines))
	for i, line := range lines {
		kinds[i] = fmt.Sprintf("%s:%d", line.Kind, line.AmountCents)
	}
	return kinds
}
