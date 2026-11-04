package postgres

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestErasureStopsNextDatabaseStepAfterLockSessionDies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	operator := testpkg.CreateTestOperator(t, db)
	store, err := NewErasureStore(db)
	require.NoError(t, err)
	err = store.WithSchoolLock(t.Context(), id, func(ctx context.Context) error {
		// This isolated clone has only this test's coordination advisory lock.
		var pid int
		require.NoError(t, db.NewRaw(`SELECT pid FROM pg_locks WHERE locktype='advisory' AND database=(SELECT oid FROM pg_database WHERE datname=current_database())`).Scan(t.Context(), &pid))
		var terminated bool
		require.NoError(t, db.NewRaw(`SELECT pg_terminate_backend(?,5000)`, pid).Scan(t.Context(), &terminated))
		require.True(t, terminated)
		raw, stepErr := store.Call(ctx, time.Second, time.Minute, "begin", id, operator.ID, "ORDER-LOST-LOCK")
		require.Empty(t, raw, "no new erasure order may start without coordination")
		require.ErrorContains(t, stepErr, "coordination session")
		return stepErr
	})
	require.ErrorContains(t, err, "last confirmed")
	require.ErrorContains(t, err, "57P01", "preserve the actual server termination cause")
	require.NotContains(t, err.Error(), "may retain its lock until session termination")
	var orders int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.school_erasures WHERE school_id=?`, id).Scan(t.Context(), &orders))
	require.Zero(t, orders)
	require.NoError(t, store.WithSchoolLock(t.Context(), id, func(context.Context) error { return nil }))
}
