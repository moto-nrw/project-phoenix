package presenceprojectionhttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// The "Klassenliste" preset renders its weekday cells through the shared
// class-list table (#4749): the Kind-Gehzeit plus the day's
// Geh-/Abholregelung, like the Klassenliste of an enrollment phase. A day
// with only an arrival is still a care day; a day without a plan is "—".
func TestClassRosterExportRowUsesSharedWeekdayCell(t *testing.T) {
	t.Parallel()

	student := StudentResponse{
		ID:          1,
		FirstName:   "Mila",
		LastName:    "Anders",
		SchoolClass: "1a",
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday:    {departure.DeparturePickup},
			departure.PickupDayWednesday: {departure.DepartureAccompanied},
		},
		DepartureCompanions: []departure.CompanionLink{
			{CompanionStudentID: 2, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{departure.PickupDayWednesday}},
		},
	}
	plan := weeklySchedule{
		ArrivalByWeekday:  map[int]string{weekdayTuesday: "11:45"},
		CareDaysByWeekday: map[int]bool{weekdayMonday: true, weekdayWednesday: true},
		PickupByWeekday:   map[int]string{weekdayMonday: "14:30", weekdayWednesday: "16:00"},
	}
	base := buildExportRow(student, plan, map[int64]string{1: "Angemeldet: Fußball"}, testExportDate, true)

	row := classRosterExportRow(student, plan, base)

	assert.Equal(t, "Mila Anders", row.Values[lists.ColumnName])
	assert.Equal(t, "1a", row.Values[lists.ColumnSchoolClass])
	assert.Equal(t, "14:30 Uhr, wird abgeholt", row.Values[lists.ColumnWeeklyMonday])
	assert.Equal(t, "Keine Abholzeit, geht alleine", row.Values[lists.ColumnWeeklyTuesday])
	assert.Equal(t, "16:00 Uhr, mit anderem Kind (mit: Mia Schulz)", row.Values[lists.ColumnWeeklyWednesday])
	assert.Equal(t, "—", row.Values[lists.ColumnWeeklyThursday])
	assert.Equal(t, "—", row.Values[lists.ColumnWeeklyFriday])
	assert.Equal(t, "Angemeldet: Fußball", row.Values[lists.ColumnEnrollmentSummary],
		"columns the shared table does not own keep the generic export's value")
}

// The generic row carries the Wochenliste cell; only the "Klassenliste" swaps
// it, the OGS Wochenliste keeps "Ankunft/Abholung".
func TestApplyClassRosterRowsReplacesWochenlisteCells(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{{ID: 1, FirstName: "Ben", LastName: "Berg", SchoolClass: "2b"}}
	weekly := map[int64]weeklySchedule{1: {
		ArrivalByWeekday:  map[int]string{weekdayMonday: "08:00"},
		CareDaysByWeekday: map[int]bool{weekdayMonday: true},
		PickupByWeekday:   map[int]string{weekdayMonday: "15:00"},
	}}
	sources := responseRowSources(students, weekly, map[int64]string{}, testExportDate, true)
	require.Equal(t, "Ankunft: 08:00, Abholung: 15:00", sources[0].row.Values[lists.ColumnWeeklyMonday])

	applyClassRosterRows(sources, students, weekly)

	assert.Equal(t, "15:00 Uhr, geht alleine", sources[0].row.Values[lists.ColumnWeeklyMonday])
	assert.Equal(t, "2b", sources[0].schoolClass, "the merge sort keys stay untouched")
}

// The companion links are loaded only for exports that print a departure
// rule; the weekday cells of the "Klassenliste" are one of them.
func TestExportPrintsDepartureRules(t *testing.T) {
	t.Parallel()

	weekday := lists.ResolveColumns([]lists.ColumnID{lists.ColumnName, lists.ColumnWeeklyMonday}, lists.PresetClassRoster)
	nameOnly := lists.ResolveColumns([]lists.ColumnID{lists.ColumnName}, lists.PresetClassRoster)
	departureColumn := lists.ResolveColumns([]lists.ColumnID{lists.ColumnDeparture}, lists.PresetOGSWeekly)

	assert.True(t, exportPrintsDepartureRules(lists.PresetClassRoster, weekday))
	assert.False(t, exportPrintsDepartureRules(lists.PresetClassRoster, nameOnly))
	assert.False(t, exportPrintsDepartureRules(lists.PresetOGSWeekly, weekday))
	assert.True(t, exportPrintsDepartureRules(lists.PresetOGSWeekly, departureColumn))
}
