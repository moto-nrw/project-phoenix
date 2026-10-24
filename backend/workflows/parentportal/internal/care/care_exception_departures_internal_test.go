package care

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The parent's day list carries the Abholart the OGS set or approved (#3945).
// It joins a day that already has a Gehzeit and becomes its own day
// otherwise, without making that day's Gehzeit staff-owned.
func TestMergeDepartureExceptions(t *testing.T) {
	t.Parallel()

	pickup := time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC)
	days := []*CareException{{
		Date: calendar.NewDate(2026, time.September, 10), PickupTime: &pickup,
		Source: careplan.ExceptionSourceGuardian, PickupSource: careplan.ExceptionSourceGuardian,
	}}
	merged := mergeDepartureExceptions(days, []DepartureException{
		{ExceptionDate: calendar.Date(calendar.NewDate(2026, time.September, 10)), DepartureMode: careplan.DepartureModePickup},
		{ExceptionDate: calendar.Date(calendar.NewDate(2026, time.September, 9)), DepartureMode: careplan.DepartureModeBus},
	})

	require.Len(t, merged, 2)
	assert.Equal(t, calendar.NewDate(2026, time.September, 9), merged[0].Date, "days stay in date order")
	require.NotNil(t, merged[0].DepartureMode)
	assert.Equal(t, "bus", *merged[0].DepartureMode)
	assert.Nil(t, merged[0].PickupTime)
	assert.Equal(t, careplan.ExceptionSourceStaff, merged[0].Source)
	assert.Empty(t, merged[0].PickupSource, "an Abholart alone owns no Gehzeit")

	require.NotNil(t, merged[1].DepartureMode)
	assert.Equal(t, "pickup", *merged[1].DepartureMode)
	assert.Equal(t, careplan.ExceptionSourceGuardian, merged[1].PickupSource,
		"the Gehzeit of the day keeps its own author")

	assert.Same(t, days[0], mergeDepartureExceptions(days, nil)[0])
}
