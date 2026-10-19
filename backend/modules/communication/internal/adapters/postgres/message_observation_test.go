package postgres

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestObserveResultReportsDatabaseWork(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db := bun.NewDB(sqlDB, pgdialect.New())
	InstrumentMessageQueries(db, false)
	InstrumentMessageQueries(db, false)
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectClose()

	var observed ports.Observation
	result, err := observeResult(context.Background(), func(got ports.Observation) {
		observed = got
	}, "staff_messages.test", func(ctx context.Context) (int, error) {
		_, execErr := db.ExecContext(ctx, "UPDATE users.staff_messages SET body = body")
		return 42, execErr
	})

	require.NoError(t, err)
	assert.Equal(t, 42, result)
	assert.Equal(t, "staff_messages.test", observed.Operation)
	assert.Equal(t, 1, observed.Stats.Queries)
	assert.EqualValues(t, 3, observed.Stats.Rows)
	assert.Positive(t, observed.Stats.StatementDuration)
	assert.Positive(t, observed.Duration)
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMessageQueryInstrumentationLeavesSharedDatabaseUnchanged(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	mock.MatchExpectationsInOrder(false)
	db := bun.NewDB(sqlDB, pgdialect.New())
	for range 8 {
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 3))
	}
	mock.ExpectClose()
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	for range 8 {
		t.Run("root", func(t *testing.T) {
			t.Parallel()
			rootDB := InstrumentMessageQueries(db, true)
			require.NotSame(t, db, rootDB)
			require.Same(t, db.DB, rootDB.DB, "roots share the SQL connection pool")

			var observed ports.Observation
			_, err := observeResult(context.Background(), func(got ports.Observation) {
				observed = got
			}, "staff_messages.test", func(ctx context.Context) (int, error) {
				_, execErr := rootDB.ExecContext(ctx, "UPDATE users.staff_messages SET body = body")
				return 0, execErr
			})
			require.NoError(t, err)
			require.Equal(t, 1, observed.Stats.Queries)
			require.EqualValues(t, 3, observed.Stats.Rows)

			_, err = observeResult(context.Background(), func(got ports.Observation) {
				observed = got
			}, "staff_messages.test", func(ctx context.Context) (int, error) {
				_, execErr := db.ExecContext(ctx, "UPDATE users.staff_messages SET body = body")
				return 0, execErr
			})
			require.NoError(t, err)
			require.Zero(t, observed.Stats.Queries, "root instrumentation must not mutate the shared handle")
		})
	}
}

// observeResult binds the production operation observer around the test query.
func observeResult[T any](ctx context.Context, observe func(Observation), operation string, run func(context.Context) (T, error)) (result T, err error) {
	runCtx, finish := NewMessageObservation(observe)(ctx, operation)
	defer func() { finish(err) }()
	return run(runCtx)
}
