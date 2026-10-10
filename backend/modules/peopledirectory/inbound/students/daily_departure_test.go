package students

import (
	"context"
	"testing"
	"time"

	dep "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestDailyDepartureModes(t *testing.T) {
	t.Parallel()

	student := StudentResponse{
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: {dep.DeparturePickup},
			dep.PickupDayFriday: {dep.DepartureAlone, dep.DepartureBus},
		},
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}

	assert.Equal(t,
		[]dep.DepartureMode{dep.DeparturePickup},
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 1)).Modes,
	)
	assert.Equal(t,
		[]dep.DepartureMode{dep.DepartureAlone, dep.DepartureBus},
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 5)).Modes,
	)
	assert.Equal(t,
		[]dep.DepartureMode{dep.DepartureAlone},
		dailyDepartureForDate(student, timezone.NewDate(2026, time.June, 2)).Modes,
	)
}

func TestDailyDepartureModesRedactionAndMissingRule(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.June, 1)
	configured := StudentResponse{
		AllowedDepartureModes: dep.AllowedDepartureModes{
			dep.PickupDayMonday: {dep.DeparturePickup},
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
		mode   dep.DepartureMode
	}{
		{status: "Geht alleine nach Hause", mode: dep.DepartureAlone},
		{status: "self", mode: dep.DepartureAlone},
		{status: "Wird abgeholt", mode: dep.DeparturePickup},
		{status: "parent", mode: dep.DeparturePickup},
		{status: "parents", mode: dep.DeparturePickup},
		{status: "guardian", mode: dep.DeparturePickup},
		{status: "picked_up", mode: dep.DeparturePickup},
		{status: "Bus", mode: dep.DepartureBus},
		{status: dep.PickupStatusAccompanied, mode: dep.DepartureAccompanied},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			student := StudentResponse{
				PickupStatus:            tt.status,
				DepartureRuleConfigured: true,
				HasFullAccess:           true,
			}
			departure := dailyDepartureForDate(student, monday)

			assert.Equal(t, []dep.DepartureMode{tt.mode}, departure.Modes)
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
		Modes:      []dep.DepartureMode{dep.DepartureAlone, dep.DeparturePickup},
		Configured: true,
	}

	assert.True(t, dailyDepartureMatchesFilter(departure, "self"))
	assert.True(t, dailyDepartureMatchesFilter(departure, "pickedUp"))
	assert.False(t, dailyDepartureMatchesFilter(departure, "none"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{}, "none"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{
		Modes:      []dep.DepartureMode{dep.DepartureBus},
		Configured: true,
	}, "other"))
	assert.True(t, dailyDepartureMatchesFilter(dailyDeparture{
		Modes:      []dep.DepartureMode{dep.DepartureAccompanied},
		Configured: true,
	}, "other"))
}

// A weekend that follows Friday's plan (#3921) reads the departure plan and
// the companions of the Friday before; a weekday never reads the setting.
func TestDeparturePlanDateFollowsFriday(t *testing.T) {
	t.Parallel()

	friday := timezone.NewDate(2026, time.June, 5)
	calls := 0
	following := func(follows bool) context.Context {
		return calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) {
			calls++
			return follows, nil
		})
	}
	for _, day := range []timezone.Date{friday.AddDays(1), friday.AddDays(2)} {
		got, err := departurePlanDate(following(true), day)
		assert.NoError(t, err)
		assert.Equal(t, friday, got)
		got, err = departurePlanDate(following(false), day)
		assert.NoError(t, err)
		assert.Equal(t, day, got)
	}
	calls = 0
	got, err := departurePlanDate(following(true), friday)
	assert.NoError(t, err)
	assert.Equal(t, friday, got)
	assert.Zero(t, calls)
}
