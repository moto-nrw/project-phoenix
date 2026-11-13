package studentintegration_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeparturePlansReadCanonicalModesForRequestedChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
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

// #6990: a row whose mode set is empty answers from its legacy columns, as
// every other plan read does. Otherwise the request review read "Geht alleine"
// where the family saw "Fährt Bus" and flagged a change nobody made.
func TestDeparturePlansResolveLegacyColumnsLikeEveryPlanRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Legacy", "Departure", "1a")
	_, err := db.NewRaw(`UPDATE users.student_care_profiles SET allowed_departure_modes = '{}'::jsonb, departure_days = '{}'::jsonb,
		bus_days = '{"wed": true}'::jsonb, pickup_days = '{}'::jsonb
		WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`, student.ID).Exec(ctx)
	require.NoError(t, err)
	plans, err := module.ListStudentDepartureModes(ctx, []int64{student.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"bus"}, plans[student.ID]["wed"])
}
