package schedule

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSQLResult struct {
	rows    int64
	rowsErr error
}

func (s stubSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (s stubSQLResult) RowsAffected() (int64, error) { return s.rows, s.rowsErr }

func TestExpectRestoredRows(t *testing.T) {
	require.EqualError(t, expectRestoredRows(nil, errors.New("exec failed"), 1, "visits"), "exec failed")

	require.EqualError(t, expectRestoredRows(stubSQLResult{rowsErr: errors.New("count failed")}, nil, 1, "visits"), "count failed")

	err := expectRestoredRows(stubSQLResult{rows: 0}, nil, 1, "visits")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot mismatch for visits")

	require.NoError(t, expectRestoredRows(stubSQLResult{rows: 2}, nil, 2, "supervisors"))
}
