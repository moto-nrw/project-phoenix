package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/require"
)

// ClassArrivalTimeRow is one class's Unterrichtsschluss per weekday. The
// table education.class_arrival_times belongs to the Timetable owner; the
// fixtures write and read it through that owner's contract (#3556), which
// the caller composes (modules/timetable/compose.NewClassArrivals).
type ClassArrivalTimeRow = timetable.ClassArrivalPlan

// CreateTestClassArrivalTime creates one tenant-owned dismissal timetable.
func CreateTestClassArrivalTime(tb testing.TB, arrivals timetable.ClassArrivals, class string, times map[string]string) *ClassArrivalTimeRow {
	tb.Helper()
	return UpsertTestClassArrivalTime(tb, arrivals, class, times)
}

// UpsertTestClassArrivalTime stores the weekday map of one class, replacing
// the class's row when it already has one, like the maintenance screen.
func UpsertTestClassArrivalTime(tb testing.TB, arrivals timetable.ClassArrivals, class string, times map[string]string) *ClassArrivalTimeRow {
	tb.Helper()
	plan, err := arrivals.UpsertClassArrivalPlan(Ctx(tb), timetable.ClassArrivalPlanInput{SchoolClass: class, ArrivalTimes: times})
	require.NoError(tb, err)
	return &plan
}

// ClassArrivalTimesOf returns the stored rows of one class, matched on the
// normalized class like every school_class join.
func ClassArrivalTimesOf(tb testing.TB, arrivals timetable.ClassArrivals, class string) []ClassArrivalTimeRow {
	tb.Helper()
	plans, err := arrivals.ListClassArrivalPlans(Ctx(tb), []string{class})
	require.NoError(tb, err)
	return plans
}
