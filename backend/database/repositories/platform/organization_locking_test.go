package platform_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOrganizationRepository_LockingContract verifies the FOR SHARE / FOR UPDATE
// behavior documented on OrganizationRepository.FindByIDForShare. The service layer
// relies on this contract to prevent race conditions where a school could be
// created under or migrated to an organization that is being soft-deleted
// concurrently. If FindByIDForShare / FindByIDForUpdate lose their locking
// clauses (via refactor, ORM upgrade, etc.), these tests fail.
//
// PostgreSQL lock semantics under test:
//   - FOR UPDATE blocks readers holding FOR SHARE on the same row
//   - FOR SHARE does NOT block other FOR SHARE readers
//   - FOR SHARE is blocked by an in-flight FOR UPDATE holder
//
// Tests use a short lock_timeout on the blocked transaction so the race is
// detected deterministically instead of through goroutine timing.
func TestOrganizationRepository_LockingContract(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOrganizationRepository(db)
	ctx := testpkg.Ctx(t)

	now := time.Now().UnixNano()
	org := &platformModels.Organization{
		Model:  modelBase.Model{ID: now},
		Name:   fmt.Sprintf("LockOrg %d", now),
		Slug:   fmt.Sprintf("lock-org-%d", now),
		Active: true,
	}
	require.NoError(t, repo.Create(ctx, org))
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, org.ID)
	})

	t.Run("FOR UPDATE blocks a concurrent FOR SHARE holder", func(t *testing.T) {
		// Hold FOR SHARE on the org row in tx1.
		tx1, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback() }()
		tx1Ctx := modelBase.ContextWithTx(ctx, &tx1)

		got, err := repo.FindByIDForShare(tx1Ctx, org.ID)
		require.NoError(t, err)
		require.NotNil(t, got)

		// tx2: try FOR UPDATE with a short lock_timeout — must fail because tx1 holds FOR SHARE.
		err = runWithLockTimeout(ctx, db, "200ms", func(tx2Ctx context.Context) error {
			_, innerErr := repo.FindByIDForUpdate(tx2Ctx, org.ID)
			return innerErr
		})
		require.Error(t, err, "FindByIDForUpdate must block on a concurrent FOR SHARE holder")
		assert.True(t,
			isLockTimeoutError(err),
			"expected lock_timeout error, got: %v", err,
		)

		// Release tx1; FOR UPDATE should now succeed.
		require.NoError(t, tx1.Rollback())

		err = runWithLockTimeout(ctx, db, "200ms", func(tx2Ctx context.Context) error {
			_, innerErr := repo.FindByIDForUpdate(tx2Ctx, org.ID)
			return innerErr
		})
		require.NoError(t, err, "FindByIDForUpdate must succeed once FOR SHARE holder releases")
	})

	t.Run("FOR SHARE blocks on a concurrent FOR UPDATE holder", func(t *testing.T) {
		// Hold FOR UPDATE on the org row in tx1.
		tx1, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback() }()
		tx1Ctx := modelBase.ContextWithTx(ctx, &tx1)

		got, err := repo.FindByIDForUpdate(tx1Ctx, org.ID)
		require.NoError(t, err)
		require.NotNil(t, got)

		// tx2: try FOR SHARE with a short lock_timeout — must fail.
		err = runWithLockTimeout(ctx, db, "200ms", func(tx2Ctx context.Context) error {
			_, innerErr := repo.FindByIDForShare(tx2Ctx, org.ID)
			return innerErr
		})
		require.Error(t, err, "FindByIDForShare must block on a concurrent FOR UPDATE holder")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout error, got: %v", err)
	})

	t.Run("concurrent FOR SHARE holders do not block each other", func(t *testing.T) {
		// Two goroutines hold FOR SHARE simultaneously; neither should time out.
		tx1, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback() }()
		tx1Ctx := modelBase.ContextWithTx(ctx, &tx1)
		_, err = repo.FindByIDForShare(tx1Ctx, org.ID)
		require.NoError(t, err)

		// tx2 with a short lock_timeout — should still succeed.
		var wg sync.WaitGroup
		var tx2Err error
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx2Err = runWithLockTimeout(ctx, db, "500ms", func(tx2Ctx context.Context) error {
				_, innerErr := repo.FindByIDForShare(tx2Ctx, org.ID)
				return innerErr
			})
		}()
		wg.Wait()
		assert.NoError(t, tx2Err, "concurrent FOR SHARE acquisitions must not block each other")
	})
}

// runWithLockTimeout opens a bun transaction, sets a local lock_timeout, invokes
// fn with a tx-scoped context, and rolls back. Returns fn's error so the caller
// can distinguish lock_timeout failures from other errors.
func runWithLockTimeout(ctx context.Context, db *bun.DB, timeout string, fn func(context.Context) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = ?", timeout); err != nil {
		return err
	}

	txCtx := modelBase.ContextWithTx(ctx, &tx)
	return fn(txCtx)
}

// isLockTimeoutError returns true when PostgreSQL refused the lock request due to
// lock_timeout. The SQLSTATE for this is 55P03 (lock_not_available) and the
// message contains "canceling statement due to lock timeout".
func isLockTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "lock_not_available") ||
		strings.Contains(msg, "55P03")
}
