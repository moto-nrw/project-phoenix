package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Deliberately NOT parallel: this test holds a row lock while a second
// transaction waits for it.
//
// The lock is what makes the attachment upload safe (#2890): the upload checks
// "still a draft" and "still under the limit" and writes afterwards, so a
// second upload must wait rather than pass the same check.
func TestParentAnnouncementFindByIDForUpdateBlocksSecondWriter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	draft := &usersModels.ParentAnnouncement{
		Title: "Entwurf mit Anhang", Body: "Testtext",
		Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active:   true, CreatedBy: chain.AccountID,
	}
	draft.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, draft))
	t.Cleanup(func() { _ = repo.Delete(ctx, draft.ID) })

	runtimeCtx := tenant.WithTenantID(
		tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db)),
		chain.TenantID,
	)

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	holderCtx := tenant.WithTransactionForTest(runtimeCtx, &holder)
	locked, err := repo.FindByIDForUpdate(holderCtx, draft.ID)
	require.NoError(t, err)
	require.NotNil(t, locked)

	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	waiterCtx, cancel := context.WithTimeout(tenant.WithTransactionForTest(runtimeCtx, &waiter), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, waitErr := repo.FindByIDForUpdate(waiterCtx, draft.ID)
		result <- waitErr
	}()

	select {
	case lockErr := <-result:
		t.Fatalf("second reader returned while the row lock was held: %v", lockErr)
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, holder.Rollback())
	select {
	case lockErr := <-result:
		require.NoError(t, lockErr)
	case <-time.After(5 * time.Second):
		t.Fatal("row lock was not released by rollback")
	}
	require.NoError(t, waiter.Rollback())
}

// A lock request for an announcement of another tenant finds nothing — and
// therefore locks nothing. The attachment paths turn that into 404.
func TestParentAnnouncementFindByIDForUpdateIsTenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	draft := &usersModels.ParentAnnouncement{
		Title: "Fremder Entwurf", Body: "Testtext",
		Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active:   true, CreatedBy: chain.AccountID,
	}
	draft.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, draft))
	t.Cleanup(func() { _ = repo.Delete(ctx, draft.ID) })

	found, err := repo.FindByIDForUpdate(ctx, draft.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	foreign, err := repo.FindByIDForUpdate(
		tenant.WithTenantID(ctx, chain.TenantID+1_000_000), draft.ID)
	require.NoError(t, err)
	require.Nil(t, foreign)
}
