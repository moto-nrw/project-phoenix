package auth_test

// Coverage for MFACredentialRepository.Update + List on the auth/tenant
// side. Mirrors the operator-side coverage tests so the diff metrics for
// the two parallel implementations stay symmetric.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMFACredentialRepository_Update_PersistsChanges(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := authRepo.NewMFACredentialRepository(db)

	account := testpkg.CreateTestAccount(t, db, "mfa-cred-update")
	defer testpkg.CleanupAccount(t, db, account.ID)

	cred := &auth.MFACredential{
		AccountID: account.ID,
		Method:    auth.MFAMethodEmail,
	}
	require.NoError(t, repo.Create(ctx, cred))

	stamp := time.Now().Truncate(time.Second).UTC()
	cred.LastUsedAt = &stamp
	require.NoError(t, repo.Update(ctx, cred))

	fetched, err := repo.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.LastUsedAt)
	assert.WithinDuration(t, stamp, fetched.LastUsedAt.UTC(), time.Second)
}

func TestMFACredentialRepository_Update_NilRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := authRepo.NewMFACredentialRepository(db)
	err := repo.Update(context.Background(), nil)
	require.Error(t, err, "nil input must be rejected so callers get a clear validation error")
}

func TestMFACredentialRepository_List_FilterByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := authRepo.NewMFACredentialRepository(db)

	account := testpkg.CreateTestAccount(t, db, "mfa-cred-list")
	defer testpkg.CleanupAccount(t, db, account.ID)

	require.NoError(t, repo.Create(ctx, &auth.MFACredential{
		AccountID: account.ID,
		Method:    auth.MFAMethodEmail,
	}))

	results, err := repo.List(ctx, map[string]any{"account_id": account.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, account.ID, results[0].AccountID)
}

func TestMFACredentialRepository_List_NoFilters_Succeeds(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := authRepo.NewMFACredentialRepository(db)
	_, err := repo.List(ctx, nil)
	require.NoError(t, err)
}
