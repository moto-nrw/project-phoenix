package users_test

import (
	"testing"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentGuardianRepositoryFindByGuardianProfileIDs(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestParentGuardianChain(t, db)
	second := testpkg.CreateTestParentGuardianChain(t, db)
	repo := usersRepo.NewStudentGuardianRepository(db)

	rows, err := repo.FindByGuardianProfileIDs(ctx, []int64{second.GuardianProfileID, first.GuardianProfileID})
	require.NoError(t, err)
	found := make(map[int64]bool, len(rows))
	for _, row := range rows {
		found[row.GuardianProfileID] = true
	}
	assert.True(t, found[first.GuardianProfileID])
	assert.True(t, found[second.GuardianProfileID])

	rows, err = repo.FindByGuardianProfileIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
