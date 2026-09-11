package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func holidayNames(t *testing.T, region, from, to string) map[string]string {
	t.Helper()
	list, err := HolidaysInRange(region, from, to)
	require.NoError(t, err)
	names := make(map[string]string, len(list))
	for _, holiday := range list {
		names[holiday.Date] = holiday.Name
	}
	return names
}

func TestHolidaysInRangePreservesGermanRegionalRules(t *testing.T) {
	t.Parallel()

	nw, err := HolidaysInRange("DE-NW", "2026-01-01", "2026-12-31")
	require.NoError(t, err)
	assert.Len(t, nw, 11)
	for index := 1; index < len(nw); index++ {
		assert.Less(t, nw[index-1].Date, nw[index].Date)
	}
	nwNames := holidayNames(t, "DE-NW", "2026-01-01", "2026-12-31")
	assert.Equal(t, "Neujahrstag", nwNames["2026-01-01"])
	assert.Equal(t, "Karfreitag", nwNames["2026-04-03"])
	assert.Equal(t, "Ostermontag", nwNames["2026-04-06"])
	assert.Equal(t, "Christi Himmelfahrt", nwNames["2026-05-14"])
	assert.Equal(t, "Pfingstmontag", nwNames["2026-05-25"])
	assert.Equal(t, "Fronleichnam", nwNames["2026-06-04"])
	assert.Equal(t, "Tag der Deutschen Einheit", nwNames["2026-10-03"])
	assert.Equal(t, "Allerheiligen", nwNames["2026-11-01"])
	assert.Equal(t, "Erster Weihnachtsfeiertag", nwNames["2026-12-25"])
	assert.Equal(t, "Zweiter Weihnachtsfeiertag", nwNames["2026-12-26"])

	assert.Equal(t, "Buß- und Bettag", holidayNames(t, "DE-SN", "2026-01-01", "2026-12-31")["2026-11-18"])
	assert.Equal(t, "Internationaler Frauentag", holidayNames(t, "DE-BE", "2026-01-01", "2026-12-31")["2026-03-08"])
	assert.Equal(t, "Weltkindertag", holidayNames(t, "DE-TH", "2026-01-01", "2026-12-31")["2026-09-20"])
	assert.Equal(t, "Mariä Himmelfahrt", holidayNames(t, "DE-SL", "2026-01-01", "2026-12-31")["2026-08-15"])
	assert.Empty(t, holidayNames(t, "DE-BY", "2026-01-01", "2026-12-31")["2026-08-15"])
}

func TestHolidaysInRangePreservesHistoricalValidityWindows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		region, date, want string
	}{
		{"DE-MV", "2022-03-08", ""},
		{"DE-MV", "2023-03-08", "Internationaler Frauentag"},
		{"DE-BE", "2024-05-08", ""},
		{"DE-BE", "2025-05-08", "Tag der Befreiung"},
		{"DE-TH", "2018-09-20", ""},
		{"DE-TH", "2019-09-20", "Weltkindertag"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, holidayNames(t, tc.region, tc.date, tc.date)[tc.date], tc.region+" "+tc.date)
	}
}

func TestHolidayRangesAndDateSet(t *testing.T) {
	t.Parallel()

	list, err := HolidaysInRange("DE-NW", "2026-12-20", "2027-01-10")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{"2026-12-25", "2026-12-26", "2027-01-01"}, []string{list[0].Date, list[1].Date, list[2].Date})

	set, err := HolidayDates("DE-BB", "2026-01-01", "2026-12-31")
	require.NoError(t, err)
	assert.True(t, set["2026-04-05"], "Brandenburg includes Easter Sunday")
	assert.True(t, set["2026-05-24"], "Brandenburg includes Pentecost Sunday")
	assert.False(t, set["2026-05-02"])
}

func TestHolidaysInRangeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	for _, input := range [][3]string{
		{"DE-XX", "2026-01-01", "2026-12-31"},
		{"DE-NW", "not-a-date", "2026-12-31"},
		{"DE-NW", "2026-01-01", "not-a-date"},
	} {
		_, err := HolidaysInRange(input[0], input[1], input[2])
		assert.Error(t, err)
	}
}
