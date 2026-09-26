package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func TestClassDayCancellationsUseCareDayService(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{CareDays: fakeCareDays{statuses: map[int64]ports.CareDay{
		1: ports.CareDayCancelled,
		2: ports.CareDayScheduled,
		3: ports.CareDayNotScheduled,
	}}})

	facts := newClassDayFacts()
	cancelled, err := svc.classDayCancellations(context.Background(), []int64{1, 2, 3}, timezone.NewDate(2026, 8, 5), &facts)

	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{1: true}, cancelled)
	// "An dem Tag nicht gebucht" is not an absence, but the child must not
	// be listed as staying either — surfaced separately.
	assert.Equal(t, map[int64]bool{3: true}, facts.notScheduled)
}

func TestClassDayCancellationUsesArrivalExceptionReportTime(t *testing.T) {
	t.Parallel()

	reportedAt := time.Date(2026, 8, 5, 7, 24, 0, 0, time.UTC)
	svc := newDayReports(ClassDayDependencies{
		Times: fakeDayTimes{arrivals: map[int64]EffectiveArrival{
			1: {IsException: true, ChangedAt: &reportedAt},
		}},
		CareDays: fakeCareDays{statuses: map[int64]ports.CareDay{
			1: ports.CareDayCancelled,
		}},
	})
	facts := newClassDayFacts()

	require.NoError(t, svc.classDayEffectiveTimes(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5), &facts))
	cancelled, err := svc.classDayCancellations(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5), &facts)
	require.NoError(t, err)
	for studentID := range cancelled {
		facts.statuses[studentID] = studentStatusDayCancelled
		if stamp, ok := facts.arrivalCancelledAt[studentID]; ok {
			facts.statusReportedAt[studentID] = stamp
		}
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", []DayRosterRow{{
		StudentID: 1, Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}},
	}}, facts)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, reportedAt, *report.Rows[0].ReportedAt)
}

func TestClassDayStatusPrecedenceSickWins(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{
		{StudentID: 1, Status: statusDayExcused},
		{StudentID: 1, Status: statusDaySick},
		{StudentID: 2, Status: statusDayClassTrip},
		{StudentID: 2, Status: statusDayExcused},
	}}})

	statuses, _, err := svc.classDayStatuses(context.Background(), []int64{1, 2}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, "sick", statuses[1])
	assert.Equal(t, "class_trip", statuses[2])
}

func TestClassDayStatusUnknownValueStillCounts(t *testing.T) {
	t.Parallel()

	// The status set is an extension point: a value the precedence map does
	// not know yet still means "reported for the day" and must survive —
	// silently dropping it would put a reported-absent child under "bleibt
	// in der Betreuung". Known statuses keep precedence over unknown ones.
	svc := newDayReports(ClassDayDependencies{StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{
		{StudentID: 1, Status: "quarantine"},
		{StudentID: 2, Status: "quarantine"},
		{StudentID: 2, Status: statusDayExcused},
	}}})

	statuses, _, err := svc.classDayStatuses(context.Background(), []int64{1, 2}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, "quarantine", statuses[1])
	assert.Equal(t, "excused", statuses[2])
}

func TestClassDayDepartureRendersSingleDay(t *testing.T) {
	t.Parallel()

	pickupAndBus := peopledirectory.AllowedDepartureModes{
		"fri": {peopledirectory.DepartureBus, peopledirectory.DeparturePickup},
	}
	assert.Equal(t, "Bus, Abholung", classDayDeparture(weekdayDepartureModes(pickupAndBus, nil, "fri"), "fri", nil, nil))
	// A day without any allowed mode is UNKNOWN — the empty string lets the
	// report fall back to the form answer / "Keine Angabe", never to a
	// fabricated "Geht alleine".
	assert.Equal(t, "", classDayDeparture(weekdayDepartureModes(pickupAndBus, nil, "mon"), "mon", nil, nil))

	// The parent-written note is NOT rendered here: it is free text that
	// routinely carries a guardian's name and phone number, which this view
	// promises never to show. The day roster no longer carries the note at
	// all, so the accompanied departure can only name linked companions.
	accompanied := peopledirectory.AllowedDepartureModes{
		"fri": {peopledirectory.DepartureAccompanied},
	}
	assert.Equal(t, "Mit anderem Kind", classDayDeparture(weekdayDepartureModes(accompanied, nil, "fri"), "fri", nil, nil))
}

func TestClassDayDepartureNamesOnlyCompanionsOnTheSheet(t *testing.T) {
	t.Parallel()

	// A Laufgemeinschaft may pair children of different classes, and the
	// class-day sheet is scoped to one class: a companion from 4c must not
	// surface on the 1a sheet, a classmate already listed there may.
	const classmateID, foreignID = int64(11), int64(99)
	accompanied := weekdayDepartureModes(peopledirectory.AllowedDepartureModes{
		"fri": {peopledirectory.DepartureAccompanied},
	}, nil, "fri")
	onSheet := map[int64]bool{classmateID: true}

	classmate := []peopledirectory.CompanionLink{
		{CompanionStudentID: classmateID, FirstName: "Tom", LastName: "Berger", Weekdays: []string{"fri"}},
	}
	foreign := []peopledirectory.CompanionLink{
		{CompanionStudentID: foreignID, FirstName: "Max", LastName: "Mustermann", Weekdays: []string{"fri"}},
	}

	// The weekday suffix is FormatCompanionLinks' doing: a link that does not
	// cover all five days names them, so a Friday-only arrangement cannot be
	// misread as a standing one.
	assert.Equal(t, "Mit anderem Kind (Tom Berger (Fr))", classDayDeparture(accompanied, "fri", classmate, onSheet))
	assert.Equal(t, "Mit anderem Kind", classDayDeparture(accompanied, "fri", foreign, onSheet))
	// Mixed links keep the classmate and drop the outsider rather than
	// falling back to naming nobody.
	assert.Equal(t, "Mit anderem Kind (Tom Berger (Fr))",
		classDayDeparture(accompanied, "fri", append(append([]peopledirectory.CompanionLink{}, foreign...), classmate...), onSheet))
}

// clockTime builds a wall-clock TIME value the way the driver scans one.
func clockTime(hour, minute int) *time.Time {
	value := time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC)
	return &value
}

// TestApplyClassDayPickupDeviation pins what counts as "heute anders als
// sonst" for a Lehrkraft (#2294). The distinction matters in the product: a
// block that also announces unchanged times trains the reader to skip it.
func TestApplyClassDayPickupDeviation(t *testing.T) {
	t.Parallel()

	recorded := time.Date(2026, 8, 24, 9, 12, 0, 0, time.UTC)

	tests := []struct {
		name        string
		entry       EffectivePickup
		wantPickup  string
		wantChanged bool
		wantRegular string
		wantStamp   bool
	}{
		{
			name:       "plan time without exception is no deviation",
			entry:      EffectivePickup{PickupTime: clockTime(15, 0), RegularPickupTime: clockTime(15, 0)},
			wantPickup: "15:00",
		},
		{
			name: "earlier pickup names the regular time it replaces",
			entry: EffectivePickup{
				PickupTime: clockTime(12, 15), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "12:15", wantChanged: true, wantRegular: "15:00", wantStamp: true,
		},
		{
			name: "later pickup is a deviation too",
			entry: EffectivePickup{
				PickupTime: clockTime(16, 30), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "16:30", wantChanged: true, wantRegular: "15:00", wantStamp: true,
		},
		{
			// A parent re-entering the time the plan already holds must not
			// reach the Lehrkraft as a change.
			name: "exception repeating the plan time is not a deviation",
			entry: EffectivePickup{
				PickupTime: clockTime(15, 0), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "15:00", wantStamp: true,
		},
		{
			// The child is not normally in care that weekday; there is no
			// "sonst" to name, but the time itself is news.
			name: "pickup on a day without a plan time is a deviation without a regular time",
			entry: EffectivePickup{
				PickupTime: clockTime(14, 0), IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "14:00", wantChanged: true, wantStamp: true,
		},
		{
			// "Kommt heute nicht" travels as a status, never as a changed
			// pickup time — otherwise the row claims a pickup that is not
			// happening.
			name: "timeless exception records only when it became known",
			entry: EffectivePickup{
				RegularPickupTime: clockTime(15, 0), IsException: true, ChangedAt: &recorded,
			},
			wantStamp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := newClassDayFacts()

			applyClassDayPickup(&facts, 7, tc.entry)

			assert.Equal(t, tc.wantPickup, facts.pickups[7])
			assert.Equal(t, tc.wantChanged, facts.pickupChanged[7])
			assert.Equal(t, tc.wantRegular, facts.pickupRegular[7])
			_, hasStamp := facts.pickupChangedAt[7]
			assert.Equal(t, tc.wantStamp, hasStamp)
		})
	}
}

// TestClassDayStatusesReportTime keeps the stamp attached to the winning
// status: sick outranks excused, and so does its report time.
func TestClassDayStatusesReportTime(t *testing.T) {
	t.Parallel()

	sickAt := time.Date(2026, 8, 5, 11, 24, 0, 0, time.UTC)
	excusedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	svc := newDayReports(ClassDayDependencies{StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{
		{StudentID: 1, Status: statusDayExcused, ReportedAt: excusedAt},
		{StudentID: 1, Status: statusDaySick, ReportedAt: sickAt},
	}}})

	statuses, stamps, err := svc.classDayStatuses(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, "sick", statuses[1])
	assert.Equal(t, sickAt, stamps[1])
}
