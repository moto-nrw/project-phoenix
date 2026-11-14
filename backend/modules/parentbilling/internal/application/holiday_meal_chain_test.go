package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The lunch of the holiday care stands outside the rule chain (#5631, ADR
// 0115 addendum). The routes prove it end to end in
// api/holiday_lunch_api_test.go; the edges of the chain, the frozen lines
// and the accounts are pinned here.

// withLunch gives every booking of the child lunches at 4,30 € each.
func withLunch(child parentbilling.HolidayChild, lunches int) parentbilling.HolidayChild {
	for i := range child.Bookings {
		child.Bookings[i].Lunch = true
		child.Bookings[i].LunchDays, child.Bookings[i].LunchCents = lunches, int64(lunches)*430
	}
	return child
}

func mealLines(child parentbilling.HolidayChildContribution) []parentbilling.HolidayContributionLine {
	var meals []parentbilling.HolidayContributionLine
	for _, line := range child.Lines {
		if line.Kind == parentbilling.LineMeal {
			meals = append(meals, line)
		}
	}
	return meals
}

func TestLunchIsAPositionOfItsOwnAfterTheChain(t *testing.T) {
	t.Parallel()
	booked := withLunch(chainChild(1, 1, 6500, 6500), 5)
	result := computeHolidayContribution([]parentbilling.HolidayChild{booked}, holidayRules{}, false, nil)

	child := result.Children[0]
	require.Len(t, child.Lines, 4)
	assert.Equal(t, []string{parentbilling.LineBase, parentbilling.LineBase, parentbilling.LineMeal, parentbilling.LineMeal},
		[]string{child.Lines[0].Kind, child.Lines[1].Kind, child.Lines[2].Kind, child.Lines[3].Kind})
	meal := child.Lines[2]
	assert.Equal(t, int64(2150), meal.AmountCents, "5 lunches at 4,30 € are 21,50 €")
	assert.Equal(t, 5, *meal.BookedDays)
	assert.Equal(t, calendar.NewDate(2027, 7, 5), *meal.WeekStartsOn)
	assert.Equal(t, booked.Bookings[0].WeekID, meal.WeekID)
	assert.Empty(t, meal.TimeModelName)
	assert.Equal(t, int64(2*6500+2*2150), result.TotalCents)
}

func TestNoRuleOfTheChainReachesTheLunch(t *testing.T) {
	t.Parallel()
	sibling := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	exemption := &parentbilling.ContributionRule{Kind: parentbilling.RuleBenefitsExemption, Note: "Leistungsbezug"}
	children := func() []parentbilling.HolidayChild {
		return []parentbilling.HolidayChild{withLunch(chainChild(1, 1, 6500), 5), withLunch(chainChild(2, 2, 6500), 5)}
	}

	discounted := computeHolidayContribution(children(), holidayRules{sibling: sibling}, false, nil)
	second := discounted.Children[1]
	require.Len(t, second.Lines, 3)
	assert.Equal(t, int64(-3250), second.Lines[1].AmountCents, "the sibling discount is half the care, not half the care and the lunch")
	assert.Equal(t, parentbilling.LineMeal, second.Lines[2].Kind)
	assert.Equal(t, int64(3250+2150), second.TotalCents)
	assert.Equal(t, int64(6500+2150+3250+2150), discounted.TotalCents)

	exempt := computeHolidayContribution(children(), holidayRules{exemption: exemption}, true, nil)
	for _, child := range exempt.Children {
		assert.Equal(t, int64(2150), child.TotalCents, "the exemption frees the care; the family pays the lunch")
		assert.Equal(t, int64(-6500), child.Lines[1].AmountCents)
	}
	assert.Equal(t, int64(4300), exempt.TotalCents)

	bearer := &parentbilling.CostBearer{ID: 7, Name: "Landkreis"}
	taken := computeHolidayContribution(children(), holidayRules{sibling: sibling}, false, bearer)
	assert.Equal(t, int64(4300), taken.TotalCents, "the cost bearer takes the care over; the family pays the lunch")
	assert.Equal(t, int64(-(6500 + 3250)), billedOf(taken.Children, taken.TotalCents).CostTakeoverCents)
}

func TestLunchDoesNotChangeWhoIsTheFirstChild(t *testing.T) {
	t.Parallel()
	sibling := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	// The cheaper child has lunch: with it its total is higher, but the
	// rank follows the care alone.
	result := computeHolidayContribution([]parentbilling.HolidayChild{withLunch(chainChild(1, 1, 5000), 5), chainChild(2, 2, 6500)}, holidayRules{sibling: sibling}, false, nil)

	assert.Equal(t, int64(6500), result.Children[1].TotalCents, "the child with the dearer care stays the first child")
	assert.Equal(t, int64(2500+2150), result.Children[0].TotalCents)
}

func TestAWaitingBookingHasNoLunchYet(t *testing.T) {
	t.Parallel()
	child := withLunch(chainChild(1, 1, 6500, 6500), 5)
	child.Bookings[1].Waitlisted = true
	result := computeHolidayContribution([]parentbilling.HolidayChild{child}, holidayRules{}, false, nil)

	require.Len(t, mealLines(result.Children[0]), 1)
	assert.Equal(t, int64(6500+2150), result.TotalCents)
}

func TestFrozenLinesKeepTheirLunchWhenTheGroupIsComputedAgain(t *testing.T) {
	t.Parallel()
	first := computeHolidayContribution([]parentbilling.HolidayChild{withLunch(chainChild(1, 1, 6500, 6500), 4)}, holidayRules{}, false, nil)

	// The registration now names other prices for the care and the lunch:
	// the frozen lines decide, as for the care.
	current := withLunch(chainChild(1, 1, 9900, 9900), 5).Bookings
	for i := range current {
		current[i].LunchCents = 2500
	}
	frozen := frozenHolidayBookings(first.Children[0].Lines, current)
	require.Len(t, frozen, 2)
	for _, booking := range frozen {
		assert.Equal(t, 4, booking.LunchDays)
		assert.Equal(t, int64(1720), booking.LunchCents)
		assert.Equal(t, int64(6500), booking.PriceCents)
	}
	again := computeHolidayContribution([]parentbilling.HolidayChild{{ID: 1, Position: 1, Bookings: frozen}}, holidayRules{}, false, nil)
	assert.True(t, sameContribution(first, again), "computing a frozen contribution again changes nothing")
}

// Booked means billed (#5631): the Träger-Büro takes the lunch off a
// confirmed week or adds it, and the contribution follows. A lunch it adds
// costs the price of today; the care keeps its frozen price.
func TestARebookedLunchChangesTheFrozenContribution(t *testing.T) {
	t.Parallel()
	first := computeHolidayContribution([]parentbilling.HolidayChild{withLunch(chainChild(1, 1, 6500, 6500), 4)}, holidayRules{}, false, nil)

	// The first week loses its lunch, the second keeps it.
	current := withLunch(chainChild(1, 1, 9900, 9900), 4).Bookings
	current[0].Lunch = false
	current[0].LunchDays, current[0].LunchCents = 0, 0
	frozen := frozenHolidayBookings(first.Children[0].Lines, current)
	require.Len(t, frozen, 2)
	assert.Zero(t, frozen[0].LunchDays)
	assert.Zero(t, frozen[0].LunchCents)
	assert.Equal(t, int64(1720), frozen[1].LunchCents)
	assert.Equal(t, int64(6500), frozen[0].PriceCents)

	// A week confirmed without lunch gains one at the price of today.
	without := computeHolidayContribution([]parentbilling.HolidayChild{chainChild(1, 1, 6500, 6500)}, holidayRules{}, false, nil)
	current = chainChild(1, 1, 9900, 9900).Bookings
	current[1].Lunch = true
	current[1].LunchDays, current[1].LunchCents = 5, 2500
	frozen = frozenHolidayBookings(without.Children[0].Lines, current)
	require.Len(t, frozen, 2)
	assert.Zero(t, frozen[0].LunchDays)
	assert.Equal(t, 5, frozen[1].LunchDays)
	assert.Equal(t, int64(2500), frozen[1].LunchCents)

	// A week the registration no longer books keeps what it froze.
	frozen = frozenHolidayBookings(first.Children[0].Lines, nil)
	require.Len(t, frozen, 2)
	assert.Equal(t, int64(1720), frozen[0].LunchCents)
}

func TestQuoteCountsOneLunchPerCareDay(t *testing.T) {
	t.Parallel()
	price := int64(430)
	monday := calendar.NewDate(2027, 7, 5)
	whole := parentbilling.HolidayWeek{ID: 1, StartsOn: monday, EndsOn: monday.AddDays(4), LunchDayCents: &price, WeekLunchDays: 5,
		TimeModels: []parentbilling.HolidayTimeModel{{ID: 10, Name: "bis 16 Uhr", PriceCents: 6500}}}
	byDays := parentbilling.HolidayWeek{ID: 2, StartsOn: monday.AddDays(7), EndsOn: monday.AddDays(11), LunchDayCents: &price, WeekLunchDays: 5,
		CareDays:   []calendar.Date{monday.AddDays(7), monday.AddDays(8), monday.AddDays(9), monday.AddDays(10), monday.AddDays(11)},
		TimeModels: []parentbilling.HolidayTimeModel{{ID: 20, Name: "bis 16 Uhr", PriceCents: 1300}}}
	without := parentbilling.HolidayWeek{ID: 3, StartsOn: monday.AddDays(14), EndsOn: monday.AddDays(18),
		TimeModels: []parentbilling.HolidayTimeModel{{ID: 30, Name: "bis 16 Uhr", PriceCents: 6500}}}
	weeks := []parentbilling.HolidayWeek{whole, byDays, without}

	priced, err := quotedChildren(weeks, []parentbilling.HolidayQuoteChild{{Bookings: []parentbilling.HolidayQuoteBooking{
		{WeekID: 1, TimeModelID: 10, Lunch: true},
		{WeekID: 2, TimeModelID: 20, Days: []calendar.Date{monday.AddDays(7), monday.AddDays(9)}, Lunch: true},
		{WeekID: 3, TimeModelID: 30},
	}}})
	require.NoError(t, err)
	bookings := priced[0].Bookings
	assert.Equal(t, []int{5, 2, 0}, []int{bookings[0].LunchDays, bookings[1].LunchDays, bookings[2].LunchDays})
	assert.Equal(t, []int64{2150, 860, 0}, []int64{bookings[0].LunchCents, bookings[1].LunchCents, bookings[2].LunchCents})

	_, err = quotedChildren(weeks, []parentbilling.HolidayQuoteChild{{Bookings: []parentbilling.HolidayQuoteBooking{{WeekID: 3, TimeModelID: 30, Lunch: true}}}})
	require.ErrorIs(t, err, parentbilling.ErrInvalidHolidayQuote, "a week without lunch takes no booking with lunch")
}

func TestLunchBooksOnItsOwnRevenueAccountAndKeepsItsAmountInTheExport(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 7, 5)
	friday := monday.AddDays(4)
	lunches := 5
	child := parentbilling.BillingChild{ChildID: 1, Positions: []parentbilling.BillingPosition{
		{Kind: parentbilling.LineBase, AmountCents: 6500, WeekStartsOn: &monday, WeekEndsOn: &friday, TimeModelName: "bis 16 Uhr", WeekID: 1},
		{Kind: parentbilling.LineSibling, AmountCents: -3250},
		{Kind: parentbilling.LineMeal, AmountCents: 2150, WeekStartsOn: &monday, WeekEndsOn: &friday, WeekID: 1, BookedDays: &lunches},
	}}
	assignments := parentbilling.BookingAssignments{
		RevenueAccounts: []parentbilling.RevenueAccount{
			{Scope: parentbilling.BookingScopeHoliday, Account: "4400"},
			{Scope: parentbilling.BookingScopeHolidayMeals, Account: "4410"},
		},
		CostCenters: []parentbilling.CostCenter{{Scope: parentbilling.BookingScopeSchool, ScopeID: 9, ValidFrom: monday.AddDays(-30), Code: "GS-9"}},
	}
	resolver := newBookingResolver(assignments, "4000")
	hosts := newHolidayHosts([]parentbilling.HolidayWeekHost{{WeekID: 1, StartsOn: monday, EndsOn: friday, HostSchoolID: 9}})
	freezeHolidayChild(resolver, hosts, monday, &child)

	care, discount, meal := child.Positions[0], child.Positions[1], child.Positions[2]
	assert.Equal(t, "4400", care.Accounts.RevenueAccount)
	assert.Equal(t, "4400", discount.Accounts.RevenueAccount, "a line without a week follows the care of the first week")
	assert.Equal(t, "4410", meal.Accounts.RevenueAccount)
	assert.Equal(t, "GS-9", meal.Accounts.CostCenter, "the lunch books on the KOST1 of the school that hosts its week")

	withoutOwn := newBookingResolver(parentbilling.BookingAssignments{RevenueAccounts: assignments.RevenueAccounts[:1]}, "4000")
	assert.Equal(t, "4400", withoutOwn.holidayMealAccount(), "without an account of its own the lunch books like the holiday care")
	assert.Equal(t, "4000", newBookingResolver(parentbilling.BookingAssignments{}, "4000").holidayMealAccount())

	invoice := parentbilling.ExportInvoice{TotalCents: 5400, Parts: []parentbilling.ExportBookingPart{
		{Kind: parentbilling.BookingPartHoliday, ChildID: 1, AmountCents: 6500, Accounts: care.Accounts},
		{Kind: parentbilling.BookingPartHoliday, ChildID: 1, AmountCents: -3250, Accounts: discount.Accounts},
		{Kind: parentbilling.BookingPartHolidayMeal, ChildID: 1, AmountCents: 2150, Accounts: meal.Accounts},
	}}
	lines, _ := resolver.documentLines(invoice)
	require.Len(t, lines, 2)
	assert.Equal(t, bookingLine{amountCents: 3250, accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "GS-9"}}, lines[0])
	assert.Equal(t, bookingLine{amountCents: 2150, accounts: parentbilling.PositionAccounts{RevenueAccount: "4410", CostCenter: "GS-9"}}, lines[1],
		"the sibling discount is not netted with the lunch")
}

func TestMealPositionNamesItsLunchesOnTheInvoice(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 7, 5)
	friday := monday.AddDays(4)
	five, one := 5, 1
	position := parentbilling.BillingPosition{Kind: parentbilling.LineMeal, AmountCents: 2150, WeekStartsOn: &monday, WeekEndsOn: &friday, BookedDays: &five}

	line := invoiceLine(position)
	assert.Equal(t, "Mittagessen, 5 Essen zu je 4,30 €", line.TimeModel)
	assert.Equal(t, "Woche 05.07. bis 09.07.2027", line.Week)
	assert.Equal(t, "Woche 05.07.2027 bis 09.07.2027, Mittagessen, 5 Essen zu je 4,30 €", overviewPositionLabel(position))
	assert.Equal(t, "Mittagessen, 1 Essen zu 4,30 €", mealLabel(parentbilling.BillingPosition{Kind: parentbilling.LineMeal, AmountCents: 430, BookedDays: &one}))
	// A Storno negates the amount; the price of one lunch stays positive.
	assert.Equal(t, "Mittagessen, 5 Essen zu je 4,30 €", mealLabel(parentbilling.BillingPosition{Kind: parentbilling.LineMeal, AmountCents: -2150, BookedDays: &five}))
	assert.Equal(t, &five, billingPosition(parentbilling.HolidayContributionLine{Kind: parentbilling.LineMeal, BookedDays: &five}).BookedDays)
	assert.Nil(t, billingPosition(parentbilling.HolidayContributionLine{Kind: parentbilling.LineBase, BookedDays: &five}).BookedDays,
		"only a meal position names a number on the invoice")
}

func TestFrozenLunchSurvivesWhenAllBookedDaysClose(t *testing.T) {
	t.Parallel()
	first := computeHolidayContribution([]parentbilling.HolidayChild{withLunch(chainChild(1, 1, 6500), 1)}, holidayRules{}, false, nil)
	current := withLunch(chainChild(1, 1, 9900), 0).Bookings

	frozen := frozenHolidayBookings(first.Children[0].Lines, current)
	again := computeHolidayContribution([]parentbilling.HolidayChild{{ID: 1, Position: 1, Bookings: frozen}}, holidayRules{}, false, nil)
	assert.True(t, sameContribution(first, again), "a later closing day does not deselect lunch or change its frozen amount")
}
