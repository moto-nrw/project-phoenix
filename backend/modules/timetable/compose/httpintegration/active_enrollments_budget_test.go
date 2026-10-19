package httpintegration_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestActiveStudentEnrollmentsQueryBudgetStaysFlat pins the child's current
// activities (#4268): the note form's reference picker lists them, and the
// per-activity name is the obvious place for one lookup per row. Enrollments
// and their groups are two batched statements, however many a child has.
func TestActiveStudentEnrollmentsQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, activities, _, _, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Aktiv", "Budget", "2b")

	enroll := func(from, count int) {
		t.Helper()
		for i := from; i < from+count; i++ {
			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("active-enrollment-budget-%d", i))
			require.NoError(t, activities.EnrollStudent(ctx, group.ID, student.ID))
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	reads := func(expected int) []string {
		t.Helper()
		counter.Reset()
		byStudent, readErr := activities.GetActiveStudentEnrollmentsByStudentIDs(
			ctx, []int64{student.ID}, calendar.TodayDate())
		require.NoError(t, readErr)
		require.Len(t, byStudent[student.ID], expected)
		for _, group := range byStudent[student.ID] {
			require.NotEmpty(t, group.Name, "the budget only counts if the names are resolved")
		}
		// The tenant transaction frame is the runtime's, not this read's.
		return counter.Matching(func(sqlLower string) bool {
			return strings.Contains(sqlLower, "activities.")
		})
	}

	enroll(0, 3)
	small := reads(3)
	enroll(3, 5)
	large := reads(8)
	require.Len(t, large, len(small), "the read grew with the number of activities")
	testpkg.AssertQueryBudget(t, "modules.timetable.active_enrollments.list", large)
}
