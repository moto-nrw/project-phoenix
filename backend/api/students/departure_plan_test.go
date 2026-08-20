package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestApplyDeparturePlan_NoInputPreservesAllowedModes(t *testing.T) {
	t.Parallel()

	student := &users.Student{
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureAlone,
				users.DepartureBus,
			},
		},
		BusDays: users.BusDays{users.PickupDayMonday: true},
	}

	applyDeparturePlan(nil, nil, nil, nil, nil, nil, student)

	assert.Equal(t, users.AllowedDepartureModes{
		users.PickupDayMonday: []users.DepartureMode{
			users.DepartureAlone,
			users.DepartureBus,
		},
	}, student.AllowedDepartureModes)
}

func TestApplyDeparturePlan_LegacyDepartureDoesNotCollapseAllowedModes(t *testing.T) {
	t.Parallel()

	student := &users.Student{
		DepartureDays: users.DepartureDays{
			users.PickupDayMonday: users.DeparturePickup,
		},
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureBus,
				users.DeparturePickup,
			},
		},
		BusDays:    users.BusDays{users.PickupDayMonday: true},
		PickupDays: users.PickupDays{users.PickupDayMonday: true},
	}
	departure := users.DepartureDays{
		users.PickupDayMonday: users.DeparturePickup,
	}

	applyDeparturePlan(nil, &departure, nil, nil, nil, nil, student)

	assert.Equal(t, users.AllowedDepartureModes{
		users.PickupDayMonday: []users.DepartureMode{
			users.DepartureBus,
			users.DeparturePickup,
		},
	}, student.AllowedDepartureModes)
	assert.True(t, student.BusDays[users.PickupDayMonday])
	assert.True(t, student.PickupDays[users.PickupDayMonday])
}

func TestApplyDeparturePlan_LegacyMapsDoNotReplaceAllowedModes(t *testing.T) {
	t.Parallel()

	student := &users.Student{
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureAlone,
				users.DepartureBus,
			},
			users.PickupDayTuesday: []users.DepartureMode{
				users.DeparturePickup,
			},
		},
		BusDays: users.BusDays{users.PickupDayMonday: true},
	}
	busDays := users.BusDays{users.PickupDayTuesday: true}

	applyDeparturePlan(nil, nil, nil, nil, nil, &busDays, student)

	assert.Equal(t, users.BusDays{users.PickupDayTuesday: true}, student.BusDays)
	assert.Equal(t, users.AllowedDepartureModes{
		users.PickupDayMonday: []users.DepartureMode{
			users.DepartureAlone,
			users.DepartureBus,
		},
		users.PickupDayTuesday: []users.DepartureMode{
			users.DeparturePickup,
		},
	}, student.AllowedDepartureModes)
}
