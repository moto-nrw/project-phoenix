package staffgroups_test

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestStaffGroupReadsQueryBudget guards the three staff-keyed reads against
// per-row lookups: each resolves its ids at the owner and loads the groups in
// one batch, so the statement count stays flat as rows grow.
func TestStaffGroupReadsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	query := buildStaffGroups(t, db)
	ctx := testpkg.Ctx(t)
	counter := testpkg.CaptureQueries(t, db)
	teacher := testpkg.CreateTestTeacher(t, db, "Budget", "Teacher")
	substitute := testpkg.CreateTestStaff(t, db, "Budget", "Substitute")
	count := 0

	add := func(n int) {
		for range n {
			assigned := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Budget assigned %d", count))
			testpkg.CreateTestGroupTeacher(t, db, assigned.ID, teacher.ID)
			substituted := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Budget substituted %d", count))
			testpkg.CreateTestGroupSubstitution(t, db, substituted.ID, nil, substitute.ID, substitutionDay, substitutionDay)
			testpkg.CreateTestClassTeacher(t, db, substitute.ID, fmt.Sprintf("%da", count+1))
			count++
		}
	}
	read := func() int {
		counter.Reset()
		groups, err := query.ListGroupsByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		require.Len(t, groups, count)
		substitutions, err := query.ListSubstitutedGroups(ctx, substitute.ID, substitutionDay.String())
		require.NoError(t, err)
		require.Len(t, substitutions, count)
		classes, err := query.ListSchoolClassesByStaff(ctx, substitute.ID)
		require.NoError(t, err)
		require.Len(t, classes, count)
		return counter.Total()
	}

	add(1)
	small := read()
	add(3)
	require.Equal(t, small, read(), "statement count must not grow with the rows")
	testpkg.AssertQueryBudget(t, "modules.schoolstructure.staff_group_reads", counter.Queries())
}
