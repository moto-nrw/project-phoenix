package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The class day and the supervision sheet show the Abholart the OGS set or
// approved for the day (#3945) instead of the weekly plan; children without
// one keep the weekday's modes.
func TestClassDayDeparturesUseTheDayAbholart(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{
		Times: fakeDayTimes{departures: map[int64]string{1: "pickup"}},
	})
	students := []DayRosterStudent{
		{ID: 1, DepartureModes: []peopledirectory.DepartureMode{peopledirectory.DepartureBus}},
		{ID: 2, DepartureModes: []peopledirectory.DepartureMode{peopledirectory.DepartureBus}},
	}

	departures, err := svc.classDayDepartures(context.Background(), students, calendar.NewDate(2026, 9, 9), "wed")
	require.NoError(t, err)
	assert.Equal(t, "Abholung", departures[1])
	assert.Equal(t, "Bus", departures[2])
}

func TestSupervisionDepartureUsesTheDayAbholart(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{
		Times: fakeDayTimes{departures: map[int64]string{7: "alone"}},
	})
	student := &SheetStudent{ID: 7, AllowedDepartureModes: peopledirectory.AllowedDepartureModes{
		"wed": {peopledirectory.DeparturePickup},
	}}

	departure, err := svc.supervisionDeparture(context.Background(), student, calendar.NewDate(2026, 9, 9), []int64{7})
	require.NoError(t, err)
	assert.Equal(t, "Geht alleine", departure)
}
