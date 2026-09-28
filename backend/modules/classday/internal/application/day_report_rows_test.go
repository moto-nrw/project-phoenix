package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Enrollment's day roster no longer carries the roster's per-day departure
// form answers (DepartureByDay) at all: the rows below cannot leak them, and
// the departure assertions pin that the day plan is the only source.

func TestBuildClassDayReportProjection(t *testing.T) {
	t.Parallel()

	rows := []DayRosterRow{
		{
			StudentID:  1,
			FirstName:  "Mila",
			LastName:   "Anders",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Ganztag"},
			},
			PickupByDay:  map[string]string{"wed": "15:00"},
			ArrivalByDay: map[string]string{"wed": "07:30"},
		},
		{
			StudentID:  2,
			FirstName:  "Finn",
			LastName:   "Becker",
			Registered: false,
		},
		{
			StudentID:  3,
			FirstName:  "Ida",
			LastName:   "Conrad",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Randstunde"},
			},
		},
	}
	statuses := map[int64]string{3: statusDaySick}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr 2026/27", rows, classDayFacts{
		statuses:   statuses,
		departures: map[int64]string{1: "Abholung"},
		pickups:    map[int64]string{1: "16:00"},
	})

	require.Len(t, report.Rows, 3)
	assert.Equal(t, "wed", report.Weekday)
	assert.True(t, report.SchoolDay)
	assert.Equal(t, "Schuljahr 2026/27", report.PhaseName)

	assert.True(t, report.Rows[0].StaysToday)
	assert.Equal(t, []string{"Ganztag"}, report.Rows[0].Offerings)
	// The effective projected pickup time beats the form answer.
	assert.Equal(t, "16:00", report.Rows[0].Pickup)
	assert.Equal(t, "07:30", report.Rows[0].Arrival)
	// The day-specific departure is the ONLY source on a school day.
	assert.Equal(t, "Abholung", report.Rows[0].Departure)
	// A child without any per-day plan renders as explicit "Keine Angabe".
	assert.Equal(t, "Keine Angabe", report.Rows[1].Departure)
	assert.Equal(t, "Keine Angabe", report.Rows[2].Departure)

	assert.False(t, report.Rows[1].StaysToday)
	assert.Empty(t, report.Rows[1].Offerings)

	// A reported sick day wins over the enrollment: the student is absent,
	// not staying, even though the weekday has an offering.
	assert.False(t, report.Rows[2].StaysToday)
	assert.Equal(t, "sick", report.Rows[2].Status)

	assert.Equal(t, classday.DayTotals{Students: 3, Staying: 1, Leaving: 1, Absent: 1}, report.Totals)
}

func TestBuildClassDayReportWeekend(t *testing.T) {
	t.Parallel()

	rows := []DayRosterRow{{
		StudentID:      1,
		Registered:     true,
		OfferingsByDay: map[string][]string{"mon": {"Ganztag"}},
	}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 8), "", rows, classDayFacts{
		// Even a departure fact must not reach a weekend row.
		departures: map[int64]string{1: "Bus"},
	})

	assert.False(t, report.SchoolDay)
	assert.Equal(t, "", report.Weekday)
	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].StaysToday)
	// Kein Schultag: keine Übergabe, also auch keine Abgangs-Aussage — die
	// Tageswerte dürfen nicht ins Einzeltag-Feld durchsickern (auch nicht für
	// Nicht-UI-Consumer des Endpoints).
	assert.Equal(t, "", report.Rows[0].Departure)
	// Kein Schultag: nur der Klassenverband ist eine ehrliche Zahl.
	assert.Equal(t, classday.DayTotals{Students: 1}, report.Totals)
}

func TestBuildClassDayReportNotScheduledOverridesOffering(t *testing.T) {
	t.Parallel()

	rows := []DayRosterRow{{
		StudentID:      1,
		Registered:     true,
		OfferingsByDay: map[string][]string{"wed": {"Ganztag"}},
	}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{notScheduled: map[int64]bool{1: true}})

	require.Len(t, report.Rows, 1)
	// The approved offering still lists Wednesday, but the parents struck
	// the weekday from the care plan: the child goes home, not "bleibt in
	// der Betreuung" without a pickup time.
	assert.False(t, report.Rows[0].StaysToday)
	assert.Equal(t, "", report.Rows[0].Status)
	assert.Equal(t, classday.DayTotals{Students: 1, Leaving: 1}, report.Totals)
}

func TestClassDayReportRendersUnknownForUnplannedDay(t *testing.T) {
	t.Parallel()

	// Real classDayDeparture output, not a hand-built impossible state: the
	// day map carries what classDayDeparture actually returns for a day the
	// plan does not cover.
	monOnly := peopledirectory.AllowedDepartureModes{
		"mon": {peopledirectory.DepartureBus},
	}
	rows := []DayRosterRow{
		{StudentID: 1, Registered: true},
		{StudentID: 2, Registered: true},
	}
	departures := map[int64]string{
		1: classDayDeparture(weekdayDepartureModes(monOnly, nil, "wed"), "wed", nil, nil),
		2: classDayDeparture(weekdayDepartureModes(nil, nil, "wed"), "wed", nil, nil),
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{departures: departures})

	require.Len(t, report.Rows, 2)
	// A Wednesday sheet must neither print a Monday-only plan value nor
	// fabricate "Geht alleine" for a child without any plan.
	assert.Equal(t, "Keine Angabe", report.Rows[0].Departure)
	assert.Equal(t, "Keine Angabe", report.Rows[1].Departure)
}

// TestBuildClassDayReportCarriesPickupDeviation checks the projection onto
// the row a Lehrkraft reads.
func TestBuildClassDayReportCarriesPickupDeviation(t *testing.T) {
	t.Parallel()

	recorded := time.Date(2026, 8, 5, 8, 40, 0, 0, time.UTC)
	rows := []DayRosterRow{
		{StudentID: 1, LastName: "Adam", Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}},
		{StudentID: 2, LastName: "Bosch", Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}},
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		pickups:         map[int64]string{1: "12:15", 2: "15:00"},
		pickupChanged:   map[int64]bool{1: true},
		pickupRegular:   map[int64]string{1: "15:00"},
		pickupChangedAt: map[int64]time.Time{1: recorded},
	})

	require.Len(t, report.Rows, 2)
	assert.True(t, report.Rows[0].PickupChanged)
	assert.Equal(t, "12:15", report.Rows[0].Pickup)
	assert.Equal(t, "15:00", report.Rows[0].PickupRegular)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, recorded, *report.Rows[0].ReportedAt)
	// The unchanged child stays clean: no deviation, nothing to date.
	assert.False(t, report.Rows[1].PickupChanged)
	assert.Empty(t, report.Rows[1].PickupRegular)
	assert.Nil(t, report.Rows[1].ReportedAt)
	// A deviating pickup time does not take the child out of care.
	assert.True(t, report.Rows[0].StaysToday)
}

// TestBuildClassDayReportReportedAtFollowsStatus pins the precedence: the
// badge the reader sees is the status, so the timestamp next to it must date
// the status and not an unrelated pickup edit.
func TestBuildClassDayReportReportedAtFollowsStatus(t *testing.T) {
	t.Parallel()

	statusStamp := time.Date(2026, 8, 5, 7, 5, 0, 0, time.UTC)
	pickupStamp := time.Date(2026, 8, 4, 16, 0, 0, 0, time.UTC)
	rows := []DayRosterRow{{StudentID: 1, Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		statuses:         map[int64]string{1: statusDaySick},
		statusReportedAt: map[int64]time.Time{1: statusStamp},
		pickupChanged:    map[int64]bool{1: true},
		pickupChangedAt:  map[int64]time.Time{1: pickupStamp},
	})

	require.Len(t, report.Rows, 1)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, statusStamp, *report.Rows[0].ReportedAt)
}

// TestBuildClassDayReportListEntryHasNoDeviation guards the #2382 boundary:
// a child without any OGS record has no plan to deviate from, so no stray
// facts keyed on student id 0 may attach to it.
func TestBuildClassDayReportListEntryHasNoDeviation(t *testing.T) {
	t.Parallel()

	rows := []DayRosterRow{{ListEntry: true, ListEntryID: 42, LastName: "Cole"}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		pickupChanged: map[int64]bool{0: true},
		pickupRegular: map[int64]string{0: "15:00"},
	})

	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].PickupChanged)
	assert.Empty(t, report.Rows[0].PickupRegular)
}

func TestBuildClassDayReportListEntryProjection(t *testing.T) {
	t.Parallel()

	rows := []DayRosterRow{
		{
			StudentID:  21,
			FirstName:  "Mila",
			LastName:   "Anders",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Ganztag"},
			},
		},
		{
			ListEntry:   true,
			ListEntryID: 101,
			FirstName:   "Zoe",
			LastName:    "Aalders",
		},
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, newClassDayFacts())

	require.Len(t, report.Rows, 2)
	entryRow := report.Rows[1]
	require.True(t, entryRow.ListEntry)
	assert.Equal(t, int64(101), entryRow.ListEntryID)
	assert.Zero(t, entryRow.StudentID)
	assert.False(t, entryRow.StaysToday)
	// "Keine Betreuung" is the whole statement: no departure text at all —
	// especially not "Keine Angabe", which would read as a plan gap.
	assert.Empty(t, entryRow.Departure)

	assert.Equal(t, 2, report.Totals.Students)
	assert.Equal(t, 1, report.Totals.Staying)
	// The entry is NOT "geht nach Hause" — it lands in its own neutral
	// "Keine Betreuung" bucket (#2399 review).
	assert.Equal(t, 0, report.Totals.Leaving)
	assert.Equal(t, 1, report.Totals.ListEntries)
}
