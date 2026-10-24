package departure_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"
)

func TestHasCompanionNote(t *testing.T) {
	t.Parallel()
	blank, note := "  ", "Mit Lina"
	assert.False(t, departure.HasCompanionNote(nil))
	assert.False(t, departure.HasCompanionNote(&blank))
	assert.True(t, departure.HasCompanionNote(&note))
}

func TestMissingAccompaniedDaysKeepsWeekdayOrder(t *testing.T) {
	t.Parallel()
	plan := departure.Plan{
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday: {departure.DepartureAccompanied},
			departure.PickupDayFriday: {departure.DepartureBus},
		},
		DepartureDays: departure.DepartureDays{departure.PickupDayWednesday: departure.DepartureAccompanied},
	}

	missing := plan.MissingAccompaniedDays([]string{"fri", "sat", "mon", "tue", "wed"})

	assert.Equal(t, []string{departure.PickupDayTuesday, departure.PickupDayFriday}, missing)
	assert.Nil(t, plan.MissingAccompaniedDays([]string{departure.PickupDayMonday, departure.PickupDayWednesday}))
}
