package activities_test

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisorPlannedFindByStaffIDQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	repo := plannedSupervisorRepository(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Budget")

	add := func(from, to int) {
		for i := from; i < to; i++ {
			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("supervisor-budget-%d", i))
			createSupervisor(t, db, staff.ID, group.ID, i == 0)
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueries(t, db)
	run := func() []string {
		counter.Reset()
		supervisors, err := repo.FindByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		require.NotEmpty(t, supervisors)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "repositories.activities.supervisors_by_staff.reads", large)
}
