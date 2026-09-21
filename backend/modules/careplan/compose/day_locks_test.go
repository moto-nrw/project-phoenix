package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The advisory key is tenant-scoped, so a missing tenant is a programming
// error: failing before any database access keeps two tenants from colliding
// on the same student and date.
func TestDayLocksRequireTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	locks, err := NewDayLocks(db, func(context.Context, int64) error { return nil }, errors.New("student not found"))
	require.NoError(t, err)

	err = locks.LockExceptionDay(context.Background(), 123, "2026-03-02")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")

	err = locks.LockStudentAndExceptionDay(context.Background(), 123, "2026-03-02")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant is required")

	err = postgres.LockExceptionDay(context.Background(), db, 0, 123, "2026-03-02")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant id is required")
}
