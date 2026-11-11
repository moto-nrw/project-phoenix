package enrollmenthttp_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The approval holds the membership before taking the child lock. A bulk
// invitation for that same school must preserve an active membership without
// locking the account exclusively and then waiting on the approval's hold.
func TestParentPinApprovalAndBulkInvitationInTheSameSchool(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	rig := setupPinGuardRig(t, "Sammeleinladung")
	bulk, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, rig.db), testpkg.TenantRuntime(t, rig.db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	blocker := newRenewalGate()
	blocker.armed.Store(true)
	t.Cleanup(blocker.open)
	blocked := make(chan error, 1)
	go func() {
		blocked <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			if err := lockStudent(rig)(txCtx, tx); err != nil {
				return err
			}
			blocker.park(txCtx)
			return nil
		})
	}()
	holder := awaitHolder(t, blocker.reached, blocked, "the child blocker")
	approved := rig.decideAsync(t, rig.decision)
	approvalPID := awaitWaiter(t, rig.db, holder.pid, approved, "the approval must hold the membership before waiting for the child")
	invited := make(chan error, 1)
	go func() {
		result, err := bulk.BulkInviteToStudents(ctx, identityaccess.BulkInviteRequest{
			StudentIDs: []int64{rig.f.studentID}, CreatedBy: rig.creatorID,
		})
		if err == nil {
			assert.Equal(t, 1, result.SkippedActive)
			assert.Zero(t, result.Invited)
			assert.Empty(t, result.Problems)
		}
		invited <- err
	}()
	var inviteErr error
	finished := false
	require.Eventually(t, func() bool {
		select {
		case inviteErr = <-invited:
			finished = true
			return true
		default:
			return waitsFor(rig.db, approvalPID)
		}
	}, 10*time.Second, 10*time.Millisecond, "the bulk invitation either completes or waits on the approval's membership hold")
	blocker.open()
	require.NoError(t, awaitRenewal(t, blocked))
	require.NoError(t, awaitRenewal(t, approved), "the approval must commit without a deadlock with the bulk invitation")
	if !finished {
		inviteErr = awaitRenewal(t, invited)
	}
	require.NoError(t, inviteErr, "the bulk invitation must commit without a deadlock with the approval")
	child := rig.child(t, rig.submitted.Children[0].ID)
	require.Equal(t, enrollmentNative.ChildStatusApproved, child.Status)
	require.Equal(t, &rig.f.studentID, child.CreatedStudentID)
}
