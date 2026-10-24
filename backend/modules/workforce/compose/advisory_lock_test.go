package compose

import (
	"context"
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
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	require.NoError(t, err)
	ctx, evidence := testpkg.CaptureUnitOfWorkEvidence(ctx)
	testpkg.AttachLockWaitEvidence(db)
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ testpkg.Tx) error {
		release := time.AfterFunc(40*time.Millisecond, func() { _ = holder.Rollback() })
		defer release.Stop()
		return NewStaffShiftLock(db)(txCtx, staff.ID)
	}))

	var events []testpkg.UnitOfWorkEvidence
	committed := 0
	for _, event := range evidence() {
		if event.Kind == "lock_wait" {
			events = append(events, event)
		}
		if event.Kind == "transaction" && event.Result == "commit" {
			committed++
		}
	}
	require.Equal(t, 1, committed)
	require.Len(t, events, 1)
	assert.Equal(t, "lock_wait", events[0].Kind)
	assert.GreaterOrEqual(t, events[0].Duration, 15*time.Millisecond)
}
