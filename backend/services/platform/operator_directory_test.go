package platform_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The retained operator directory port is the root's adapter over the public
// Identity & Access operator capability (#3252). These tests pin the contract
// the retained e-mail change, invitation, MFA and passkey flows consume:
// validation on the retained model, (nil, nil) for a missing row, the
// identity and timestamps written back into the caller's value, and the
// atomic MFA lockout counter.

func newOperatorDirectory(t *testing.T, db *bun.DB) platform.OperatorDirectory {
	t.Helper()
	directory, err := services.NewOperatorDirectoryForTests(db)
	require.NoError(t, err)
	return directory
}

func TestOperatorDirectory_Create(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	directory := newOperatorDirectory(t, db)
	ctx := testpkg.Ctx(t)
	// The retained model value comes from the fixture row so the test needs
	// no persistence model import of its own.
	template := *testpkg.CreateTestOperator(t, db)

	t.Run("success", func(t *testing.T) {
		operator := template
		operator.ID = 0
		operator.Email = fmt.Sprintf("directory-create-%d@example.com", time.Now().UnixNano())
		operator.DisplayName = "New Operator"
		require.NoError(t, directory.Create(ctx, &operator))
		testpkg.OwnTestOperator(t, db, operator.ID)
		assert.NotZero(t, operator.ID)
		assert.NotEqual(t, template.ID, operator.ID)
		assert.NotZero(t, operator.CreatedAt)
	})
	t.Run("nil operator", func(t *testing.T) {
		err := directory.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
	t.Run("validation runs on the retained model", func(t *testing.T) {
		invalid := template
		invalid.ID = 0
		invalid.Email = ""
		require.EqualError(t, directory.Create(ctx, &invalid), "email is required")
		invalid.Email = "invalid-email"
		require.EqualError(t, directory.Create(ctx, &invalid), "invalid email format")
		invalid.Email = "test@example.com"
		invalid.DisplayName = ""
		require.EqualError(t, directory.Create(ctx, &invalid), "display name is required")
	})
}

func TestOperatorDirectory_ReadAndUpdate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	directory := newOperatorDirectory(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	found, err := directory.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, operator.Email, found.Email)
	locked, err := directory.FindByIDForUpdate(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, locked)
	byEmail, err := directory.FindByEmail(ctx, operator.Email)
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, operator.ID, byEmail.ID)

	missing, err := directory.FindByID(ctx, operator.ID+1_000_000)
	require.NoError(t, err)
	assert.Nil(t, missing, "a missing operator is (nil, nil)")
	missing, err = directory.FindByEmail(ctx, "nonexistent@example.com")
	require.NoError(t, err)
	assert.Nil(t, missing)

	found.DisplayName = "Updated Name"
	found.Active = false
	require.NoError(t, directory.Update(ctx, found))
	reloaded, err := directory.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", reloaded.DisplayName)
	assert.False(t, reloaded.Active)
	require.Error(t, directory.Update(ctx, nil))
	found.Email = "invalid-email"
	require.Error(t, directory.Update(ctx, found))

	listed, err := directory.List(ctx)
	require.NoError(t, err)
	var seen bool
	for _, entry := range listed {
		seen = seen || entry.ID == operator.ID
	}
	assert.True(t, seen)
}

// N concurrent operator MFA failures must produce N counted attempts, not
// one: the compare-and-set runs in SQL, so racing callers cannot overwrite
// each other's increment.
func TestOperatorDirectory_IncrementMFAAttempts_AtomicUnderRace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	directory := newOperatorDirectory(t, db)
	op := testpkg.CreateTestOperator(t, db)

	const (
		concurrency = 12
		threshold   = 5
		lockout     = 15 * time.Minute
	)

	results := make(chan int, concurrency)
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			res, err := directory.IncrementMFAAttempts(context.Background(), op.ID, threshold, lockout)
			if err != nil {
				errs <- err
				return
			}
			results <- res.Attempts
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	seen := make(map[int]bool, concurrency)
	for n := range results {
		assert.False(t, seen[n], "each increment must observe a unique post-update count, duplicate=%d means the UPDATE was not atomic", n)
		seen[n] = true
	}
	assert.Len(t, seen, concurrency)

	persisted, err := directory.FindByID(context.Background(), op.ID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, persisted.MFAAttempts, "persisted mfa_attempts must equal the goroutine count")
	require.NotNil(t, persisted.MFALockedUntil)
	assert.True(t, persisted.MFALockedUntil.After(time.Now()))
}

func TestOperatorDirectory_ResetMFAAttempts_ClearsCounterAndLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	directory := newOperatorDirectory(t, db)
	op := testpkg.CreateTestOperator(t, db)

	for range 6 {
		_, err := directory.IncrementMFAAttempts(context.Background(), op.ID, 5, 15*time.Minute)
		require.NoError(t, err)
	}
	require.NoError(t, directory.ResetMFAAttempts(context.Background(), op.ID))

	persisted, err := directory.FindByID(context.Background(), op.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, persisted.MFAAttempts)
	assert.Nil(t, persisted.MFALockedUntil)
}
