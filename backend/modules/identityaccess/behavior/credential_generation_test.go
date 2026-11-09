package behavior_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	api "github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The credential generation (#6289, design in the PR): every artifact a
// password check hands out is bound to the account's credential state, and a
// password write moves that state on. These tests drive each binding against
// a password change that commits in between.

// lockRacePool is a private pool on the package database for a test that
// holds a lock while two commands queue behind it: such a test keeps four
// connections at once (holder, two waiting commands, the wait poll), and on
// the shared pool, sized -parallel plus four, several of them starve each
// other's commands of a connection instead of the lock they wait for.
func lockRacePool(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// credentialFixture is an account with a password and an e-mail factor at
// its school, over a module that composes the second factor.
type credentialFixture struct {
	db        *bun.DB
	module    api.AuthTestModule
	tokenAuth *sessiontoken.TokenAuth
	accountID int64
	tenantID  int64
	email     string
}

func newCredentialFixture(t *testing.T, prefix string) credentialFixture {
	t.Helper()
	db := lockRacePool(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	tenantID := testpkg.Tenant(t)
	email := fmt.Sprintf("%s-%d@test.local", prefix, time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, db, email, testPassword)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	require.NoError(t, module.MFA.EnrollMFA(context.Background(), account.ID))
	return credentialFixture{db: db, module: module, tokenAuth: challengeTokenAuth(t, module), accountID: account.ID, tenantID: tenantID, email: email}
}

func (f credentialFixture) generation(t *testing.T) int64 {
	t.Helper()
	var generation int64
	require.NoError(t, f.db.NewRaw(`SELECT mfa_reset_generation FROM auth.accounts WHERE id = ?`, f.accountID).Scan(context.Background(), &generation))
	return generation
}

// verifiedChallenge stores an active tenant challenge with a known code at
// the account's current generation and redeems it.
func (f credentialFixture) verifiedChallenge(t *testing.T, code string) identityaccess.VerifiedMFAChallenge {
	t.Helper()
	ctx := context.Background()
	stored, err := mfaRecordRows(t, f.db).CreateChallenge(ctx, identityaccess.AccountMFAChallenge{
		AccountID: f.accountID, Scope: identityaccess.MFAChallengeScopeTenant, TenantID: f.tenantID,
		CodeHash: testpkg.HashTestPassword(t, code), ExpiresAt: time.Now().Add(identityaccess.MFAChallengeTTL),
		IPAddress: net.ParseIP("203.0.113.10"),
	})
	require.NoError(t, err)
	token, err := tokentest.CreateMFAChallengeJWT(f.tokenAuth, tokentest.MFAChallengeClaims{
		AccountID: f.accountID, Scope: identityaccess.MFAChallengeScopeTenant, TenantID: f.tenantID, ChallengeID: stored.ID,
	}, identityaccess.MFAChallengeTTL)
	require.NoError(t, err)
	verified, err := f.module.MFA.VerifyMFAChallenge(ctx, token, code)
	require.NoError(t, err)
	return verified
}

// lockAccount opens a transaction that holds the account row, the lock a
// password write, a challenge and a remembered device all take.
func (f credentialFixture) lockAccount(t *testing.T) (*sql.Tx, int) {
	t.Helper()
	ctx := context.Background()
	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback() })
	var pid int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, f.accountID)
	require.NoError(t, err)
	return holder.Tx, pid
}

func (f credentialFixture) changePassword(newPassword string) error {
	return f.module.Auth.ChangeAccountPassword(context.Background(), identityaccess.OwnPasswordChange{
		AccountID: f.accountID, CurrentPassword: testPassword, NewPassword: newPassword,
	})
}

func awaitDone(t *testing.T, done <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not finish after the lock was released", what)
		return nil
	}
}

// TestRememberedDeviceOfAnExchangeRacingAPasswordChangeIsRefused reproduces
// review finding 1 on #6297: an MFA exchange prepared its remembered device
// and minted its session before a password change; the change revoked the
// session, and the device the exchange stored afterwards still skipped the
// second factor. The prepared device is bound to the credential generation
// the change moves on.
func TestRememberedDeviceOfAnExchangeRacingAPasswordChangeIsRefused(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t, "exchange-device-race")
	ctx := context.Background()
	verified := f.verifiedChallenge(t, "246810")
	issue, err := f.module.MFA.PrepareTrustedDevice(ctx, f.accountID, f.tenantID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	require.NotNil(t, issue, "trusted devices are on at the school")
	access, _, err := f.module.Auth.IssueTokensForVerifiedMFA(ctx, verified, "203.0.113.7", "Mozilla/5.0 (Test)")
	require.NoError(t, err)

	holder, holderPID := f.lockAccount(t)
	changed := make(chan error, 1)
	go func() { changed <- f.changePassword(testNewPassword) }()
	changePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the password change must wait for the account row")
	type deviceResult struct {
		cookie string
		err    error
	}
	stored := make(chan deviceResult, 1)
	go func() {
		cookie, _, deviceErr := issue(ctx)
		stored <- deviceResult{cookie: cookie, err: deviceErr}
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID, changePID}, "the device must queue behind the password change")
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitDone(t, changed, "the password change"))
	var device deviceResult
	select {
	case device = <-stored:
	case <-time.After(10 * time.Second):
		t.Fatal("the device did not finish after the lock was released")
	}
	require.Error(t, device.err, "the exchange stores no device after the password changed")
	if device.cookie != "" {
		trusted, verifyErr := f.module.MFA.VerifyTrustedDevice(ctx, f.accountID, f.tenantID, device.cookie)
		require.NoError(t, verifyErr)
		assert.False(t, trusted, "no remembered device survives the change")
	}
	require.ErrorIs(t, f.module.Auth.VerifyAccessSession(ctx, accessSessionOf(t, access)), identityaccess.ErrInvalidToken,
		"the exchange's session ended with the change")
}

// TestConcurrentOwnPasswordChangesOnOneProofLetOneWin reproduces review
// finding 2 on #6297: two changes checked the same current password; the
// later write replaced the earlier change's hash. The write is a
// compare-and-swap on the hash the current password was checked against.
func TestConcurrentOwnPasswordChangesOnOneProofLetOneWin(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t, "password-change-cas")
	holder, holderPID := f.lockAccount(t)
	first, second := testpkg.NewTestPassword(), testpkg.NewTestPassword()
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() { firstDone <- f.changePassword(first) }()
	firstPID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the first change must wait for the account row")
	go func() { secondDone <- f.changePassword(second) }()
	waitForBlockedBackend(t, f.db, []int{holderPID, firstPID}, "the second change must wait too")
	require.NoError(t, holder.Commit())

	firstErr := awaitDone(t, firstDone, "the first change")
	secondErr := awaitDone(t, secondDone, "the second change")
	require.NoError(t, firstErr, "the change that got the row first wins")
	require.ErrorIs(t, secondErr, identityaccess.ErrInvalidCredentials, "a change on a stale proof is refused")

	service := newFixtureAuthService(t, f.db, f.module.Auth)
	_, _, err := service.Login(testpkg.Ctx(t), f.email, first)
	require.NoError(t, err, "the winner's password signs in")
	_, _, err = service.Login(testpkg.Ctx(t), f.email, second)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "the refused change left no password behind")
}

// TestChallengeIssuedOnAStaleCredentialIsRefused binds the issuance of a code
// to the credential state its password check (or enrollment token, or the
// challenge it renews) saw. A challenge whose issuance waits for a password
// change that commits first is refused under the account lock.
func TestChallengeIssuedOnAStaleCredentialIsRefused(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t, "challenge-issue-race")
	checked := f.generation(t)

	holder, holderPID := f.lockAccount(t)
	changed := make(chan error, 1)
	go func() { changed <- f.changePassword(testNewPassword) }()
	changePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the password change must wait for the account row")
	started := make(chan error, 1)
	go func() {
		_, startErr := f.module.MFA.StartMFAChallenge(context.Background(), f.accountID, f.tenantID, identityaccess.MFAChallengeScopeTenant, net.ParseIP("203.0.113.7"), checked)
		started <- startErr
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID, changePID}, "the challenge must queue behind the password change")
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitDone(t, changed, "the password change"))
	require.ErrorIs(t, awaitDone(t, started, "the challenge"), identityaccess.ErrInvalidCredentials,
		"no code goes out for a credential state the change replaced")
}

// TestChallengeBeforeAPasswordWriteIsNotRedeemable covers redemption: a code
// mailed before the own change or a reset neither redeems nor renews.
func TestChallengeBeforeAPasswordWriteIsNotRedeemable(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(t *testing.T, f credentialFixture){
		"own change": func(t *testing.T, f credentialFixture) { require.NoError(t, f.changePassword(testNewPassword)) },
		"reset": func(t *testing.T, f credentialFixture) {
			link, err := f.module.Auth.InitiatePasswordReset(context.Background(), f.email, identityaccess.PasswordResetScopeStaff)
			require.NoError(t, err)
			require.NotNil(t, link)
			t.Cleanup(func() {
				_, cleanupErr := f.db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'password_reset' AND related_entity_id = ?`, link.ID).Exec(context.Background())
				require.NoError(t, cleanupErr)
			})
			require.NoError(t, f.module.Auth.ResetPassword(context.Background(), link.Token, testpkg.NewTestPassword()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCredentialFixture(t, "challenge-before-write")
			ctx := context.Background()
			stored, err := mfaRecordRows(t, f.db).CreateChallenge(ctx, identityaccess.AccountMFAChallenge{
				AccountID: f.accountID, Scope: identityaccess.MFAChallengeScopeTenant, TenantID: f.tenantID,
				CodeHash: testpkg.HashTestPassword(t, "135790"), ExpiresAt: time.Now().Add(identityaccess.MFAChallengeTTL),
			})
			require.NoError(t, err)
			token, err := tokentest.CreateMFAChallengeJWT(f.tokenAuth, tokentest.MFAChallengeClaims{
				AccountID: f.accountID, Scope: identityaccess.MFAChallengeScopeTenant, TenantID: f.tenantID, ChallengeID: stored.ID,
			}, identityaccess.MFAChallengeTTL)
			require.NoError(t, err)

			write(t, f)

			_, err = f.module.MFA.VerifyMFAChallenge(ctx, token, "135790")
			require.Error(t, err, "the code mailed before the write redeems nothing")
			_, err = f.module.MFA.ResendMFAChallengeForScope(ctx, token, net.ParseIP("203.0.113.7"), identityaccess.MFAChallengeScopeTenant)
			require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "nor does it renew into a fresh code")
		})
	}
}

// TestEnrollmentTokenBeforeAPasswordChangeStartsNoEnrollment binds the
// enrollment token to the credential generation its password sign-in
// checked: after a password change the token starts no enrollment code.
func TestEnrollmentTokenBeforeAPasswordChangeStartsNoEnrollment(t *testing.T) {
	t.Parallel()
	sc := newLoginGateScenario(t)
	sc.setOverride(identityaccess.MFAAdminOverrideForceOn)
	ctx := context.Background()
	result, err := sc.svc.LoginWithMFAGate(ctx, sc.email, loginGatePassword, loginGateIP, "ua-test", "", "")
	require.NoError(t, err)
	require.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
	claims := decodePreviewTokenPayload(t, result.AccessToken)
	generation, ok := claims["credential_generation"].(float64)
	require.True(t, ok, "the enrollment token names the checked credential generation")

	require.NoError(t, sc.svc.ChangeAccountPassword(ctx, identityaccess.OwnPasswordChange{
		AccountID: sc.accountID, CurrentPassword: loginGatePassword, NewPassword: testpkg.NewTestPassword(),
	}))

	_, err = sc.mfa.StartMFAChallenge(ctx, sc.accountID, sc.tenantID, identityaccess.MFAChallengeScopeTenant, net.ParseIP(loginGateIP), int64(generation))
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "the token of the old password starts no enrollment")
}
