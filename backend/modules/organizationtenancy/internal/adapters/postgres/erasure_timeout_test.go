package postgres

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestErasureSchoolLockSurvivesStatementTimeout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	operator := testpkg.CreateTestOperator(t, db)
	store, err := NewErasureStore(db)
	require.NoError(t, err)
	// Inject the exact server error that pgdriver treats as a broken socket in
	// begin, outside erase_school's query_canceled exception handler.
	_, err = db.ExecContext(t.Context(), `CREATE FUNCTION platform.cancel_erasure_begin() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'private timeout detail' USING ERRCODE = '57014'; END $$;
 CREATE TRIGGER cancel_erasure_begin BEFORE INSERT ON platform.school_erasures
 FOR EACH ROW EXECUTE FUNCTION platform.cancel_erasure_begin()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS cancel_erasure_begin ON platform.school_erasures; DROP FUNCTION IF EXISTS platform.cancel_erasure_begin()`)
		require.NoError(t, err)
	})
	err = store.WithSchoolLock(t.Context(), id, func(ctx context.Context) error {
		stepErr := store.withConnection(ctx, func(ctx context.Context, conn bun.Conn) error {
			_, err := conn.ExecContext(ctx, `SELECT platform.begin_school_erasure(?,?,?)`, id, operator.ID, "ORDER-TIMEOUT")
			return err
		})
		var timeout pgdriver.Error
		require.ErrorAs(t, stepErr, &timeout)
		require.Equal(t, "57014", timeout.Field('C'))
		require.Error(t, stepErr)

		// Even after losing the work session, the rest of the callback (including
		// file/ledger cleanup) must remain protected from a competing run.
		overlapErr := store.WithSchoolLock(t.Context(), id, func(context.Context) error {
			t.Error("timeout released the lock before the run ended")
			return nil
		})
		require.ErrorContains(t, overlapErr, "school already running")
		return stepErr
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "coordination release")
	_, err = db.ExecContext(t.Context(), `DROP TRIGGER cancel_erasure_begin ON platform.school_erasures; DROP FUNCTION platform.cancel_erasure_begin()`)
	require.NoError(t, err)
	require.NoError(t, store.WithSchoolLock(t.Context(), id, func(ctx context.Context) error {
		_, err := store.Call(ctx, time.Second, time.Minute, "begin", id, operator.ID, "ORDER-TIMEOUT")
		return err
	}))
}

func TestErasureSchoolLockSurvivesRealStatementTimeout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	store, err := NewErasureStore(db)
	require.NoError(t, err)
	err = store.WithSchoolLock(t.Context(), id, func(ctx context.Context) error {
		stepErr := store.withConnection(ctx, func(ctx context.Context, conn bun.Conn) error {
			if _, err := conn.ExecContext(ctx, `SET statement_timeout = '20ms'`); err != nil {
				return err
			}
			_, err := conn.ExecContext(ctx, `SELECT pg_sleep(10)`)
			return err
		})
		var timeout pgdriver.Error
		require.ErrorAs(t, stepErr, &timeout)
		require.Equal(t, "57014", timeout.Field('C'))
		require.ErrorContains(t, store.WithSchoolLock(t.Context(), id, func(context.Context) error {
			t.Error("work timeout must not release the coordination lock")
			return nil
		}), "school already running")
		return stepErr
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "coordination release")
	require.NoError(t, store.WithSchoolLock(t.Context(), id, func(context.Context) error { return nil }))
}

func TestErasureRejectsSingleConnectionPool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	db.SetMaxOpenConns(1)
	_, err := NewErasureStore(db)
	require.ErrorContains(t, err, "at least two connections")
}
