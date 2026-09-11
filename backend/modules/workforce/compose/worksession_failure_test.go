package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A failing store must surface as a failure the caller can act on: the cause
// stays reachable and is never misreported as "not found" or as an empty
// result (#2690). The module gets its own pool so closing it touches no other
// test.
func TestWorkSessionReadsPreserveStoreFailures(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Store", "Failure")
	capability := buildWorkforce(t, db)
	require.NoError(t, db.Close())

	sessions, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID})
	require.Error(t, err, "a closed pool is a failure, not an empty history")
	assert.Nil(t, sessions)
	assert.ErrorContains(t, err, "database is closed")
	assert.NotErrorIs(t, err, workforce.ErrWorkSessionNotFound)

	_, err = capability.LatestOpenWorkSession(ctx, staff.ID)
	require.Error(t, err, "a closed pool is a failure, not an absent block")
	assert.NotErrorIs(t, err, workforce.ErrWorkSessionNotFound)

	_, err = capability.FindWorkSession(ctx, 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, workforce.ErrWorkSessionNotFound, "a read failure is not a missing row")

	_, err = capability.ListStaffDocuments(ctx, workforce.StaffDocumentFilter{StaffID: staff.ID})
	require.Error(t, err)
	assert.ErrorContains(t, err, "database is closed")
}
