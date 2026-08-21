package students

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestDailyDepartureModes(t *testing.T) {
	t.Parallel()

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
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 1)).Modes,
	)
	assert.Equal(t,
		[]users.DepartureMode{users.DepartureAlone, users.DepartureBus},
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 5)).Modes,
	)
	assert.Equal(t,
		[]users.DepartureMode{users.DepartureAlone},
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 2)).Modes,
	)
}

func TestDailyDepartureModesRedactionAndMissingRule(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.June, 1)
	configured := StudentResponse{
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: {users.DeparturePickup},
		},
		DepartureRuleConfigured: true,
		HasFullAccess:           false,
	}

	assert.Nil(t, dailyDepartureForDate(configured, monday).Modes)
	assert.Nil(t, dailyDepartureForDate(StudentResponse{HasFullAccess: true}, monday).Modes)
	assert.Nil(t, dailyDepartureForDate(StudentResponse{
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}, timezone.NewDate(2026, time.June, 6)).Modes)
}

func TestDailyDepartureModesSupportsLegacyRules(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.June, 1)
	tests := []struct {
		status string
		mode   users.DepartureMode
	}{
		{status: "Geht alleine nach Hause", mode: users.DepartureAlone},
		{status: "self", mode: users.DepartureAlone},
		{status: "Wird abgeholt", mode: users.DeparturePickup},
		{status: "parent", mode: users.DeparturePickup},
		{status: "parents", mode: users.DeparturePickup},
		{status: "guardian", mode: users.DeparturePickup},
		{status: "picked_up", mode: users.DeparturePickup},
		{status: "Bus", mode: users.DepartureBus},
		{status: users.PickupStatusAccompanied, mode: users.DepartureAccompanied},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			student := StudentResponse{
				PickupStatus:            tt.status,
				DepartureRuleConfigured: true,
				HasFullAccess:           true,
			}
			departure := dailyDepartureForDate(student, monday)

			assert.Equal(t, []users.DepartureMode{tt.mode}, departure.Modes)
			assert.Empty(t, departure.LegacyLabel)
			assert.True(t, departure.Configured)
		})
	}
}

func TestDailyDeparturePreservesUnknownLegacyRuleAsNonSelf(t *testing.T) {
	t.Parallel()

	departure := dailyDepartureForDate(StudentResponse{
		PickupStatus:            "Taxi mit Begleitperson",
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}, timezone.NewDate(2026, time.June, 1))

	assert.Empty(t, departure.Modes)
	assert.Equal(t, "Taxi mit Begleitperson", departure.LegacyLabel)
	assert.True(t, departure.Configured)
	assert.False(t, dailyDepartureMatchesFilter(departure, "self"))
	assert.False(t, dailyDepartureMatchesFilter(departure, "pickedUp"))
	assert.False(t, dailyDepartureMatchesFilter(departure, "none"))
	assert.True(t, dailyDepartureMatchesFilter(departure, "other"))
}

func TestDailyDepartureMatchesFilterSupportsMultipleModes(t *testing.T) {
	t.Parallel()

	departure := dailyDeparture{
		Modes:      []users.DepartureMode{users.DepartureAlone, users.DeparturePickup},
		Configured: true,
	}

	assert.True(t, dailyDepartureMatchesFilter(departure, "self"))
	assert.True(t, dailyDepartureMatchesFilter(departure, "pickedUp"))
	assert.False(t, dailyDepartureMatchesFilter(departure, "none"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{}, "none"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{
		Modes:      []users.DepartureMode{users.DepartureBus},
		Configured: true,
	}, "other"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{
		Modes:      []users.DepartureMode{users.DepartureAccompanied},
		Configured: true,
	}, "other"))
}
