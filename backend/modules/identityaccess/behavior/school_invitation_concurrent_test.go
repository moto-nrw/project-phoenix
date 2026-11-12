package behavior_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Revocation holds the dormant staff account before acceptance starts.
// Acceptance must wait there without holding the school, so revocation can
// take the school lock and commit without a deadlock (#6305).
func TestSchoolInvitationAcceptanceAndSchoolDeletionShareLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	env := newInvitationEnv(t, db)
	schoolID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccountWithPassword(t, db, inviteeAddress("invitation-school-deletion"), invitationPassword)
	_, err := db.NewRaw(`UPDATE auth.accounts SET active = FALSE WHERE id = ?`, account.ID).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ?`, account.ID).Exec(t.Context())
	require.NoError(t, err)
	testpkg.CreateTestSessionFamily(t, db, account.ID, schoolID, "tenant")
	creator := testpkg.CreateTestAccount(t, db, "inviter-deletion")
	role := testpkg.CreateTestRole(t, db, "invited-staff-deletion")
	invitation, err := env.service.CreateSchoolInvitation(testpkg.Ctx(t), identityaccess.SchoolInvitationRequest{
		Email: account.Email, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Dormant"), LastName: testpkg.StrPtr("Staff"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	deletion, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var workers sync.WaitGroup
	defer func() {
		_ = deletion.Rollback()
		cancel()
		workers.Wait()
	}()
	var deletionPID int
	require.NoError(t, deletion.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &deletionPID))
	_, err = deletion.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR NO KEY UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)

	accepted := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, err := env.service.AcceptSchoolInvitation(ctx, invitation.Token, identityaccess.InvitationRegistration{
			Password: invitationPassword, ConfirmPassword: invitationPassword,
		})
		accepted <- err
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))`, deletionPID).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 5*time.Second, 10*time.Millisecond, "acceptance waits for the account held by deletion")

	_, revokeErr := deletion.NewRaw(`SELECT count(*) FROM auth.revoke_school_sessions(?)`, schoolID).Exec(ctx)
	if revokeErr == nil {
		_, revokeErr = deletion.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID).Exec(ctx)
	}
	if revokeErr == nil {
		revokeErr = deletion.Commit()
	} else {
		require.NoError(t, deletion.Rollback())
	}
	assert.NoError(t, revokeErr, "school deletion must not deadlock with acceptance")
	select {
	case acceptErr := <-accepted:
		assert.ErrorIs(t, acceptErr, identityaccess.ErrInvitationTenantDeleted, "acceptance sees the committed deletion")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.Zero(t, sessionsAtSchool(t, db, schoolID))
	assert.False(t, env.storedInvitation(t, invitation.ID).IsUsed(), "the refused invitation remains unspent")
	state, err := testpkg.ReadAccountState(ctx, db, account.ID)
	require.NoError(t, err)
	assert.False(t, state.Active, "the refused acceptance must not restore the dormant account")
	member, err := testpkg.ActiveAccountTenantExists(ctx, db, account.ID, schoolID)
	require.NoError(t, err)
	assert.False(t, member, "the refused acceptance grants no school access")
}
