package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

func TestLostBookingWeekdays_NamesDaysAnOfferingNoLongerBooks(t *testing.T) {
	t.Parallel()

	randstunde := &careplan.CareOffering{ID: 7, DaysOfWeekMode: daysOfWeekModeParentChoice}
	work := &offeringAdjustmentWork{
		offeringByID: map[int64]*careplan.CareOffering{7: randstunde},
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 7, SelectedDays: []string{"mon", "fri"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7, SelectedDays: []string{"mon"}}},
	}

	assert.Equal(t, []int{5}, lostBookingWeekdays(work))
}

// The impact only reports: a legacy parent-choice booking without stored days
// must not fail the booking change it describes (#5102).
func TestLostBookingWeekdays_SkipsBookingsWithoutResolvableDays(t *testing.T) {
	t.Parallel()

	legacy := &careplan.CareOffering{ID: 7, DaysOfWeekMode: daysOfWeekModeParentChoice}
	work := &offeringAdjustmentWork{
		offeringByID: map[int64]*careplan.CareOffering{7: legacy},
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 7}},
	}

	assert.Empty(t, lostBookingWeekdays(work))
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

// A switch between Ganztag variants on the same days keeps the child in care:
// no hand-maintained Regeltermin may be reported as lost.
func TestLostBookingWeekdays_SwitchBetweenOfferingsOnTheSameDaysLosesNothing(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
	}

	assert.Empty(t, lostBookingWeekdays(work))
}

// Dropping the Randstunde on Monday stays a loss even though Ganztag still
// books Monday: the Randstunden block must still be reported.
func TestLostBookingWeekdays_DroppedRandstundeIsLostWhileGanztagStays(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon", "thu"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"thu"}}},
	}

	assert.Equal(t, []int{1}, lostBookingWeekdays(work))
}

// A switch does not hide a Randstunde dropped on the same day.
func TestLostBookingWeekdays_SwitchDoesNotHideDroppedRandstunde(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon"}}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 7}},
	}

	assert.Equal(t, []int{1}, lostBookingWeekdays(work))
}

func TestLostBookingWeekdays_NewlyBookedRandstundeLosesNothing(t *testing.T) {
	t.Parallel()

	work := &offeringAdjustmentWork{
		offeringByID: switchOfferings(),
		beforeLinks:  []*careplan.BookedOffering{{CareOfferingID: 6}},
		replacement:  []*careplan.BookedOffering{{CareOfferingID: 6}, {CareOfferingID: 5, SelectedDays: []string{"mon", "wed"}}},
	}

	assert.Empty(t, lostBookingWeekdays(work))
}
