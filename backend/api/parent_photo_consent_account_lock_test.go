package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestParentPhotoConsentCommitsBeforeAccountAccessRevocation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	rig := newGuardRig(t, db)
	f := rig.family(t)
	rig.givePhoto(t, f)
	rig.locked.armed.Store(true)
	defer rig.locked.open()
	write := rig.sendAsync(t, photoConsent(http.MethodDelete, f.StudentID), nil, f.token)
	var held lockHolder
	select {
	case held = <-rig.locked.holder:
		require.NoError(t, held.err)
	case rec := <-write:
		t.Fatalf("photo withdrawal answered before taking the guard: %d %s", rec.Code, rec.Body.String())
	case <-time.After(writeDeadline):
		t.Fatal("photo withdrawal never took the guard")
	}

	ctx, cancel := context.WithTimeout(context.Background(), writeDeadline)
	defer cancel()
	started := make(chan lockHolder, 1)
	revoked := make(chan error, 1)
	go func() {
		revoked <- db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			var revoker lockHolder
			revoker.err = tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &revoker.pid)
			started <- revoker
			if revoker.err != nil {
				return revoker.err
			}
			// RevokeAccountTenantAccess locks the account FOR UPDATE before
			// deactivating the mapping. Its role policy refuses pure guardian
			// accounts, so exercise that locking protocol with its statements.
			var accountID int64
			if err := tx.NewRaw("SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE", f.AccountID).Scan(ctx, &accountID); err != nil {
				return err
			}
			_, err := tx.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW(), updated_at = NOW()
				WHERE account_id = ? AND tenant_id = ? AND status = 'active'`, f.AccountID, f.TenantID).Exec(ctx)
			return err
		})
	}()
	var revoker lockHolder
	select {
	case revoker = <-started:
		require.NoError(t, revoker.err)
	case <-ctx.Done():
		t.Fatal("access revocation never started")
	}
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.NewRaw("SELECT ? = ANY(pg_blocking_pids(?))", held.pid, revoker.pid).Scan(ctx, &blocked)
		return err == nil && blocked
	}, 10*time.Second, 10*time.Millisecond, "access revocation must wait for the guarded photo withdrawal")

	// Without the early account lock, the revoker holds the account and
	// waits for the mapping. The audit insert then needs KEY SHARE on that
	// account, completing a deadlock. Both transactions must commit instead.
	rig.locked.open()
	rec := awaitAnswer(t, write, "photo withdrawal")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	select {
	case err := <-revoked:
		require.NoError(t, err, "access revocation must commit after photo withdrawal")
	case <-ctx.Done():
		t.Fatal("access revocation did not finish")
	}
	require.EqualValues(t, 1, rig.scalar(t, `SELECT count(*) FROM audit.student_consent_changes
		WHERE student_id = ? AND actor_account_id = ?`, f.StudentID, f.AccountID))
	require.Zero(t, rig.scalar(t, `SELECT count(*) FROM users.student_profiles
		WHERE id = ? AND photo_consent_given_at IS NOT NULL`, f.StudentID))
	require.EqualValues(t, 1, rig.scalar(t, `SELECT count(*) FROM auth.account_tenants
		WHERE account_id = ? AND tenant_id = ? AND status = 'inactive'`, f.AccountID, f.TenantID))
}
