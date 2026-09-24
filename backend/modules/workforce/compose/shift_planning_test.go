package compose

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Dienstplan row adapters over the Workforce capability keep the
// repository error shapes the planning services classify on (#3418). The
// composition of the planning services themselves is pinned by the
// behaviour suite in shift_planning_behaviour_test.go.

// zeroRow returns a zero row of the type a row write accepts. The rows are
// Workforce's planning vocabulary, which this suite reaches only through the
// adapters it pins.
func zeroRow[T any](func(context.Context, *T) error) *T {
	return new(T)
}

func TestShiftRowsKeepTheRepositoryErrorShapes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	rows := NewShiftRows(buildWorkforce(t, db))
	ctx := testpkg.Ctx(t)

	_, err := rows.FindByID(ctx, int64(987654321))
	require.Error(t, err)
	var databaseErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &databaseErr, "a missing row stays a repository database error")
	assert.ErrorIs(t, err, modelBase.ErrNotFound)
	assert.ErrorIs(t, err, sql.ErrNoRows, "callers classify the driver no-rows value")

	_, err = rows.FindByID(ctx, "not-an-id")
	require.ErrorAs(t, err, &databaseErr)
	assert.ErrorContains(t, err, "unsupported id type")

	require.ErrorContains(t, rows.Create(ctx, nil), "StaffShift cannot be nil or zero value")

	// A validation failure keeps its bare wording instead of being wrapped as
	// a database error, as the row models' own Validate() reported it.
	invalid := zeroRow(rows.Create) // StaffID 0
	err = rows.Create(ctx, invalid)
	require.Error(t, err)
	assert.False(t, errors.As(err, &databaseErr), "validation is the caller's fault, not the database's")
}
