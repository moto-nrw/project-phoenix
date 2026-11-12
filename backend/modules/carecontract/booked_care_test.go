package carecontract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func bookedDate(day int) calendar.Date { return calendar.NewDate(2027, 9, day) }

// 6 September 2027 is a Monday.
func TestBookedContractDayTakesTheLatestModuleEndOfTheNewestVersion(t *testing.T) {
	t.Parallel()
	lastDay := bookedDate(13)
	contract := carecontract.BookedContract{Versions: []carecontract.BookedVersion{
		{Number: 2, EffectiveFrom: bookedDate(20), Services: []carecontract.Service{
			{Weekday: "mon", ModuleName: "OGS", EndsAt: "15:00"},
		}},
		{Number: 1, EffectiveFrom: bookedDate(1), Services: []carecontract.Service{
			{Weekday: "mon", ModuleName: "Frühbetreuung", EndsAt: "08:00"},
			{Weekday: "mon", ModuleName: "OGS", EndsAt: "16:30"},
			{Weekday: "tue", ModuleName: "Spätbetreuung", EndsAt: "17:30", LastDay: &lastDay},
			{Weekday: "wed", ModuleName: "OGS", EndsAt: ""},
		}},
	}}

	monday := contract.Day(bookedDate(6))
	require.True(t, monday.InForce)
	assert.True(t, monday.Booked)
	require.NotNil(t, monday.EndsAt)
	assert.Equal(t, 16, monday.EndsAt.Hour(), "the latest module ends the day")
	assert.Equal(t, 30, monday.EndsAt.Minute())
	assert.Equal(t, []string{"Frühbetreuung", "OGS"}, monday.Modules)

	assert.True(t, contract.Day(bookedDate(7)).Booked, "Tuesday is booked up to its last day")
	tuesdayAfter := contract.Day(bookedDate(14))
	assert.True(t, tuesdayAfter.InForce)
	assert.False(t, tuesdayAfter.Booked, "after its last day the module no longer counts")

	wednesday := contract.Day(bookedDate(8))
	assert.True(t, wednesday.Booked)
	assert.Nil(t, wednesday.EndsAt, "a module without an end time leaves the end unknown")

	thursday := contract.Day(bookedDate(9))
	assert.True(t, thursday.InForce)
	assert.False(t, thursday.Booked, "a weekday without a module is a day without care")

	later := contract.Day(bookedDate(20))
	require.NotNil(t, later.EndsAt)
	assert.Equal(t, 15, later.EndsAt.Hour(), "the newer version decides from its date")

	assert.False(t, contract.Day(calendar.NewDate(2027, 8, 30)).InForce, "before every version the care plan decides")
}
