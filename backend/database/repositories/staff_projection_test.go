package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaregiverChainsComeFromSchoolMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)

	chains := repositories.CaregiverChainsForTests(membership)

	teacher := testpkg.CreateTestTeacher(t, db, "Kette", "Lehrkraft")
	plainStaff := testpkg.CreateTestStaff(t, db, "Kette", "Ohne Profil")
	stranger := testpkg.CreateTestPerson(t, db, "Kette", "Ohne Anstellung")

	staffRow, err := membership.FindStaff(ctx, teacher.StaffID)
	require.NoError(t, err)

	result, err := chains.CaregiverChainByPersonIDs(ctx, []int64{staffRow.PersonID, plainStaff.PersonID, stranger.ID})
	require.NoError(t, err)

	chain, found := result[staffRow.PersonID]
	require.True(t, found)
	assert.Equal(t, teacher.StaffID, chain.StaffID)
	assert.Equal(t, teacher.ID, chain.TeacherID)

	plain, found := result[plainStaff.PersonID]
	require.True(t, found)
	assert.Equal(t, plainStaff.ID, plain.StaffID)
	assert.Zero(t, plain.TeacherID, "a staff member without a teacher profile carries no teacher")

	_, found = result[stranger.ID]
	assert.False(t, found, "a person without a staff record has no caregiver chain")

	require.NoError(t, membership.DeleteStaff(ctx, plainStaff.ID))
	result, err = chains.CaregiverChainByPersonIDs(ctx, []int64{plainStaff.PersonID})
	require.NoError(t, err)
	assert.Empty(t, result, "an offboarded staff member is no caregiver")
}
