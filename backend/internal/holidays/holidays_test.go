package holidays

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func date(y int, m time.Month, d int) timezone.Date {
	return timezone.NewDate(y, m, d)
}

func TestForYearNW2026(t *testing.T) {
	t.Parallel()

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

	nameOn := func(region string, year int, d timezone.Date) string {
		list, err := ForYear(region, year)
		require.NoError(t, err)
		for _, h := range list {
			if h.Date == d {
				return h.Name
			}
		}
		return ""
	}

	// Frauentag is statutory in MV only since 2023.
	assert.Empty(t, nameOn("DE-MV", 2022, date(2022, time.March, 8)))
	assert.Equal(t, "Internationaler Frauentag", nameOn("DE-MV", 2023, date(2023, time.March, 8)))

	// Berlin's Tag der Befreiung was a one-off holiday in 2020 and 2025.
	assert.Empty(t, nameOn("DE-BE", 2024, date(2024, time.May, 8)))
	assert.Equal(t, "Tag der Befreiung", nameOn("DE-BE", 2025, date(2025, time.May, 8)))

	// Weltkindertag is statutory in Thüringen only since 2019.
	assert.Empty(t, nameOn("DE-TH", 2018, date(2018, time.September, 20)))
	assert.Equal(t, "Weltkindertag", nameOn("DE-TH", 2019, date(2019, time.September, 20)))
}

func TestBrandenburgIncludesEasterAndPentecostSundays(t *testing.T) {
	t.Parallel()

	// Easter 2026 is April 5, Pentecost Sunday May 24. Statutory holidays
	// in Brandenburg only.
	bb, err := DateSet("DE-BB", date(2026, time.January, 1), date(2026, time.December, 31))
	require.NoError(t, err)
	assert.True(t, bb[date(2026, time.April, 5)])
	assert.True(t, bb[date(2026, time.May, 24)])

	nw, err := DateSet("DE-NW", date(2026, time.January, 1), date(2026, time.December, 31))
	require.NoError(t, err)
	assert.False(t, nw[date(2026, time.April, 5)])
	assert.False(t, nw[date(2026, time.May, 24)])

	list, err := ForYear("DE-BB", 2026)
	require.NoError(t, err)
	names := make(map[timezone.Date]string, len(list))
	for _, h := range list {
		names[h.Date] = h.Name
	}
	assert.Equal(t, "Ostersonntag", names[date(2026, time.April, 5)])
	assert.Equal(t, "Pfingstsonntag", names[date(2026, time.May, 24)])
}

func TestDisplayNameNormalization(t *testing.T) {
	t.Parallel()

	list, err := ForYear("DE-NW", 2026)
	require.NoError(t, err)
	names := make(map[timezone.Date]string, len(list))
	for _, h := range list {
		names[h.Date] = h.Name
	}

	assert.Equal(t, "Neujahrstag", names[date(2026, time.January, 1)])
	assert.Equal(t, "Tag der Deutschen Einheit", names[date(2026, time.October, 3)])
	assert.Equal(t, "Erster Weihnachtsfeiertag", names[date(2026, time.December, 25)])

	by, err := ForYear("DE-BY", 2026)
	require.NoError(t, err)
	found := ""
	for _, h := range by {
		if h.Date == date(2026, time.January, 6) {
			found = h.Name
		}
	}
	assert.Equal(t, "Heilige Drei Könige", found)
}

func TestInRangeCrossesYears(t *testing.T) {
	t.Parallel()

	list, err := InRange("DE-NW", date(2026, time.December, 20), date(2027, time.January, 10))
	require.NoError(t, err)

	require.Len(t, list, 3)
	assert.Equal(t, date(2026, time.December, 25), list[0].Date)
	assert.Equal(t, date(2026, time.December, 26), list[1].Date)
	assert.Equal(t, date(2027, time.January, 1), list[2].Date)
}

func TestInRangeInvalid(t *testing.T) {
	t.Parallel()

	_, err := InRange("DE-NW", date(2026, time.June, 2), date(2026, time.June, 1))
	assert.Error(t, err)
}

func TestInRangeRejectsUnknownRegion(t *testing.T) {
	t.Parallel()

	_, err := InRange("DE-XX", date(2026, time.June, 1), date(2026, time.June, 2))
	assert.Error(t, err)
}

func TestDateSet(t *testing.T) {
	t.Parallel()

	set, err := DateSet("DE-NW", date(2026, time.May, 1), date(2026, time.May, 31))
	require.NoError(t, err)

	assert.True(t, set[date(2026, time.May, 1)])  // Tag der Arbeit
	assert.True(t, set[date(2026, time.May, 14)]) // Christi Himmelfahrt
	assert.True(t, set[date(2026, time.May, 25)]) // Pfingstmontag
	assert.False(t, set[date(2026, time.May, 2)])
}

func TestDateSetPropagatesRangeErrors(t *testing.T) {
	t.Parallel()

	_, err := DateSet("DE-NW", date(2026, time.June, 2), date(2026, time.June, 1))
	assert.Error(t, err)
}

func TestRegionsComplete(t *testing.T) {
	t.Parallel()

	regions := Regions()
	assert.Len(t, regions, 16)
	for _, code := range regions {
		assert.True(t, ValidRegion(code))
	}
	assert.True(t, ValidRegion(DefaultRegion))
}
