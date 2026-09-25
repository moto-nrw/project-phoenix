package students

import (
	"testing"

	dep "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/stretchr/testify/assert"
)

func TestApplyDeparturePlan_NoInputPreservesAllowedModes(t *testing.T) {
	t.Parallel()

	student := &Student{
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: []dep.DepartureMode{
				dep.DepartureAlone,
				dep.DepartureBus,
			},
		},
		BusDays: dep.BusDays{dep.PickupDayMonday: true},
	}

	applyDeparturePlan(nil, nil, nil, nil, nil, nil, student)

	assert.Equal(t, dep.AllowedDepartureModes{
		dep.PickupDayMonday: []dep.DepartureMode{
			dep.DepartureAlone,
			dep.DepartureBus,
		},
	}, student.AllowedDepartureModes)
}

func TestApplyDeparturePlan_LegacyDepartureDoesNotCollapseAllowedModes(t *testing.T) {
	t.Parallel()

	student := &Student{
		DepartureDays: dep.DepartureDays{
			dep.PickupDayMonday: dep.DeparturePickup,
		},
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: []dep.DepartureMode{
				dep.DepartureBus,
				dep.DeparturePickup,
			},
		},
		BusDays:    dep.BusDays{dep.PickupDayMonday: true},
		PickupDays: dep.PickupDays{dep.PickupDayMonday: true},
	}
	departure := dep.DepartureDays{
		dep.PickupDayMonday: dep.DeparturePickup,
	}

	applyDeparturePlan(nil, &departure, nil, nil, nil, nil, student)

	assert.Equal(t, dep.AllowedDepartureModes{
		dep.PickupDayMonday: []dep.DepartureMode{
			dep.DepartureBus,
			dep.DeparturePickup,
		},
	}, student.AllowedDepartureModes)
	assert.True(t, student.BusDays[dep.PickupDayMonday])
	assert.True(t, student.PickupDays[dep.PickupDayMonday])
}

func TestApplyDeparturePlan_LegacyMapsDoNotReplaceAllowedModes(t *testing.T) {
	t.Parallel()

	student := &Student{
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: []dep.DepartureMode{
				dep.DepartureAlone,
				dep.DepartureBus,
			},
			dep.PickupDayTuesday: []dep.DepartureMode{
				dep.DeparturePickup,
			},
		},
		BusDays: dep.BusDays{dep.PickupDayMonday: true},
	}
	busDays := dep.BusDays{dep.PickupDayTuesday: true}

	applyDeparturePlan(nil, nil, nil, nil, nil, &busDays, student)

	assert.Equal(t, dep.BusDays{dep.PickupDayTuesday: true}, student.BusDays)
	assert.Equal(t, dep.AllowedDepartureModes{
		dep.PickupDayMonday: []dep.DepartureMode{
			dep.DepartureAlone,
			dep.DepartureBus,
		},
		dep.PickupDayTuesday: []dep.DepartureMode{
			dep.DeparturePickup,
		},
	}, student.AllowedDepartureModes)
}
