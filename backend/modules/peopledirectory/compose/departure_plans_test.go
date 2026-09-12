package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeparturePlansReadCanonicalModesForRequestedChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Departure", "Baseline", "1a")
	unset := testpkg.CreateTestStudent(t, db, "Unset", "Departure", "1a")
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, student.ID, peopledirectory.EnrollmentProfilePatch{
		DepartureSet:          true,
		AllowedDepartureModes: map[string][]string{"mon": {"pickup", "alone"}, "fri": {"bus"}},
	}))
	plans, err := module.ListStudentDepartureModes(ctx, []int64{student.ID, unset.ID, student.ID})
	require.NoError(t, err)
	require.Equal(t, map[string][]string{"mon": {"alone", "pickup"}, "fri": {"bus"}}, plans[student.ID])
	require.Contains(t, plans, unset.ID)
	require.Empty(t, plans[unset.ID])

	otherCtx, _ := otherTenantContext(t, db)
	plans, err = module.ListStudentDepartureModes(otherCtx, []int64{student.ID})
	require.NoError(t, err)
	require.Empty(t, plans)
	plans, err = module.ListStudentDepartureModes(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, plans)
}
