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
