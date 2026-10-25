package mealplan_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// #3873: the delivery list has one line per day and class when the school
// orders by class, with the net change after the deadline, and skips
// closed and unordered days.
func TestDeliveryLinesPerDayAndClass(t *testing.T) {
	t.Parallel()
	week := mealplan.OrderWeek{Detail: "by_class", Days: []mealplan.OrderDay{
		{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3,
			Classes: []mealplan.ClassCount{{SchoolClass: "2a", Count: 2}, {SchoolClass: "1b", Count: 1}},
			Changes: []mealplan.OrderChange{{Child: mealplan.OrderChild{SchoolClass: "2a"}, Delta: 1}, {Child: mealplan.OrderChild{SchoolClass: "2a"}, Delta: 1}, {Child: mealplan.OrderChild{SchoolClass: "1b"}, Delta: -1}}},
		{Date: "2026-09-08", Closed: true},
		{Date: "2026-09-04", Status: mealplan.OrderUnordered},
	}}
	lines, err := week.DeliveryLines("OGS am Markt", "=Hof")
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"07.09.2026", "OGS am Markt", "'=Hof", "1b", "1", "−1"},
		{"07.09.2026", "OGS am Markt", "'=Hof", "2a", "2", "+2"},
	}, lines, "a school's text never starts a formula")

	week.Detail = "total"
	lines, err = week.DeliveryLines("OGS am Markt", "")
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"07.09.2026", "OGS am Markt", "", "Alle", "3", "+1"}}, lines)
}

// #3873: the CSV opens in a German spreadsheet: semicolons, a byte order
// mark and the fixed header.
func TestDeliveryExportAsCSV(t *testing.T) {
	t.Parallel()
	export, err := mealplan.DeliveryExport("2026-09-07", "2026-09-11", [][]string{{"07.09.2026", "OGS", "", "Alle", "3", ""}})
	require.NoError(t, err)
	assert.Equal(t, "Vom 07.09.2026 bis 11.09.2026", export.Subtitle)
	assert.Equal(t, "lieferliste-2026-09-07-bis-2026-09-11", export.Filename)
	data, err := export.CSV()
	require.NoError(t, err)
	text := string(data)
	assert.True(t, strings.HasPrefix(text, "\uFEFFDatum;Schule;Lieferadresse;Klasse oder Gruppe;Anzahl;Nachmeldung\n"), text)
	assert.Contains(t, text, "07.09.2026;OGS;;Alle;3;\n")
}
