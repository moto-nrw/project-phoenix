package users

import (
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestResolveAllowedDepartureModes(t *testing.T) {
	t.Parallel()

	t.Run("hydrated unified change wins when legacy mirrors are unchanged", func(t *testing.T) {
		current := &studentDepartureState{
			BusDays:       userModels.BusDays{userModels.PickupDayMonday: true},
			PickupDays:    userModels.PickupDays{},
			DepartureDays: userModels.DepartureDays{userModels.PickupDayMonday: userModels.DepartureBus},
			AllowedDepartureModes: userModels.AllowedDepartureModes{
				userModels.PickupDayMonday: []userModels.DepartureMode{userModels.DepartureBus},
			},
		}
		student := &userModels.Student{
			BusDays:       userModels.BusDays{userModels.PickupDayMonday: true},
			PickupDays:    userModels.PickupDays{},
			DepartureDays: userModels.DepartureDays{userModels.PickupDayThursday: userModels.DeparturePickup},
			AllowedDepartureModes: userModels.AllowedDepartureModes{
				userModels.PickupDayMonday: []userModels.DepartureMode{userModels.DepartureBus},
			},
		}

		got := resolveAllowedDepartureModes(student, current).DepartureDays()

		assert.Equal(t, userModels.DepartureAlone, got.ModeFor(userModels.PickupDayMonday))
		assert.Equal(t, userModels.DeparturePickup, got.ModeFor(userModels.PickupDayThursday))
	})

	t.Run("legacy mirror change still folds legacy maps", func(t *testing.T) {
		current := &studentDepartureState{
			BusDays:       userModels.BusDays{userModels.PickupDayMonday: true},
			PickupDays:    userModels.PickupDays{},
			DepartureDays: userModels.DepartureDays{userModels.PickupDayMonday: userModels.DepartureBus},
			AllowedDepartureModes: userModels.AllowedDepartureModes{
				userModels.PickupDayMonday: []userModels.DepartureMode{userModels.DepartureBus},
			},
		}
		student := &userModels.Student{
			BusDays:       userModels.BusDays{userModels.PickupDayTuesday: true},
			PickupDays:    userModels.PickupDays{},
			DepartureDays: userModels.DepartureDays{userModels.PickupDayMonday: userModels.DepartureBus},
			AllowedDepartureModes: userModels.AllowedDepartureModes{
				userModels.PickupDayMonday: []userModels.DepartureMode{userModels.DepartureBus},
			},
		}

		got := resolveAllowedDepartureModes(student, current).DepartureDays()

		assert.Equal(t, userModels.DepartureAlone, got.ModeFor(userModels.PickupDayMonday))
		assert.Equal(t, userModels.DepartureBus, got.ModeFor(userModels.PickupDayTuesday))
	})
}
