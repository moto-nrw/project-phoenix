package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOffboardingCleanupQueueRollsBackRetriesAndFencesExpiredClaims(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Subject")
	now := time.Now()
	queue, err := NewDocumentCleanup(db, func() time.Time { return now })
	require.NoError(t, err)
	failure := errors.New("retirement failed after enqueue")
	err = testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := queue.Enqueue(txCtx, staff.ID); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	backlog, err := queue.Backlog(ctx)
	require.NoError(t, err)
	require.Zero(t, backlog.Pending)
	require.NoError(t, queue.Enqueue(ctx, staff.ID))
	require.NoError(t, queue.Enqueue(ctx, staff.ID))
	claims, err := queue.Claim(ctx, 10)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, 1, claims[0].Attempts)
	otherClaims, err := queue.Claim(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, otherClaims, "a committed lease excludes another worker")
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	finished, err := queue.Finish(testpkg.TenantContext(otherID), claims[0], true)
	require.NoError(t, err)
	require.False(t, finished, "even a known lease token cannot cross the tenant boundary")
	now = now.Add(3 * time.Minute)
	reclaimed, err := queue.Claim(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, 2, reclaimed[0].Attempts)
	finished, err = queue.Finish(ctx, claims[0], true)
	require.NoError(t, err)
	require.False(t, finished, "an expired worker cannot complete a replacement worker's claim")
	finished, err = queue.Finish(ctx, reclaimed[0], false)
	require.NoError(t, err)
	require.True(t, finished)
	claims, err = queue.Claim(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, claims, "a failed attempt observes the retry delay")
	now = now.Add(time.Minute)
	claims, err = queue.Claim(ctx, 10)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, 3, claims[0].Attempts)
	finished, err = queue.Finish(ctx, claims[0], true)
	require.NoError(t, err)
	require.True(t, finished)
	require.NoError(t, queue.Enqueue(ctx, staff.ID))
	backlog, err = queue.Backlog(ctx)
	require.NoError(t, err)
	require.Zero(t, backlog.Pending, "retrying retirement cannot reopen completed cleanup")
}

func TestOffboardingCleanupClaimRejectsUncommittedCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	queue, err := NewDocumentCleanup(db, nil)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinCurrentTenant(testpkg.Ctx(t), func(ctx context.Context) error {
		_, err := queue.Claim(ctx, 1)
		require.ErrorContains(t, err, "claim must commit independently")
		return nil
	}))
}
