package application

import (
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
	result := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4501), chainChild(2, 2, 6750)}, holidayRules{sibling: rule}, false)

	require.Len(t, result.Children, 2)
	cheaper, dearer := result.Children[0], result.Children[1]
	assert.Equal(t, int64(6750), dearer.TotalCents)
	require.Len(t, cheaper.Lines, 2)
	assert.Equal(t, parentbilling.LineSibling, cheaper.Lines[1].Kind)
	assert.Equal(t, int64(-1500), cheaper.Lines[1].AmountCents, "4501 × 33.33 % = 1500.18 rounds to 1500 cent")
	assert.Equal(t, 2, *cheaper.Lines[1].ChildRank)
	assert.Equal(t, int64(3001+6750), result.TotalCents)

	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	odd := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4500), chainChild(2, 2, 4501)}, holidayRules{sibling: half}, false)
	assert.Equal(t, int64(-2250), odd.Children[0].Lines[1].AmountCents, "equal ranks keep the form order; 4500 / 2 = 2250")
	assert.Len(t, odd.Children[1].Lines, 1, "the dearer child is the first child")
	halfUp := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 4501), chainChild(2, 2, 4501)}, holidayRules{sibling: half}, false)
	assert.Equal(t, int64(-2251), halfUp.Children[1].Lines[1].AmountCents, "2250.5 rounds half up to 2251 cent")
}

func TestFixedSiblingDiscountNeverExceedsTheBasePriceAndCountsOnlyBookedChildren(t *testing.T) {
	t.Parallel()
	rule := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(3), DiscountCents: new(int64(5000))}
	children := []parentbilling.HolidayChild{chainChild(1, 1, 6750), chainChild(2, 2), chainChild(3, 3, 4500), chainChild(4, 4, 4500)}
	result := computeHolidayContribution(children, holidayRules{sibling: rule}, false)

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

	freed := computeHolidayContribution(children, holidayRules{exemption: exemption}, true)
	assert.True(t, freed.ExemptionAvailable)
	assert.True(t, freed.BenefitsExemption)
	assert.Equal(t, int64(0), freed.TotalCents)
	last := freed.Children[0].Lines[len(freed.Children[0].Lines)-1]
	assert.Equal(t, parentbilling.LineBenefitsExemption, last.Kind)
	assert.Equal(t, int64(-9000), last.AmountCents)
	assert.Equal(t, "Beitragsfrei wegen Leistungsbezug", last.Note)

	without := computeHolidayContribution(children, holidayRules{}, true)
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
