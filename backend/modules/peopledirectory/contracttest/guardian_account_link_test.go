package contracttest_test

import (
	"context"
	"errors"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianAccountLink_PreservesExistingChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := peopleCompose.NewRetainedGuardianProfileRepository(db, testpkg.StudentDirectory{}, testpkg.GuardianLinks{})
	account := testpkg.CreateTestAccount(t, db, "linked-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "old-contact")
	next := testpkg.CreateTestGuardianProfile(t, db, "new-contact")
	child := testpkg.CreateTestStudent(t, db, "Existing", "Child", "1a")
	// Even a pickup-only relationship prevents implicit account reassignment.
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, old.ID, "pickup_only")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, old.ID, account.ID))
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, next.ID, account.ID), userModels.ErrGuardianAccountConflict)
	storedOld, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, old.ID)
	require.NoError(t, err)
	require.NotNil(t, storedOld.AccountID)
	assert.Equal(t, account.ID, *storedOld.AccountID)
	storedNext, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, next.ID)
	require.NoError(t, err)
	assert.Nil(t, storedNext.AccountID)
}

func TestGuardianAccountLink_RejectsTakingOverAnotherAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := peopleCompose.NewRetainedGuardianProfileRepository(db, testpkg.StudentDirectory{}, testpkg.GuardianLinks{})
	first := testpkg.CreateTestAccount(t, db, "first-parent")
	second := testpkg.CreateTestAccount(t, db, "second-parent")
	profile := testpkg.CreateTestGuardianProfile(t, db, "owned-contact")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, profile.ID, first.ID))
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, profile.ID, second.ID), userModels.ErrGuardianAccountConflict)
	stored, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, profile.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.AccountID)
	assert.Equal(t, first.ID, *stored.AccountID)
}

func TestGuardianAccountLink_DoesNotDetachAnotherSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := peopleCompose.NewRetainedGuardianProfileRepository(db, testpkg.StudentDirectory{}, testpkg.GuardianLinks{})
	account := testpkg.CreateTestAccount(t, db, "multischool-parent")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherCtx := testpkg.ContextForTenant(ctx, otherTenant)
	other := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Other", "School", "other-school")
	require.NoError(t, testpkg.TenantCommand2(t, otherCtx, db, repo.LinkAccount, other.ID, account.ID))
	next := testpkg.CreateTestGuardianProfile(t, db, "local-contact")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, next.ID, account.ID))
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, other.ID, account.ID), peopleValues.ErrGuardianProfileNotFound)
	for _, id := range []int64{other.ID, next.ID} {
		readCtx := ctx
		if id == other.ID {
			readCtx = otherCtx
		}
		profile, err := testpkg.TenantQuery1(t, readCtx, db, repo.FindByID, id)
		require.NoError(t, err)
		require.NotNil(t, profile.AccountID)
		assert.Equal(t, account.ID, *profile.AccountID)
	}
}

func TestGuardianAccountLink_RollsBackWithInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := peopleCompose.NewRetainedGuardianProfileRepository(db, testpkg.StudentDirectory{}, testpkg.GuardianLinks{})
	account := testpkg.CreateTestAccount(t, db, "rollback-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "rollback-old")
	next := testpkg.CreateTestGuardianProfile(t, db, "rollback-new")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, old.ID, account.ID))
	rejected := errors.New("later invitation step failed")
	err := testpkg.RunInTenantTransaction(ctx, func(txCtx context.Context) error {
		if err := testpkg.TenantCommand2(t, txCtx, db, repo.LinkAccount, next.ID, account.ID); err != nil {
			return err
		}
		return rejected
	})
	require.ErrorIs(t, err, rejected)
	storedOld, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, old.ID)
	require.NoError(t, err)
	require.NotNil(t, storedOld.AccountID)
	assert.Equal(t, account.ID, *storedOld.AccountID)
	storedNext, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, next.ID)
	require.NoError(t, err)
	assert.Nil(t, storedNext.AccountID)
}

func TestGuardianAccountLink_ConcurrentReplacementsDoNotStealChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := peopleCompose.NewRetainedGuardianProfileRepository(db, testpkg.StudentDirectory{}, testpkg.GuardianLinks{})
	account := testpkg.CreateTestAccount(t, db, "concurrent-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "concurrent-old")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, old.ID, account.ID))
	first := testpkg.CreateTestGuardianProfile(t, db, "concurrent-first")
	second := testpkg.CreateTestGuardianProfile(t, db, "concurrent-second")
	child := testpkg.CreateTestStudent(t, db, "Concurrent", "Child", "1a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, first.ID, "legal_guardian")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, second.ID, "legal_guardian")
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []int64{first.ID, second.ID} {
		go func() {
			<-start
			results <- testpkg.TenantCommand2(t, ctx, db, repo.LinkAccount, id, account.ID)
		}()
	}
	close(start)
	succeeded := 0
	for range 2 {
		err := <-results
		if err == nil {
			succeeded++
		} else {
			require.ErrorIs(t, err, userModels.ErrGuardianAccountConflict)
		}
	}
	assert.Equal(t, 1, succeeded)
	storedOld, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, old.ID)
	require.NoError(t, err)
	assert.Nil(t, storedOld.AccountID)
}
