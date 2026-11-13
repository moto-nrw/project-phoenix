package careplanhttp_test

import (
	"testing"

	peopleState "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	dep "github.com/moto-ogs/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func TestApplyDeparturePlan_NoInputPreservesAllowedModes(t *testing.T) {
	t.Parallel()

	student := &peopleState.StudentState{
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: []dep.DepartureMode{
				dep.DepartureAlone,
				dep.DepartureBus,
			},
		},
		BusDays: dep.BusDays{dep.PickupDayMonday: true},
	}

	careplanhttp.ApplyDeparturePlanForTest(nil, nil, nil, nil, nil, nil, student)

	assert.Equal(t, dep.AllowedDepartureModes{
		dep.PickupDayMonday: []dep.DepartureMode{
			dep.DepartureAlone,
			dep.DepartureBus,
		},
	}, student.AllowedDepartureModes)
}

func TestApplyDeparturePlan_LegacyDepartureDoesNotCollapseAllowedModes(t *testing.T) {
	t.Parallel()

	student := &peopleState.StudentState{
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

	careplanhttp.ApplyDeparturePlanForTest(nil, &departure, nil, nil, nil, nil, student)

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

	student := &peopleState.StudentState{
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

	careplanhttp.ApplyDeparturePlanForTest(nil, nil, nil, nil, nil, &busDays, student)

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
