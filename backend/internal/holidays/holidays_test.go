package holidays

import (
	"testing"
	"time"

	"github.com/rickar/cal/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func date(y int, m time.Month, d int) timezone.Date {
	return timezone.NewDate(y, m, d)
}

func TestForYearNW2026(t *testing.T) {
	list, err := ForYear("DE-NW", 2026)
	require.NoError(t, err)

	byDate := make(map[timezone.Date]string, len(list))
	for _, h := range list {
		byDate[h.Date] = h.Name
	}

	// NRW has exactly 11 state-wide holidays.
	assert.Len(t, list, 11)
	assert.Equal(t, "Neujahrstag", byDate[date(2026, time.January, 1)])
	// Easter 2026 is April 5 — the movable feasts hang off it.
	assert.Equal(t, "Karfreitag", byDate[date(2026, time.April, 3)])
	assert.Equal(t, "Ostermontag", byDate[date(2026, time.April, 6)])
	assert.Equal(t, "Christi Himmelfahrt", byDate[date(2026, time.May, 14)])
	assert.Equal(t, "Pfingstmontag", byDate[date(2026, time.May, 25)])
	assert.Equal(t, "Fronleichnam", byDate[date(2026, time.June, 4)])
	assert.Equal(t, "Allerheiligen", byDate[date(2026, time.November, 1)])
	assert.Equal(t, "Zweiter Weihnachtsfeiertag", byDate[date(2026, time.December, 26)])

	// Sorted by date.
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1].Date.Before(list[i].Date))
	}
}

func TestRegionalDifferences(t *testing.T) {
	names := func(region string) map[string]bool {
		list, err := ForYear(region, 2026)
		require.NoError(t, err)
		set := make(map[string]bool, len(list))
		for _, h := range list {
			set[h.Name] = true
		}
		return set
	}

	assert.True(t, names("DE-SN")["Buß- und Bettag"])
	assert.False(t, names("DE-NW")["Buß- und Bettag"])

	assert.True(t, names("DE-BE")["Frauentag"])
	assert.True(t, names("DE-TH")["Weltkindertag"])
	assert.True(t, names("DE-SL")["Mariä Himmelfahrt"])
	assert.False(t, names("DE-BY")["Mariä Himmelfahrt"]) // only in Catholic municipalities — not state-wide

	assert.True(t, names("DE-HH")["Reformationstag"])
	assert.False(t, names("DE-NW")["Reformationstag"])
}

func TestForYearUnknownRegion(t *testing.T) {
	_, err := ForYear("DE-XX", 2026)
	assert.Error(t, err)
}

func TestForYearSkipsDefinitionsOutsideTheirValidityWindow(t *testing.T) {
	const region = "DE-TEST"
	regionHolidays[region] = []*cal.Holiday{{
		Name:      "Future holiday",
		StartYear: 2027,
		Month:     time.January,
		Day:       1,
		Func:      cal.CalcDayOfMonth,
	}}
	t.Cleanup(func() { delete(regionHolidays, region) })

	list, err := ForYear(region, 2026)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInRangeCrossesYears(t *testing.T) {
	list, err := InRange("DE-NW", date(2026, time.December, 20), date(2027, time.January, 10))
	require.NoError(t, err)

	require.Len(t, list, 3)
	assert.Equal(t, date(2026, time.December, 25), list[0].Date)
	assert.Equal(t, date(2026, time.December, 26), list[1].Date)
	assert.Equal(t, date(2027, time.January, 1), list[2].Date)
}

func TestInRangeInvalid(t *testing.T) {
	_, err := InRange("DE-NW", date(2026, time.June, 2), date(2026, time.June, 1))
	assert.Error(t, err)
}

func TestInRangeRejectsUnknownRegion(t *testing.T) {
	_, err := InRange("DE-XX", date(2026, time.June, 1), date(2026, time.June, 2))
	assert.Error(t, err)
}

func TestDateSet(t *testing.T) {
	set, err := DateSet("DE-NW", date(2026, time.May, 1), date(2026, time.May, 31))
	require.NoError(t, err)

	assert.True(t, set[date(2026, time.May, 1)])  // Tag der Arbeit
	assert.True(t, set[date(2026, time.May, 14)]) // Christi Himmelfahrt
	assert.True(t, set[date(2026, time.May, 25)]) // Pfingstmontag
	assert.False(t, set[date(2026, time.May, 2)])
}

func TestDateSetPropagatesRangeErrors(t *testing.T) {
	_, err := DateSet("DE-NW", date(2026, time.June, 2), date(2026, time.June, 1))
	assert.Error(t, err)
}

func TestRegionsComplete(t *testing.T) {
	regions := Regions()
	assert.Len(t, regions, 16)
	for _, code := range regions {
		assert.True(t, ValidRegion(code))
	}
	assert.True(t, ValidRegion(DefaultRegion))
}
