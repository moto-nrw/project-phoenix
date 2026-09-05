package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryCounterRowsRespectScopeAndReset(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	counter := CaptureQueriesForContext(t, db)
	var values []int
	require.NoError(t, db.NewRaw("SELECT generate_series(1, 3)").Scan(counter.Context(Ctx(t)), &values))
	require.NoError(t, db.NewRaw("SELECT generate_series(1, 7)").Scan(Ctx(t), &values))
	rows, statements := counter.Rows()
	require.EqualValues(t, 3, rows)
	require.Equal(t, 1, statements)
	counter.Stop()
	require.NoError(t, db.NewRaw("SELECT generate_series(1, 5)").Scan(counter.Context(Ctx(t)), &values))
	rows, statements = counter.Rows()
	require.EqualValues(t, 3, rows)
	require.Equal(t, 1, statements)
	counter.Reset()
	rows, statements = counter.Rows()
	require.Zero(t, rows)
	require.Zero(t, statements)
}
