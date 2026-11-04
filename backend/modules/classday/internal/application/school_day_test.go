package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func schoolDayTestRoster() *DayRoster {
	wed := map[string][]string{"wed": {"Ganztag"}}
	return &DayRoster{
		PhaseNames: []string{"Schuljahr"},
		Rows: []DayRosterRow{
			{StudentID: 1, SchoolClass: "1a", FirstName: "Anna", LastName: "Arndt", Registered: true, OfferingsByDay: wed},
			{StudentID: 2, SchoolClass: "1A", FirstName: "Ben", LastName: "Busch", Registered: true, OfferingsByDay: wed},
			{SchoolClass: "1a", ListEntry: true, ListEntryID: 7, FirstName: "Lea", LastName: "Lang"},
			{StudentID: 3, SchoolClass: "2b", FirstName: "Carl", LastName: "Claus"},
			{StudentID: 4, SchoolClass: "2b", FirstName: "Dora", LastName: "Dorn", Registered: true, OfferingsByDay: wed},
			{StudentID: 5, SchoolClass: "3c", FirstName: "Eva", LastName: "Eck", Registered: true, OfferingsByDay: wed},
		},
	}
}

// TestSchoolDayCountsEveryClassFromOneRoster pins the school-wide overview
// (#4802): the class view's rules per class, summed for the school, from one
// roster; a reported absence counts as missing in the morning, classes the
// caller does not see are left out, an assigned class without children stays
// with zero. The stored morning marks are read through Student Presence,
// which this package's tests may not import; the HTTP test covers that read.
func TestSchoolDayCountsEveryClassFromOneRoster(t *testing.T) {
	t.Parallel()

	regular := time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
	early := time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC)
	log := &fakeClassDayAccessLog{}
	svc := NewClassDay(ClassDayDependencies{
		Caller:  fakeClassDayCaller{classes: []string{"1a", "2b", "4d"}},
		Rosters: fakeDayRosters{roster: schoolDayTestRoster()},
		StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{
			{StudentID: 2, Status: statusDaySick},
			{StudentID: 4, Status: statusDayClassTrip},
			{StudentID: 5, Status: statusDaySick},
		}},
		Times: fakeDayTimes{pickups: map[int64]EffectivePickup{
			3: {PickupTime: &early, RegularPickupTime: &regular, IsException: true},
		}},
		CareDays:  fakeCareDays{},
		AccessLog: log,
	}).(classday.SchoolDay)
	actor := classday.Actor{AccountID: 42, Roles: "schulverwaltung"}

	report, err := svc.SchoolDayReport(context.Background(), "2026-08-05", actor)

	require.NoError(t, err)
	assert.True(t, report.SchoolDay)
	assert.True(t, report.EnrollmentKnown)
	require.Len(t, report.Classes, 3)
	assert.Equal(t, []string{"1a", "2b", "4d"}, []string{report.Classes[0].SchoolClass, report.Classes[1].SchoolClass, report.Classes[2].SchoolClass})
	assert.Equal(t, classday.SchoolDayTotals{DayTotals: classday.DayTotals{Students: 3, Absent: 1, Staying: 1, ListEntries: 1}, Missing: 1, Sick: 1, Changes: 1}, report.Classes[0].Totals)
	assert.Equal(t, classday.SchoolDayTotals{DayTotals: classday.DayTotals{Students: 2, Absent: 1, Leaving: 1}, Missing: 1, ClassTrip: 1, Changes: 2}, report.Classes[1].Totals)
	assert.Equal(t, classday.SchoolDayTotals{}, report.Classes[2].Totals)
	assert.Empty(t, report.Classes[2].Children)
	assert.Equal(t, classday.SchoolDayTotals{DayTotals: classday.DayTotals{Students: 5, Absent: 2, Staying: 1, Leaving: 1, ListEntries: 1}, Missing: 2, Sick: 1, ClassTrip: 1, Changes: 3}, report.Totals)

	anna := report.Classes[0].Children[0]
	assert.Equal(t, classday.SchoolDayChild{StudentID: 1, FirstName: "Anna", LastName: "Arndt", AfterSchool: classday.SchoolDayStays}, anna)
	lea := report.Classes[0].Children[2]
	assert.Equal(t, classday.SchoolDayChild{ListEntry: true, ListEntryID: 7, FirstName: "Lea", LastName: "Lang"}, lea)
	carl := report.Classes[1].Children[0]
	assert.Equal(t, classday.SchoolDayChild{StudentID: 3, FirstName: "Carl", LastName: "Claus", AfterSchool: classday.SchoolDayGoesHome, Changed: true}, carl)

	// The access is logged once per actor, date and day of access.
	_, err = svc.SchoolDayReport(context.Background(), "2026-08-05", actor)
	require.NoError(t, err)
	require.Len(t, log.schoolViews, 1)
	assert.Equal(t, "school_day", log.schoolViews[0].Metadata["report"])
	assert.Equal(t, []string{"1a", "2b", "4d"}, log.schoolViews[0].Metadata["school_classes"])
	assert.Equal(t, 5, log.schoolViews[0].Metadata["student_count"])
	assert.Equal(t, "schulverwaltung", log.schoolViews[0].ActorRole)
	assert.Empty(t, log.views)
}

// TestSchoolDayWithoutSchoolCountsOnlyChildren mirrors the class view on a
// weekend: only the Klassenverband count, no status reaches a child.
func TestSchoolDayWithoutSchoolCountsOnlyChildren(t *testing.T) {
	t.Parallel()

	svc := NewClassDay(ClassDayDependencies{
		Caller:     fakeClassDayCaller{classes: []string{"1a", "2b"}},
		Rosters:    fakeDayRosters{roster: schoolDayTestRoster()},
		StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{{StudentID: 2, Status: statusDaySick}}},
		Times:      fakeDayTimes{},
		CareDays:   fakeCareDays{},
	}).(classday.SchoolDay)
	ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return false, nil })

	report, err := svc.SchoolDayReport(ctx, "2026-08-08", classday.Actor{AccountID: 42, Roles: "schulverwaltung"})

	require.NoError(t, err)
	assert.False(t, report.SchoolDay)
	assert.Equal(t, classday.SchoolDayTotals{DayTotals: classday.DayTotals{Students: 5}}, report.Totals)
	assert.Empty(t, report.Classes[0].Children[1].Status)
}

// TestSchoolDayWithoutPhaseSplitsNobody keeps the stays/leaves split unknown
// without a covering enrollment phase, like the class view.
func TestSchoolDayWithoutPhaseSplitsNobody(t *testing.T) {
	t.Parallel()

	roster := schoolDayTestRoster()
	roster.PhaseNames = nil
	svc := NewClassDay(ClassDayDependencies{
		Caller:     fakeClassDayCaller{classes: []string{"1a", "2b"}},
		Rosters:    fakeDayRosters{roster: roster},
		StatusDays: fakeClassDayStatusDays{},
		Times:      fakeDayTimes{},
		CareDays:   fakeCareDays{},
	}).(classday.SchoolDay)

	report, err := svc.SchoolDayReport(context.Background(), "2026-08-05", classday.Actor{AccountID: 42, Roles: "schulverwaltung"})

	require.NoError(t, err)
	assert.False(t, report.EnrollmentKnown)
	assert.Zero(t, report.Totals.Staying)
	assert.Zero(t, report.Totals.Leaving)
	assert.Equal(t, 5, report.Totals.Students)
}
