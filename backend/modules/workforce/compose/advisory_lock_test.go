package compose

import (
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained lock-wait contract follows the live Workforce adapter after
// the generic repository helper loses its last production caller.
func TestAcquireXactLockReportsContendedWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Advisory", "Lock")
	key := fmt.Sprintf("staff-shift:%d:%d", testpkg.Tenant(t), staff.ID)

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	require.NoError(t, err)
	ctx, evidence := testpkg.CaptureUnitOfWorkEvidence(ctx)
	testpkg.AttachLockWaitEvidence(db)
	release := time.AfterFunc(20*time.Millisecond, func() { _ = holder.Rollback() })
	defer release.Stop()

	require.NoError(t, NewStaffShiftLock(db)(ctx, staff.ID))

	events := evidence()
	require.Len(t, events, 1)
	assert.Equal(t, "lock_wait", events[0].Kind)
	assert.GreaterOrEqual(t, events[0].Duration, 15*time.Millisecond)
}
