package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
)

func TestParentInvitationAndChildWriteShareAccountLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	invites, err := NewGuardianInvitationServiceForTests(database, testpkg.TenantRuntime(t, db), GuardianInvitationTestConfig{Outbox: NewCapturingOutbox()})
	require.NoError(t, err)
	writer := testpkg.CreateTestParentGuardianChain(t, db)
	gate := newLockGate()
	gate.armed.Store(true)
	defer gate.open()
	rig := newGuardRig(t, db, func(deps *portalCompose.Dependencies) {
		deps.GuardianInvites = owners.NewParentGuardianAccess(invites)
		lock := deps.Memberships
		deps.Memberships = func(ctx context.Context, accountID int64) (bool, error) {
			active, err := lock(ctx, accountID)
			if accountID == writer.AccountID && err == nil && active {
				gate.afterLock(ctx)
			}
			return active, err
		}
	})
	inviter := rig.family(t)
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, inviter.TenantID, func(ctx context.Context, _ bun.Tx) error {
		return rig.linkGuardian(ctx, peopledirectory.LinkGuardian{
			StudentID: inviter.StudentID, GuardianProfileID: writer.GuardianProfileID,
			RelationshipType: "parent", GuardianRole: "legal_guardian",
		})
	}))
	rig.set(t, map[string]any{"guardians.parent_invite_mode": "direct"})
	rig.givePhoto(t, inviter)
	write := rig.sendAsync(t, photoConsent(http.MethodDelete, inviter.StudentID), nil, parentReadToken(t, writer.AccountID))
	var held lockHolder
	select {
	case held = <-gate.holder:
		require.NoError(t, held.err)
	case rec := <-write:
		t.Fatalf("child write answered before locking its account: %d %s", rec.Code, rec.Body.String())
	case <-time.After(writeDeadline):
		t.Fatal("child write never locked its account")
	}
	invite := rig.sendAsync(t, childWrite{http.MethodPost, childPath(inviter, "/related-accounts"), map[string]any{"email": writer.Email}}, nil, inviter.token)
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid)))`, held.pid).Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "invitation must wait for the writer's account")

	// The writer holds its account but has not locked the child. An invitation
	// holding that child while waiting for the account would deadlock now.
	gate.open()
	rec := awaitAnswer(t, write, "child write")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = awaitAnswer(t, invite, "invitation")
	require.Less(t, rec.Code, 300, rec.Body.String())
	require.Zero(t, rig.scalar(t, `SELECT count(*) FROM users.student_profiles
		WHERE id = ? AND photo_consent_given_at IS NOT NULL`, inviter.StudentID))
}

func TestParentInvitationLocksAccountsInIDOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	first := testpkg.CreateTestParentGuardianChain(t, db)
	second := testpkg.CreateTestParentGuardianChain(t, db)
	require.Less(t, first.AccountID, second.AccountID)
	lock := owners.NewParentInvitationAccountLock(testpkg.NewDatabase(t, db))
	require.Error(t, lock(testpkg.Ctx(t), second.AccountID, first.Email), "locks require the caller's school transaction")
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), writeDeadline)
	defer cancel()
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int
	require.NoError(t, holder.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &holderPID))
	_, err = holder.ExecContext(ctx, "SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE", first.AccountID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, db, first.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			return lock(txCtx, second.AccountID, first.Email)
		})
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "invitation must wait for the lower account ID")
	// A reciprocal invitation must still be able to take the higher account:
	// neither the caller nor the recipient may be locked ahead of the lower ID.
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE NOWAIT", second.AccountID)
		return err
	}))
	require.NoError(t, holder.Commit())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("invitation account locks did not finish")
	}
}
