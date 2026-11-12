//go:build testhooks

package api

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRejectInvitationAndContactEditShareLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), GuardianInvitationTestConfig{Outbox: NewCapturingOutbox()})
	require.NoError(t, err)
	people, err := NewGuardianTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	creator := testpkg.CreateTestAccount(t, db, "reject-contact-edit")
	child := testpkg.CreateTestStudent(t, db, "Pending", "Rejection", "1a")
	pending, err := service.InviteToStudent(testpkg.Ctx(t), identityaccess.InviteToStudentRequest{
		StudentID: child.ID, Email: fmt.Sprintf("reject-edit-%d@example.test", child.ID), CreatedBy: creator.ID,
		RequestedByParentAccountID: &creator.ID, RequireApproval: true,
	})
	require.NoError(t, err)
	require.NotNil(t, pending.InvitationID)
	require.True(t, testpkg.GuardianInvitationByID(t, db, *pending.InvitationID).ProfileCreatedForInvitation)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	finished := make(chan error, 1)
	newEmail := fmt.Sprintf("edited-contact-%d@example.test", child.ID)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var holderPID int
		require.NoError(t, tx.NewRaw(`SELECT pg_backend_pid()`).Scan(txCtx, &holderPID))
		_, err := tx.ExecContext(txCtx, `SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, pending.GuardianProfileID)
		require.NoError(t, err)
		workers.Add(1)
		go func() {
			defer workers.Done()
			finished <- service.RejectInvitation(ctx, *pending.InvitationID, creator.ID)
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
				WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond, "rejection must wait for the edited profile")
		return people.PeopleDirectory.UpdateGuardian(txCtx, pending.GuardianProfileID, peopledirectory.GuardianInput{
			FirstName: "Edited", LastName: "Contact", Email: &newEmail,
		})
	})
	require.NoError(t, err, "the contact edit must not deadlock with rejection's orphan cleanup")
	select {
	case err := <-finished:
		require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationNotFound, "rejection loses to the contact edit, not a deadlock")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var storedEmail string
	require.NoError(t, db.NewRaw(`SELECT email FROM users.guardian_profiles WHERE id = ?`, pending.GuardianProfileID).Scan(ctx, &storedEmail))
	assert.Equal(t, newEmail, storedEmail, "the successful edit is retained")
	assert.Equal(t, identityaccess.GuardianInvitationApprovalRejected, testpkg.GuardianInvitationByID(t, db, *pending.InvitationID).ApprovalStatus)
}
