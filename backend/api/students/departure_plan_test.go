package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestApplyDeparturePlan_NoInputPreservesAllowedModes(t *testing.T) {
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
