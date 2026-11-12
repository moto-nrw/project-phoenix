package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
)

// Issue #6638: a poll answer takes the account before the school mapping,
// like every guarded parent write. Its former own locks share-locked the
// mapping first and reached the account only through the answer row's
// foreign key, so an operator revocation (account FOR UPDATE, then the
// mapping) and the answer waited for each other until PostgreSQL aborted one
// of them. With the shared guard the answer waits for the revocation and is
// then refused, and nothing of the family changes.
func TestParentPollAnswerWaitsForAccountAccessRevocation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	rig := newGuardRig(t, db, func(deps *portalCompose.Dependencies) { composeParentDeclarations(t, db, deps) })
	f := rig.family(t)
	answer := pollAnswer(f, rig.poll(t, f))

	ctx, cancel := context.WithTimeout(context.Background(), writeDeadline)
	defer cancel()
	type revokerState struct {
		pid int
		err error
	}
	started := make(chan revokerState, 1)
	proceed := make(chan struct{})
	revoked := make(chan error, 1)
	go func() {
		revoked <- db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			var state revokerState
			state.err = tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &state.pid)
			if state.err == nil {
				// RevokeAccountTenantAccess locks the account FOR UPDATE
				// before it deactivates the mapping; its role policy refuses
				// pure guardian accounts, so the test runs its statements.
				var accountID int64
				state.err = tx.NewRaw("SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE", f.AccountID).Scan(ctx, &accountID)
			}
			started <- state
			if state.err != nil {
				return state.err
			}
			select {
			case <-proceed:
			case <-ctx.Done():
				return ctx.Err()
			}
			_, err := tx.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW(), updated_at = NOW()
				WHERE account_id = ? AND tenant_id = ? AND status = 'active'`, f.AccountID, f.TenantID).Exec(ctx)
			return err
		})
	}()
	var revoker revokerState
	select {
	case revoker = <-started:
		require.NoError(t, revoker.err)
	case <-ctx.Done():
		t.Fatal("access revocation never locked the account")
	}

	before := rig.fingerprint(t, f)
	done := rig.sendAsync(t, answer, nil, f.token)
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", revoker.pid).
			Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the poll answer must wait for the account the revocation holds")
	close(proceed)

	select {
	case err := <-revoked:
		require.NoError(t, err, "the revocation commits; it must not deadlock with the poll answer")
	case <-time.After(writeDeadline):
		t.Fatal("the revocation did not commit")
	}
	rec := awaitAnswer(t, done, answer.path)
	require.Equal(t, http.StatusConflict, rec.Code, "the guard refuses the answer as stale after the revocation: %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "care.announcement_stale")
	require.Empty(t, withoutWithdrawalRows(changedTables(before, rig.fingerprint(t, f))), "a refused answer changes nothing of the family")
}
