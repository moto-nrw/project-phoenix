package auth_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// caregiverChainQuery mirrors the seam the account listings type-assert on.
// The auth repository no longer joins users.staff / users.teachers itself;
// School Membership answers the chain (#2667).
type caregiverChainQuery interface {
	CaregiverChainByPersonIDs(context.Context, []int64) (map[int64]authModels.CaregiverChain, error)
}

func TestCaregiverChainsComeFromSchoolMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)

	chains, ok := factory.AccountTenant.(caregiverChainQuery)
	require.True(t, ok, "the account listings still resolve caregiver chains")

	teacher := testpkg.CreateTestTeacher(t, db, "Kette", "Lehrkraft")
	plainStaff := testpkg.CreateTestStaff(t, db, "Kette", "Ohne Profil")
	stranger := testpkg.CreateTestPerson(t, db, "Kette", "Ohne Anstellung")

	staffRow, err := factory.Staff.FindByID(ctx, teacher.StaffID)
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

	require.NoError(t, factory.Staff.Delete(ctx, plainStaff.ID))
	result, err = chains.CaregiverChainByPersonIDs(ctx, []int64{plainStaff.PersonID})
	require.NoError(t, err)
	assert.Empty(t, result, "an offboarded staff member is no caregiver")
}
