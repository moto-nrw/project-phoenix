package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCreateAndRotateRequestFeedSubscription(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "request-feed-store@example.test")
	initialHash := strings.Repeat("a", 64)
	replacementHash := strings.Repeat("b", 64)

	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		store := New(func(context.Context) (bun.IDB, error) { return tx, nil }, unusedFeed{})

		created, createErr := store.Create(ctx, tenantID, account.ID, initialHash)
		require.NoError(t, createErr)
		require.True(t, created)

		active, activeErr := store.Active(ctx, tenantID, account.ID)
		require.NoError(t, activeErr)
		require.True(t, active)

		rotated, rotateErr := store.Rotate(ctx, tenantID, account.ID, replacementHash)
		require.NoError(t, rotateErr)
		require.True(t, rotated)

		subscription, found, resolveErr := store.Resolve(ctx, replacementHash)
		require.NoError(t, resolveErr)
		require.True(t, found)
		require.Equal(t, tenantID, subscription.TenantID)
		require.Equal(t, account.ID, subscription.AccountID)
		return nil
	})
	require.NoError(t, err)
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

// unusedFeed stands in for the projection reader the subscription writes
// never call.
type unusedFeed struct{}

func (unusedFeed) ResolveAccess(context.Context, int64, int64) (requestfeed.FeedAccess, bool, error) {
	panic("request feed store test: the subscription writes read no access")
}

func (unusedFeed) ListNewParentRequests(context.Context, int64, time.Time, bool, bool) ([]requestfeed.FeedEntry, error) {
	panic("request feed store test: the subscription writes list no requests")
}
