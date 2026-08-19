package platform_test

// Coverage for OperatorMFACredentialRepository.Update + List, both at 0%
// in the diff coverage profile. The smoke test exercises Create / Find /
// UpdateLastUsedAt / DeleteByOperatorID — this file fills the gap.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorMFACredentialRepository_Update_PersistsChanges(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := platformRepo.NewOperatorMFACredentialRepository(db)

	op := createTestOperator(t, db, fmt.Sprintf("op-cred-update-%d@test.local", time.Now().UnixNano()), "Cred Update Op")
	defer cleanupTestOperator(t, db, op.ID)

	cred := &platform.OperatorMFACredential{
		OperatorID: op.ID,
		Method:     platform.OperatorMFAMethodEmail,
	}
	require.NoError(t, repo.Create(ctx, cred))

	// Mutate + Update — read-back must reflect the change.
	stamp := time.Now().Truncate(time.Second).UTC()
	cred.LastUsedAt = &stamp
	require.NoError(t, repo.Update(ctx, cred))

	fetched, err := repo.FindByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.LastUsedAt)
	assert.WithinDuration(t, stamp, fetched.LastUsedAt.UTC(), time.Second)
}

func TestOperatorMFACredentialRepository_Update_NilRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorMFACredentialRepository(db)
	err := repo.Update(context.Background(), nil)
	require.Error(t, err, "nil input must be rejected at the repo boundary so callers get a clear validation error instead of a Bun panic")
}

func TestOperatorMFACredentialRepository_List_FilterByOperatorID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := platformRepo.NewOperatorMFACredentialRepository(db)

	op := createTestOperator(t, db, fmt.Sprintf("op-cred-list-%d@test.local", time.Now().UnixNano()), "Cred List Op")
	defer cleanupTestOperator(t, db, op.ID)

	require.NoError(t, repo.Create(ctx, &platform.OperatorMFACredential{
		OperatorID: op.ID,
		Method:     platform.OperatorMFAMethodEmail,
	}))

	results, err := repo.List(ctx, map[string]any{"operator_id": op.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, op.ID, results[0].OperatorID)
}

func TestOperatorMFACredentialRepository_List_NoFilters_ReturnsAll(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	repo := platformRepo.NewOperatorMFACredentialRepository(db)
	// No filters branch — covers the for-loop "skip nil" path and exits
	// without adding any WHERE clauses. Result may be empty or contain
	// other rows from parallel tests; the only invariant is "no error".
	_, err := repo.List(ctx, nil)
	require.NoError(t, err, "list with no filters must succeed even if no rows are returned")
}
