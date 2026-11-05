package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestEnsureTestTenant_IsConcurrentSafe(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantID := UniqueTestTenantID(t)
	start := make(chan struct{})
	errs := make(chan error, 16)
	var wg sync.WaitGroup

	for range cap(errs) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errs <- ensureTestTenant(ctx, db, tenantID)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestCleanupTenantTestDataRemovesPlatformOwners(t *testing.T) {
	t.Parallel()

	db := SetupIsolatedTestDB(t)
	tenantID, _ := CreateTestTenant(t, db)

	cleanupTenantTestData(t, db, tenantID)

	for _, table := range []string{"platform.schools", "platform.organizations"} {
		var exists bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = ?)", tenantID).
			Scan(context.Background(), &exists)
		require.NoError(t, err)
		require.False(t, exists, "%s row must be removed", table)
	}
	assertCleanupDiscoversIsolatedSchema(t, db)
}

func assertCleanupDiscoversIsolatedSchema(t *testing.T, db *bun.DB) {
	t.Helper()
	// A preceding cleanup must not freeze an isolated clone's schema.
	_, err := db.ExecContext(t.Context(), `
  CREATE TABLE public.cleanup_fixture_parents (id bigint PRIMARY KEY, tenant_id bigint NOT NULL);
  CREATE TABLE public.cleanup_fixture_children (parent_id bigint REFERENCES public.cleanup_fixture_parents(id));`)
	require.NoError(t, err)
	own, _ := CreateTestTenant(t, db)
	foreign, _ := CreateTestTenant(t, db)
	for _, id := range []int64{own, foreign} {
		_, err = db.ExecContext(t.Context(), `INSERT INTO public.cleanup_fixture_parents VALUES (?, ?);
   INSERT INTO public.cleanup_fixture_children VALUES (?)`, id, id, id)
		require.NoError(t, err)
	}
	err = db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		return deleteTenantTableRows(ctx, tx, []string{"public.cleanup_fixture_parents", "public.missing_cleanup_table"}, []int64{own})
	})
	require.ErrorContains(t, err, "missing_cleanup_table")
	count, err := db.NewSelect().TableExpr("public.cleanup_fixture_parents").Count(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(2), count, "a failing batch must roll back its earlier delete")
	cleanupTenantTestData(t, db, own)
	for table, column := range map[string]string{"public.cleanup_fixture_parents": "id", "public.cleanup_fixture_children": "parent_id"} {
		var ids []int64
		require.NoError(t, db.NewRaw("SELECT "+column+" FROM "+table).Scan(t.Context(), &ids))
		require.Equal(t, []int64{foreign}, ids, "%s must retain the other tenant", table)
	}
	cleanupTenantTestData(t, db, foreign)
}
