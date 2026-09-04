package compose

import (
	"context"
	"testing"

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
	InstallMessageQueryInstrumentation(db)
	InstallMessageQueryInstrumentation(db)
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectClose()

	var observed Observation
	result, err := observeResult(context.Background(), func(got Observation) {
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
