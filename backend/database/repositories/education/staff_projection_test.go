package education_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two reads below no longer join users.teachers / users.staff; School
// Membership resolves them for the repository (#2667).

func TestEducationGroupSupervisionResolvesTeachersToStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)
	today := timezone.TodayDate()

	group := testpkg.CreateTestEducationGroup(t, db, "Zuordnung Gruppe")
	teacher := testpkg.CreateTestTeacher(t, db, "Zuordnung", "Lehrkraft")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	goneTeacher := testpkg.CreateTestTeacher(t, db, "Weg", "Lehrkraft")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, goneTeacher.ID)
	require.NoError(t, factory.Teacher.Delete(ctx, goneTeacher.ID))
	goneStaffTeacher := testpkg.CreateTestTeacher(t, db, "Weg", "Anstellung")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, goneStaffTeacher.ID)
	require.NoError(t, factory.Staff.Delete(ctx, goneStaffTeacher.StaffID))

	substitute := testpkg.CreateTestStaff(t, db, "Zuordnung", "Vertretung")
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, today.AddDays(-1), today.AddDays(1))
	goneSubstitute := testpkg.CreateTestStaff(t, db, "Weg", "Vertretung")
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, goneSubstitute.ID, today.AddDays(-1), today.AddDays(1))
	require.NoError(t, factory.Staff.Delete(ctx, goneSubstitute.ID))

	pairs, err := factory.Group.ListStaffIDsByEducationGroupIDs(ctx, []int64{group.ID}, today)
	require.NoError(t, err)

	staffIDs := make([]int64, 0, len(pairs))
	for _, pair := range pairs {
		assert.Equal(t, group.ID, pair.GroupID)
		staffIDs = append(staffIDs, pair.StaffID)
	}
	assert.Contains(t, staffIDs, teacher.StaffID, "the assigned teacher resolves to their staff member")
	assert.Contains(t, staffIDs, substitute.ID)
	assert.NotContains(t, staffIDs, goneTeacher.StaffID, "an offboarded teacher profile drops out")
	assert.NotContains(t, staffIDs, goneStaffTeacher.StaffID, "an offboarded staff member drops out")
	assert.NotContains(t, staffIDs, goneSubstitute.ID, "an offboarded substitute drops out")
	// Assignments come before substitutions, as the two queries did.
	require.NotEmpty(t, staffIDs)
	assert.Equal(t, teacher.StaffID, staffIDs[0])

	empty, err := factory.Group.ListStaffIDsByEducationGroupIDs(ctx, nil, today)
	require.NoError(t, err)
	assert.Empty(t, empty, "an empty request must not resolve to everybody")
}

func TestGroupSubstitutionsCarryTheirStaffMembers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)
	today := timezone.TodayDate()

	group := testpkg.CreateTestEducationGroup(t, db, "Vertretung Gruppe")
	regular := testpkg.CreateTestStaff(t, db, "Vertretung", "Stamm")
	substitute := testpkg.CreateTestStaff(t, db, "Vertretung", "Ersatz")
	regularID := regular.ID
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regularID, substitute.ID, today.AddDays(-1), today.AddDays(1))
	// History keeps resolving: an offboarded colleague is still named on the
	// substitution they were part of.
	require.NoError(t, factory.Staff.Delete(ctx, regular.ID))

	rows, err := factory.GroupSubstitution.FindActiveBySubstituteWithRelations(ctx, substitute.ID, today)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].SubstituteStaff)
	assert.Equal(t, substitute.PersonID, rows[0].SubstituteStaff.PersonID)
	require.NotNil(t, rows[0].RegularStaff, "a soft-deleted regular staff member still resolves")
	assert.Equal(t, regular.PersonID, rows[0].RegularStaff.PersonID)

	// The unfiltered listing takes the same path; this test owns its tenant,
	// so the substitution above is the only row it can see.
	listed, err := factory.GroupSubstitution.ListWithRelations(ctx, nil)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].SubstituteStaff)
	require.NotNil(t, listed[0].RegularStaff)
}
