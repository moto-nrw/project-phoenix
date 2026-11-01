package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// changedWeekdays returns the dropped and the net-loss weekdays of a change.
func changedWeekdays(work *offeringAdjustmentWork) [2][]int {
	dropped, netLoss := changedBookingWeekdays(work)
	return [2][]int{dropped, netLoss}
}

func TestChangedBookingWeekdays_NamesDaysAnOfferingNoLongerBooks(t *testing.T) {
	t.Parallel()

	randstunde := &careplan.CareOffering{ID: 7, DaysOfWeekMode: daysOfWeekModeParentChoice}
	work := &offeringAdjustmentWork{
		offeringByID: map[int64]*careplan.CareOffering{7: randstunde},
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 7, SelectedDays: []string{"mon", "fri"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7, SelectedDays: []string{"mon"}}},
	}

	assert.Equal(t, [2][]int{{5}, {5}}, changedWeekdays(work))
}

// The impact only reports: a legacy parent-choice booking without stored days
// must not fail the booking change it describes (#5102).
func TestChangedBookingWeekdays_SkipsBookingsWithoutResolvableDays(t *testing.T) {
	t.Parallel()

	legacy := &careplan.CareOffering{ID: 7, DaysOfWeekMode: daysOfWeekModeParentChoice}
	work := &offeringAdjustmentWork{
		offeringByID: map[int64]*careplan.CareOffering{7: legacy},
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 7}},
	}

	assert.Equal(t, [2][]int{{}, {}}, changedWeekdays(work))
}

// Ganztag variants (6 "bis 14:30", 7 "bis 16 Uhr") as fixed offerings on the
// same days, and a parent-choice Randstunde (5).
func switchOfferings() map[int64]*careplan.CareOffering {
	week := []string{"mon", "tue", "wed", "thu", "fri"}
	return map[int64]*careplan.CareOffering{
		5: {ID: 5, DaysOfWeekMode: daysOfWeekModeParentChoice},
		6: {ID: 6, DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: week},
		7: {ID: 7, DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: week},
	}
}

// A switch between Ganztag variants drops the old variant on every day it
// booked; the new variant offsets it, so no day is a net loss. With pickup
// times, Timetable decides about the switched days (#5342); without them,
// the switch keeps reporting nothing.
func TestChangedBookingWeekdays_SwitchBetweenOfferingsIsNoNetLoss(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
	}

	assert.Equal(t, [2][]int{{1, 2, 3, 4, 5}, {}}, changedWeekdays(work))
}

// Dropping the Randstunde on Monday is a net loss even though Ganztag still
// books Monday.
func TestChangedBookingWeekdays_DroppedRandstundeIsANetLossWhileGanztagStays(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon", "thu"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"thu"}}},
	}

	assert.Equal(t, [2][]int{{1}, {1}}, changedWeekdays(work))
}

// A switch does not hide a Randstunde dropped on the same day.
func TestChangedBookingWeekdays_SwitchDoesNotHideDroppedRandstunde(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7}},
	}

	assert.Equal(t, [2][]int{{1, 2, 3, 4, 5}, {1}}, changedWeekdays(work))
}

func TestChangedBookingWeekdays_NewlyBookedRandstundeLosesNothing(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon", "wed"}}},
	}

	assert.Equal(t, [2][]int{{}, {}}, changedWeekdays(work))
}

// The remaining bookings carry what decides about a hand-maintained
// Regeltermin (#5342): the weekday, the offering's kind flags and its pickup
// time that day. A day without a pickup time stays empty, a link
// that ends before the date and one without resolvable days name nothing.
func TestRemainingDayBookings_NameWeekdayCareAndPickupTime(t *testing.T) {
	t.Parallel()

	from := calendar.NewDate(2026, 10, 26)
	ended := calendar.NewDate(2026, 10, 26)
	offerings := map[int64]*careplan.CareOffering{
		5: {ID: 5, DaysOfWeekMode: daysOfWeekModeParentChoice, CountsAsCare: true, PickupTimes: map[string]string{"mon": "12:45", "fri": "12:45"}},
		6: {ID: 6, DaysOfWeekMode: daysOfWeekModeParentChoice, CountsAsCare: true, IncludesLunch: true},
		9: {ID: 9, DaysOfWeekMode: daysOfWeekModeParentChoice, PickupTimes: map[string]string{"tue": "15:00"}},
	}
	links := []*careplan.BookedOffering{
		{CareOfferingID: 5, SelectedDays: []string{"mon", "fri"}},
		{CareOfferingID: 6, SelectedDays: []string{"fri"}},
		{CareOfferingID: 9, SelectedDays: []string{"tue"}},
		{CareOfferingID: 6, SelectedDays: []string{"wed"}, ValidUntil: &ended},
		{CareOfferingID: 5},
		{CareOfferingID: 42, SelectedDays: []string{"thu"}},
	}

	assert.Equal(t, []careplan.RemainingDayBooking{
		{Weekday: 1, CountsAsCare: true, PickupTime: "12:45"},
		{Weekday: 5, CountsAsCare: true, PickupTime: "12:45"},
		{Weekday: 5, CountsAsCare: true, IncludesLunch: true},
		{Weekday: 2, PickupTime: "15:00"},
	}, remainingDayBookings(links, offerings, from))
}

// TestLinkStart_NamesTheDayALaterSelectionStarts (#5497): a selection of
// another enrollment that starts after the change's date counts from its own
// start, a phase that starts later from the phase's start; a selection that
// ends before its phase serves, or a child that is not approved, plans
// nothing.
func TestLinkStart_NamesTheDayALaterSelectionStarts(t *testing.T) {
	t.Parallel()
	from := calendar.NewDate(2026, 11, 2)
	service := &careplan.OfferingPhase{ServiceStart: calendar.NewDate(2026, 8, 17), ServiceEnd: calendar.NewDate(2027, 7, 1)}
	date := func(d calendar.Date) *calendar.Date { return &d }

	start, ok := linkStart(&careplan.BookedOffering{ValidFrom: date(calendar.NewDate(2026, 8, 17))}, service, from)
	assert.True(t, ok)
	assert.Equal(t, from, start, "in force on the date")

	start, ok = linkStart(&careplan.BookedOffering{ValidFrom: date(calendar.NewDate(2026, 11, 16))}, service, from)
	assert.True(t, ok)
	assert.Equal(t, calendar.NewDate(2026, 11, 16), start, "the rebooking starts later")

	later := &careplan.OfferingPhase{ServiceStart: calendar.NewDate(2027, 2, 1), ServiceEnd: calendar.NewDate(2027, 7, 1)}
	start, ok = linkStart(&careplan.BookedOffering{}, later, from)
	assert.True(t, ok)
	assert.Equal(t, calendar.NewDate(2027, 2, 1), start, "the phase serves from its start")

	_, ok = linkStart(&careplan.BookedOffering{ValidUntil: date(calendar.NewDate(2027, 1, 1))}, later, from)
	assert.False(t, ok, "the selection ends before its phase serves")
	_, ok = linkStart(&careplan.BookedOffering{}, nil, from)
	assert.False(t, ok, "a request child that is not approved books nothing")
}
