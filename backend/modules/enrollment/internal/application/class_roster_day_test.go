package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestClassDayPhasesReturnsEveryActiveCoveringPhase(t *testing.T) {
	t.Parallel()

	svc := NewReports(ReportDependencies{Phases: &fakeClassDayPhaseRepo{phases: []*enrollment.Phase{
		{ID: 1, Name: "Altjahr", ServiceStartDate: enrollment.Date(calendar.NewDate(2025, 8, 1)), ServiceEndDate: enrollment.Date(calendar.NewDate(2026, 7, 31))},
		{ID: 2, Name: "Schuljahr", IsActive: true, ServiceStartDate: enrollment.Date(calendar.NewDate(2026, 8, 1)), ServiceEndDate: enrollment.Date(calendar.NewDate(2027, 7, 31))},
		{ID: 3, Name: "Ferien", IsActive: true, ServiceStartDate: enrollment.Date(calendar.NewDate(2026, 8, 3)), ServiceEndDate: enrollment.Date(calendar.NewDate(2026, 8, 14))},
		{ID: 4, Name: "InaktivAlt", ServiceStartDate: enrollment.Date(calendar.NewDate(2026, 8, 1)), ServiceEndDate: enrollment.Date(calendar.NewDate(2027, 7, 31))},
	}}})

	phases, err := svc.classDayPhases(context.Background(), calendar.NewDate(2026, 8, 5))

	require.NoError(t, err)
	// EVERY covering ACTIVE phase, not one "best": a child enrolled only in
	// the Schuljahr phase must not vanish because a Ferien phase also covers
	// the date. Latest start first. The deactivated phase whose window still
	// covers the date is excluded — !IsActive is a hard reject on every
	// enrollment path, and a stale approval must not put a child into
	// "Bleiben in der Betreuung".
	require.Len(t, phases, 2)
	assert.Equal(t, []int64{3, 2}, []int64{phases[0].id, phases[1].id})
}

func TestClassDayPhasesNoneCovering(t *testing.T) {
	t.Parallel()

	svc := NewReports(ReportDependencies{Phases: &fakeClassDayPhaseRepo{phases: []*enrollment.Phase{
		{ID: 1, ServiceStartDate: enrollment.Date(calendar.NewDate(2025, 8, 1)), ServiceEndDate: enrollment.Date(calendar.NewDate(2026, 7, 31))},
	}}})

	phases, err := svc.classDayPhases(context.Background(), calendar.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Empty(t, phases)
}

func TestMergeClassDayRostersUnionsRegistrations(t *testing.T) {
	t.Parallel()

	schoolYear := []enrollment.ClassRosterRow{
		{StudentID: 1, LastName: "Anders", Registered: true, EnrollmentSummary: "Ganztag", OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}, PickupByDay: map[string]string{"wed": "16:00"}},
		{StudentID: 2, LastName: "Becker", Registered: false, EnrollmentSummary: "Keine Anmeldung"},
	}
	ferien := []enrollment.ClassRosterRow{
		{StudentID: 1, LastName: "Anders", Registered: false, EnrollmentSummary: "Keine Anmeldung"},
		{StudentID: 2, LastName: "Becker", Registered: true, EnrollmentSummary: "Ferienbetreuung", OfferingsByDay: map[string][]string{"wed": {"Ferienbetreuung"}}},
	}

	merged := mergeClassDayRosters([][]enrollment.ClassRosterRow{schoolYear, ferien})

	require.Len(t, merged, 2)
	// Registered in EITHER covering phase counts.
	assert.True(t, merged[0].Registered)
	assert.Equal(t, []string{"Ganztag"}, merged[0].OfferingsByDay["wed"])
	assert.Equal(t, "16:00", merged[0].PickupByDay["wed"])
	assert.True(t, merged[1].Registered)
	assert.Equal(t, "Ferienbetreuung", merged[1].EnrollmentSummary)
	assert.Equal(t, []string{"Ferienbetreuung"}, merged[1].OfferingsByDay["wed"])
}

// TestClassDayWithoutPhaseListsFullClass is the roster half of the class day
// without a covering phase: the full class is listed, nobody registered, and
// no phase is named. The day view's statuses and totals are its own tests.
func TestClassDayWithoutPhaseListsFullClass(t *testing.T) {
	t.Parallel()

	svc := classRosterTestService(
		[]*RosterStudent{
			{ID: 1, PersonID: 11, SchoolClass: "1a"},
			{ID: 2, PersonID: 12, SchoolClass: "1a"},
		},
		map[int64]*RosterPerson{
			11: {FirstName: "Mila", LastName: "Anders"},
			12: {FirstName: "Finn", LastName: "Becker"},
		},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	svc.deps.Phases = &fakeClassDayPhaseRepo{}

	roster, err := svc.ClassRosterDay(context.Background(), "1a", calendar.NewDate(2026, 8, 5))

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	assert.Empty(t, roster.PhaseNames)
	assert.False(t, roster.Rows[0].Registered)
	assert.Equal(t, "Anders", roster.Rows[0].LastName)
	assert.Equal(t, "Becker", roster.Rows[1].LastName)
}
