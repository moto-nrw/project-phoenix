package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// tenantCtx returns a context carrying tenant 1, matching how the parent
// service scopes calls before invoking the repo.
func tenantCtx() context.Context {
	return tenant.WithTenantID(context.Background(), 1)
}

func newNote(studentID, accountID int64, body string) *usersModels.StudentParentNote {
	n := &usersModels.StudentParentNote{
		StudentID:         studentID,
		GuardianAccountID: accountID,
		Body:              body,
	}
	n.SetTenantID(1)
	return n
}

func TestStudentParentNoteRepository_CreateAndListNewestFirst(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewStudentParentNoteRepository(db)
	ctx := tenantCtx()

	for _, body := range []string{"erste", "zweite", "dritte"} {
		require.NoError(t, repo.Create(ctx, newNote(chain.StudentID, chain.AccountID, body)))
	}

	notes, err := repo.ListByStudent(ctx, chain.StudentID, 0)
	require.NoError(t, err)
	require.Len(t, notes, 3)
	assert.Equal(t, "dritte", notes[0].Body, "newest first")
	assert.Equal(t, "erste", notes[2].Body)
}

func TestStudentParentNoteRepository_ListRespectsLimit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewStudentParentNoteRepository(db)
	ctx := tenantCtx()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(ctx, newNote(chain.StudentID, chain.AccountID, "n")))
	}

	notes, err := repo.ListByStudent(ctx, chain.StudentID, 2)
	require.NoError(t, err)
	assert.Len(t, notes, 2)
}

func TestStudentParentNoteRepository_CountByStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewStudentParentNoteRepository(db)
	ctx := tenantCtx()

	count, err := repo.CountByStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	require.NoError(t, repo.Create(ctx, newNote(chain.StudentID, chain.AccountID, "x")))
	require.NoError(t, repo.Create(ctx, newNote(chain.StudentID, chain.AccountID, "y")))

	count, err = repo.CountByStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestStudentParentNoteRepository_ListEmptyForUnknownStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := usersRepo.NewStudentParentNoteRepository(db)
	notes, err := repo.ListByStudent(tenantCtx(), 987654321, 5)
	require.NoError(t, err)
	assert.Empty(t, notes)
}
