package users_test

import (
	"context"
	"errors"
	"testing"

	userRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianAccountLink_PreservesExistingChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := userRepo.NewGuardianProfileRepository(db)
	account := testpkg.CreateTestAccount(t, db, "linked-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "old-contact")
	next := testpkg.CreateTestGuardianProfile(t, db, "new-contact")
	child := testpkg.CreateTestStudent(t, db, "Existing", "Child", "1a")
	// Even a pickup-only relationship prevents implicit account reassignment.
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, old.ID, "pickup_only")
	require.NoError(t, repo.LinkAccount(ctx, old.ID, account.ID))
	require.ErrorIs(t, repo.LinkAccount(ctx, next.ID, account.ID), users.ErrGuardianAccountConflict)
	old, err := repo.FindByID(ctx, old.ID)
	require.NoError(t, err)
	require.NotNil(t, old.AccountID)
	assert.Equal(t, account.ID, *old.AccountID)
	next, err = repo.FindByID(ctx, next.ID)
	require.NoError(t, err)
	assert.Nil(t, next.AccountID)
}

func TestGuardianAccountLink_RejectsTakingOverAnotherAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := userRepo.NewGuardianProfileRepository(db)
	first := testpkg.CreateTestAccount(t, db, "first-parent")
	second := testpkg.CreateTestAccount(t, db, "second-parent")
	profile := testpkg.CreateTestGuardianProfile(t, db, "owned-contact")
	require.NoError(t, repo.LinkAccount(ctx, profile.ID, first.ID))
	require.ErrorIs(t, repo.LinkAccount(ctx, profile.ID, second.ID), users.ErrGuardianAccountConflict)
	profile, err := repo.FindByID(ctx, profile.ID)
	require.NoError(t, err)
	require.NotNil(t, profile.AccountID)
	assert.Equal(t, first.ID, *profile.AccountID)
}

func TestGuardianAccountLink_DoesNotDetachAnotherSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := userRepo.NewGuardianProfileRepository(db)
	account := testpkg.CreateTestAccount(t, db, "multischool-parent")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherCtx := tenant.WithTenantID(ctx, otherTenant)
	other := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Other", "School", "other-school")
	require.NoError(t, repo.LinkAccount(otherCtx, other.ID, account.ID))
	next := testpkg.CreateTestGuardianProfile(t, db, "local-contact")
	require.NoError(t, repo.LinkAccount(ctx, next.ID, account.ID))
	require.ErrorIs(t, repo.LinkAccount(ctx, other.ID, account.ID), users.ErrGuardianProfileNotFound)
	for _, id := range []int64{other.ID, next.ID} {
		profile, err := repo.FindByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, profile.AccountID)
		assert.Equal(t, account.ID, *profile.AccountID)
	}
}

func TestGuardianAccountLink_RollsBackWithInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := userRepo.NewGuardianProfileRepository(db)
	account := testpkg.CreateTestAccount(t, db, "rollback-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "rollback-old")
	next := testpkg.CreateTestGuardianProfile(t, db, "rollback-new")
	require.NoError(t, repo.LinkAccount(ctx, old.ID, account.ID))
	rejected := errors.New("later invitation step failed")
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		if err := repo.LinkAccount(txCtx, next.ID, account.ID); err != nil {
			return err
		}
		return rejected
	})
	require.ErrorIs(t, err, rejected)
	old, err = repo.FindByID(ctx, old.ID)
	require.NoError(t, err)
	require.NotNil(t, old.AccountID)
	assert.Equal(t, account.ID, *old.AccountID)
	next, err = repo.FindByID(ctx, next.ID)
	require.NoError(t, err)
	assert.Nil(t, next.AccountID)
}

func TestGuardianAccountLink_ConcurrentReplacementsDoNotStealChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := userRepo.NewGuardianProfileRepository(db)
	account := testpkg.CreateTestAccount(t, db, "concurrent-parent")
	old := testpkg.CreateTestGuardianProfile(t, db, "concurrent-old")
	require.NoError(t, repo.LinkAccount(ctx, old.ID, account.ID))
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
			results <- repo.LinkAccount(ctx, id, account.ID)
		}()
	}
	close(start)
	succeeded := 0
	for range 2 {
		err := <-results
		if err == nil {
			succeeded++
		} else {
			require.ErrorIs(t, err, users.ErrGuardianAccountConflict)
		}
	}
	assert.Equal(t, 1, succeeded)
	old, err := repo.FindByID(ctx, old.ID)
	require.NoError(t, err)
	assert.Nil(t, old.AccountID)
}
