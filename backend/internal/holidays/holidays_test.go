package holidays

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namesByDate indexes a holiday list by its ISO date string, so the
// assertions can look days up without naming the calendar-date type.
func namesByDate(list []Holiday) map[string]string {
	names := make(map[string]string, len(list))
	for _, h := range list {
		names[string(h.Date)] = h.Name
	}
	return names
}

func iso(y int, m time.Month, d int) string {
	return string(testpkg.Date(y, m, d))
}

func TestForYearNW2026(t *testing.T) {
	t.Parallel()

	list, err := ForYear("DE-NW", 2026)
	require.NoError(t, err)

	byDate := namesByDate(list)

	// NRW has exactly 11 state-wide holidays.
	assert.Len(t, list, 11)
	assert.Equal(t, "Neujahrstag", byDate[iso(2026, time.January, 1)])
	// Easter 2026 is April 5 — the movable feasts hang off it.
	assert.Equal(t, "Karfreitag", byDate[iso(2026, time.April, 3)])
	assert.Equal(t, "Ostermontag", byDate[iso(2026, time.April, 6)])
	assert.Equal(t, "Christi Himmelfahrt", byDate[iso(2026, time.May, 14)])
	assert.Equal(t, "Pfingstmontag", byDate[iso(2026, time.May, 25)])
	assert.Equal(t, "Fronleichnam", byDate[iso(2026, time.June, 4)])
	assert.Equal(t, "Allerheiligen", byDate[iso(2026, time.November, 1)])
	assert.Equal(t, "Zweiter Weihnachtsfeiertag", byDate[iso(2026, time.December, 26)])

	// Sorted by date.
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1].Date.Before(list[i].Date))
	}
}

func TestRegionalDifferences(t *testing.T) {
	t.Parallel()

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

	assert.True(t, names("DE-BE")["Internationaler Frauentag"])
	assert.True(t, names("DE-TH")["Weltkindertag"])
	assert.True(t, names("DE-SL")["Mariä Himmelfahrt"])
	assert.False(t, names("DE-BY")["Mariä Himmelfahrt"]) // only in Catholic municipalities — not state-wide

	assert.True(t, names("DE-HH")["Reformationstag"])
	assert.False(t, names("DE-NW")["Reformationstag"])
}

func TestForYearUnknownRegion(t *testing.T) {
	t.Parallel()

	_, err := ForYear("DE-XX", 2026)
	assert.Error(t, err)
}

func TestForYearRespectsHolidayValidityWindows(t *testing.T) {
	t.Parallel()

	nameOn := func(region string, year int, d string) string {
		list, err := ForYear(region, year)
		require.NoError(t, err)
		return namesByDate(list)[d]
	}

	// Frauentag is statutory in MV only since 2023.
	assert.Empty(t, nameOn("DE-MV", 2022, iso(2022, time.March, 8)))
	assert.Equal(t, "Internationaler Frauentag", nameOn("DE-MV", 2023, iso(2023, time.March, 8)))

	// Berlin's Tag der Befreiung was a one-off holiday in 2020 and 2025.
	assert.Empty(t, nameOn("DE-BE", 2024, iso(2024, time.May, 8)))
	assert.Equal(t, "Tag der Befreiung", nameOn("DE-BE", 2025, iso(2025, time.May, 8)))

	// Weltkindertag is statutory in Thüringen only since 2019.
	assert.Empty(t, nameOn("DE-TH", 2018, iso(2018, time.September, 20)))
	assert.Equal(t, "Weltkindertag", nameOn("DE-TH", 2019, iso(2019, time.September, 20)))
}

func TestBrandenburgIncludesEasterAndPentecostSundays(t *testing.T) {
	t.Parallel()

	// Easter 2026 is April 5, Pentecost Sunday May 24. Statutory holidays
	// in Brandenburg only.
	bb, err := DateSet("DE-BB", testpkg.Date(2026, time.January, 1), testpkg.Date(2026, time.December, 31))
	require.NoError(t, err)
	assert.True(t, bb[testpkg.Date(2026, time.April, 5)])
	assert.True(t, bb[testpkg.Date(2026, time.May, 24)])

	nw, err := DateSet("DE-NW", testpkg.Date(2026, time.January, 1), testpkg.Date(2026, time.December, 31))
	require.NoError(t, err)
	assert.False(t, nw[testpkg.Date(2026, time.April, 5)])
	assert.False(t, nw[testpkg.Date(2026, time.May, 24)])

	list, err := ForYear("DE-BB", 2026)
	require.NoError(t, err)
	names := namesByDate(list)
	assert.Equal(t, "Ostersonntag", names[iso(2026, time.April, 5)])
	assert.Equal(t, "Pfingstsonntag", names[iso(2026, time.May, 24)])
}

func TestDisplayNameNormalization(t *testing.T) {
	t.Parallel()

	list, err := ForYear("DE-NW", 2026)
	require.NoError(t, err)
	names := namesByDate(list)

	assert.Equal(t, "Neujahrstag", names[iso(2026, time.January, 1)])
	assert.Equal(t, "Tag der Deutschen Einheit", names[iso(2026, time.October, 3)])
	assert.Equal(t, "Erster Weihnachtsfeiertag", names[iso(2026, time.December, 25)])

	by, err := ForYear("DE-BY", 2026)
	require.NoError(t, err)
	assert.Equal(t, "Heilige Drei Könige", namesByDate(by)[iso(2026, time.January, 6)])
}

func TestInRangeCrossesYears(t *testing.T) {
	t.Parallel()

	list, err := InRange("DE-NW", testpkg.Date(2026, time.December, 20), testpkg.Date(2027, time.January, 10))
	require.NoError(t, err)

	require.Len(t, list, 3)
	assert.Equal(t, testpkg.Date(2026, time.December, 25), list[0].Date)
	assert.Equal(t, testpkg.Date(2026, time.December, 26), list[1].Date)
	assert.Equal(t, testpkg.Date(2027, time.January, 1), list[2].Date)
}

func TestInRangeInvalid(t *testing.T) {
	t.Parallel()

	_, err := InRange("DE-NW", testpkg.Date(2026, time.June, 2), testpkg.Date(2026, time.June, 1))
	assert.Error(t, err)
}

func TestInRangeRejectsUnknownRegion(t *testing.T) {
	t.Parallel()

	_, err := InRange("DE-XX", testpkg.Date(2026, time.June, 1), testpkg.Date(2026, time.June, 2))
	assert.Error(t, err)
}

func TestDateSet(t *testing.T) {
	t.Parallel()

	set, err := DateSet("DE-NW", testpkg.Date(2026, time.May, 1), testpkg.Date(2026, time.May, 31))
	require.NoError(t, err)

	assert.True(t, set[testpkg.Date(2026, time.May, 1)])  // Tag der Arbeit
	assert.True(t, set[testpkg.Date(2026, time.May, 14)]) // Christi Himmelfahrt
	assert.True(t, set[testpkg.Date(2026, time.May, 25)]) // Pfingstmontag
	assert.False(t, set[testpkg.Date(2026, time.May, 2)])
}

func TestDateSetPropagatesRangeErrors(t *testing.T) {
	t.Parallel()

	_, err := DateSet("DE-NW", testpkg.Date(2026, time.June, 2), testpkg.Date(2026, time.June, 1))
	assert.Error(t, err)
}
