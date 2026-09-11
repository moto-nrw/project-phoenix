package compose

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOrganizationLifecycleLockingContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	created, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: "Locking Organization", Slug: fmt.Sprintf("locking-organization-%d", time.Now().UnixNano()), Active: true,
	})
	require.NoError(t, err)

	t.Run("lifecycle update blocks behind a school mutation reader", func(t *testing.T) {
		release, done := holdOrganizationLock(t, ctx, module, created.ID, false)
		err := withLockTimeout(ctx, "200ms", func(txCtx context.Context) error {
			_, commandErr := module.SoftDeleteOrganization(txCtx, created.ID)
			return commandErr
		})
		require.Error(t, err)
		assert.True(t, isOrganizationLockTimeout(err), "expected lock timeout, got %v", err)
		close(release)
		require.NoError(t, <-done)
	})

	t.Run("school mutation reader blocks behind a lifecycle update", func(t *testing.T) {
		release, done := holdOrganizationLock(t, ctx, module, created.ID, true)
		err := withLockTimeout(ctx, "200ms", func(txCtx context.Context) error {
			_, queryErr := module.FindOrganizationForSchoolMutation(txCtx, created.ID)
			return queryErr
		})
		require.Error(t, err)
		assert.True(t, isOrganizationLockTimeout(err), "expected lock timeout, got %v", err)
		close(release)
		require.NoError(t, <-done)
		_, err = module.RestoreOrganization(ctx, created.ID)
		require.NoError(t, err)
	})

	t.Run("school mutation readers do not block each other", func(t *testing.T) {
		release, done := holdOrganizationLock(t, ctx, module, created.ID, false)
		err := withLockTimeout(ctx, "500ms", func(txCtx context.Context) error {
			_, queryErr := module.FindOrganizationForSchoolMutation(txCtx, created.ID)
			return queryErr
		})
		assert.NoError(t, err)
		close(release)
		require.NoError(t, <-done)
	})
}

func holdOrganizationLock(t *testing.T, ctx context.Context, module *organizationtenancy.Module, id int64, update bool) (chan struct{}, chan error) {
	t.Helper()
	acquired := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
			if update {
				if _, err := module.SoftDeleteOrganization(txCtx, id); err != nil {
					return err
				}
			} else if _, err := module.FindOrganizationForSchoolMutation(txCtx, id); err != nil {
				return err
			}
			close(acquired)
			<-release
			return nil
		})
	}()
	select {
	case <-acquired:
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out acquiring organization lock")
	}
	return release, done
}

func withLockTimeout(ctx context.Context, timeout string, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
		transaction, ok := tenant.TransactionFromContext(txCtx)
		if !ok {
			return fmt.Errorf("transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return fmt.Errorf("unexpected transaction %T", transaction)
		}
		if _, err := tx.ExecContext(txCtx, "SET LOCAL lock_timeout = ?", timeout); err != nil {
			return err
		}
		return fn(txCtx)
	})
}

func isOrganizationLockTimeout(err error) bool {
	message := err.Error()
	return strings.Contains(message, "lock timeout") || strings.Contains(message, "lock_not_available") || strings.Contains(message, "55P03")
}
