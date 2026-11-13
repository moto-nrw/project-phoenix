package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasswordLoginRacingAPasswordChangeMintsNothing pins the login race of
// #6289: a password sign-in checks the password in one transaction and
// mints the session in a later one. A change of the password that commits
// in between must not leave the sign-in with a session the old password
// earned. The mint re-reads the account under its lock and compares the
// credential it checked with the stored one.
func TestPasswordLoginRacingAPasswordChangeMintsNothing(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixtureOn(t, "password-login-race", lockRacePool(t))
	ctx := testpkg.Ctx(t)
	var email string
	require.NoError(t, f.db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, f.accountID).Scan(ctx, &email))

	// The holder takes the school's MFA policy lock exclusively: the first
	// lock of the mint, taken after the password check committed.
	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID))
	_, err = holder.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, fmt.Sprintf("mfa-policy:%d", f.homeTenantID))
	require.NoError(t, err)

	type loginResult struct {
		refresh string
		err     error
	}
	login := make(chan loginResult, 1)
	go func() {
		_, refresh, loginErr := f.service.Login(context.WithoutCancel(ctx), email, testPassword)
		login <- loginResult{refresh: refresh, err: loginErr}
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID}, "the sign-in must have checked the password and wait at the mint")

	require.NoError(t, f.module.ChangeAccountPassword(ctx, identityaccess.OwnPasswordChange{
		AccountID: f.accountID, CurrentPassword: testPassword, NewPassword: testNewPassword,
	}))
	require.NoError(t, holder.Commit())

	var result loginResult
	select {
	case result = <-login:
	case <-time.After(10 * time.Second):
		t.Fatal("the sign-in did not finish after the lock was released")
	}
	require.ErrorIs(t, result.err, identityaccess.ErrInvalidCredentials, "the old password earns no session after the change")
	assert.Empty(t, result.refresh)
	assert.Empty(t, liveTokens(t, f.db, f.accountID), "no session of the account survives the change")
}
