package planning_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real native reads preserve the missing-row envelope used by planning;
// invalid creates remain request failures rather than database failures.
func TestShiftRowsKeepTheRepositoryErrorShapes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := api.MustNewWorkforce(testpkg.MustDatabase(db))
	ctx := testpkg.Ctx(t)

	_, err := rows.ReadShift(ctx, int64(987654321))
	require.Error(t, err)
	var databaseErr interface{ StoreFailure() bool }
	require.ErrorAs(t, err, &databaseErr, "a missing row stays a repository database error")
	var notFound interface{ RepositoryNotFound() }
	assert.ErrorAs(t, err, &notFound, "the missing row carries the repository not-found marker")
	assert.ErrorIs(t, err, sql.ErrNoRows, "callers classify the driver no-rows value")

	_, err = rows.CreateStaffShift(ctx, nil)
	require.ErrorContains(t, err, "StaffShift cannot be nil or zero value")

	// An untyped ID is no longer a callable native read: its int64 signature
	// rejects it at compile time instead of performing a runtime conversion.
	invalid := &workforce.StaffShift{}
	_, err = rows.CreateStaffShift(ctx, invalid)
	require.Error(t, err)
	assert.False(t, errors.As(err, &databaseErr), "validation is the caller's fault, not the database's")
}
