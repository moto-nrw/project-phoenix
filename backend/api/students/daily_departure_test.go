package students

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestDailyDepartureModes(t *testing.T) {
	student := StudentResponse{
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: {users.DeparturePickup},
			users.PickupDayFriday: {users.DepartureAlone, users.DepartureBus},
		},
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}

	assert.Equal(t,
		[]users.DepartureMode{users.DeparturePickup},
		dailyDepartureModes(student, timezone.NewDate(2026, time.June, 1)),
	)
	assert.Equal(t,
		[]users.DepartureMode{users.DepartureAlone, users.DepartureBus},
		dailyDepartureModes(student, timezone.NewDate(2026, time.June, 5)),
	)
	assert.Equal(t,
		[]users.DepartureMode{users.DepartureAlone},
		dailyDepartureModes(student, timezone.NewDate(2026, time.June, 2)),
	)
}

func TestDailyDepartureModesRedactionAndMissingRule(t *testing.T) {
	monday := timezone.NewDate(2026, time.June, 1)
	configured := StudentResponse{
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: {users.DeparturePickup},
		},
		DepartureRuleConfigured: true,
		HasFullAccess:           false,
	}

	assert.Nil(t, dailyDepartureModes(configured, monday))
	assert.Nil(t, dailyDepartureModes(StudentResponse{HasFullAccess: true}, monday))
	assert.Nil(t, dailyDepartureModes(StudentResponse{
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}, timezone.NewDate(2026, time.June, 6)))
}

func TestDailyDepartureModesSupportsLegacyRules(t *testing.T) {
	monday := timezone.NewDate(2026, time.June, 1)
	student := StudentResponse{
		PickupStatus:            "Wird abgeholt",
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}

	assert.Equal(t,
		[]users.DepartureMode{users.DeparturePickup},
		dailyDepartureModes(student, monday),
	)
}

func TestDailyDepartureMatchesFilterSupportsMultipleModes(t *testing.T) {
	modes := []users.DepartureMode{users.DepartureAlone, users.DeparturePickup}

	assert.True(t, dailyDepartureMatchesFilter(modes, "self"))
	assert.True(t, dailyDepartureMatchesFilter(modes, "pickedUp"))
	assert.False(t, dailyDepartureMatchesFilter(modes, "none"))
	assert.True(t, dailyDepartureMatchesFilter(nil, "none"))
	assert.True(t, dailyDepartureMatchesFilter([]users.DepartureMode{users.DepartureBus}, "other"))
	assert.True(t, dailyDepartureMatchesFilter([]users.DepartureMode{users.DepartureAccompanied}, "other"))
}
