package behavior_test

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	api "github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator credential generation (#6308): every artifact an operator's
// password check leads to (the e-mail code, its challenge and enrollment
// token, the remembered device and the session of the exchange) is bound to
// platform.operators.credential_generation, and a password write moves it on
// under the operator lock. These tests drive each binding against a password
// change that commits first, the way #6289 drives the account bindings.

const operatorCredentialIP = "203.0.113.61"

type operatorCredentialFixture struct {
	db         *bun.DB
	module     api.AuthTestModule
	operatorID int64
	email      string
}

func newOperatorCredentialFixture(t *testing.T, prefix string, enrolled bool) operatorCredentialFixture {
	t.Helper()
	// A private pool, as lockRacePool explains: a lock test keeps several
	// connections waiting at once.
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	module := newOperatorMFATestModule(t, db, withDeliveringMailer(nil)...)
	email := operatorLoginEmail(prefix)
	operator := testpkg.CreateTestOperatorWithPassword(t, db, email, testPassword)
	if enrolled {
		require.NoError(t, module.OperatorMFA.EnrollOperatorMFA(context.Background(), operator.ID))
	}
	return operatorCredentialFixture{db: db, module: module, operatorID: operator.ID, email: email}
}

func (f operatorCredentialFixture) generation(t *testing.T) int64 {
	t.Helper()
	var generation int64
	require.NoError(t, f.db.NewRaw(`SELECT credential_generation FROM platform.operators WHERE id = ?`, f.operatorID).Scan(context.Background(), &generation))
	return generation
}

// signIn runs the password step of the operator login.
func (f operatorCredentialFixture) signIn(t *testing.T, password string) *identityaccess.OperatorLoginResult {
	t.Helper()
	result, err := f.module.Auth.LoginOperatorWithMFAGate(context.Background(), f.email, password, operatorCredentialIP, "ua-test", "")
	require.NoError(t, err)
	return result
}

// challengeWithCode signs in with password and gives the code the login
// mailed a value the test knows.
func (f operatorCredentialFixture) challengeWithCode(t *testing.T, password, code string) string {
	t.Helper()
	result := f.signIn(t, password)
	require.Equal(t, identityaccess.LoginStatusMFARequired, result.Status)
	require.NotEmpty(t, result.ChallengeToken)
	active := activeOperatorChallenge(t, f.db, f.operatorID)
	require.NotNil(t, active, "the login mailed a code")
	substituteOperatorChallengeCode(t, f.db, active.ID, code)
	return result.ChallengeToken
}

func (f operatorCredentialFixture) changePassword(current, next string) error {
	return f.module.Auth.ChangeOperatorPassword(context.Background(), f.operatorID, current, next)
}

// lockOperator opens a transaction that holds the operator row: the lock a
// password write, a code, a remembered device and a session mint all take.
func (f operatorCredentialFixture) lockOperator(t *testing.T) (*sql.Tx, int) {
	t.Helper()
	ctx := context.Background()
	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback() })
	var pid int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM platform.operators WHERE id = ? FOR UPDATE`, f.operatorID)
	require.NoError(t, err)
	return holder.Tx, pid
}

// TestOperatorChallengeBeforeAPasswordChangeIsNotRedeemable: a code mailed
// for the old password neither redeems nor renews once the password changed,
// while a fresh sign-in with the new password and other operators' codes
// keep working.
func TestOperatorChallengeBeforeAPasswordChangeIsNotRedeemable(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-challenge-before-change", true)
	other := newOperatorCredentialFixture(t, "operator-challenge-bystander", true)
	ctx := context.Background()
	stale := f.challengeWithCode(t, testPassword, "246810")
	untouched := other.challengeWithCode(t, testPassword, "135790")

	changed := testpkg.NewTestPassword()
	require.NoError(t, f.changePassword(testPassword, changed))

	_, err := f.module.OperatorMFA.VerifyOperatorMFAChallenge(ctx, stale, "246810")
	require.Error(t, err, "the code mailed for the old password redeems nothing")
	_, err = f.module.OperatorMFA.ResendOperatorMFAChallenge(ctx, stale, net.ParseIP(operatorCredentialIP))
	require.ErrorIs(t, err, identityaccess.ErrMFAChallengeTokenInvalid, "nor does it renew into a fresh code")

	fresh := f.challengeWithCode(t, changed, "112233")
	proof, err := f.module.OperatorMFA.VerifyOperatorMFAChallenge(ctx, fresh, "112233")
	require.NoError(t, err, "a sign-in with the new password redeems its code")
	assert.Equal(t, f.generation(t), proof.CredentialGeneration)
	_, _, err = f.module.Auth.IssueTokensForAuthenticatedOperator(ctx, f.operatorID, &proof.CredentialGeneration, operatorCredentialIP, "ua-test")
	require.NoError(t, err, "and mints its session")

	_, err = other.module.OperatorMFA.VerifyOperatorMFAChallenge(ctx, untouched, "135790")
	require.NoError(t, err, "another operator's code is untouched")
}

// TestOperatorExchangeRacingAPasswordChangeMintsNothing: the code was
// redeemed before the change, the exchange's mint and its remembered device
// queue behind the change on the operator row. Neither issues anything for
// the old password.
func TestOperatorExchangeRacingAPasswordChangeMintsNothing(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-exchange-race", true)
	ctx := context.Background()
	proof, err := f.module.OperatorMFA.VerifyOperatorMFAChallenge(ctx, f.challengeWithCode(t, testPassword, "975310"), "975310")
	require.NoError(t, err)

	holder, holderPID := f.lockOperator(t)
	changed := make(chan error, 1)
	go func() { changed <- f.changePassword(testPassword, testNewPassword) }()
	changePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the password change must wait for the operator row")
	type minted struct {
		refresh string
		err     error
	}
	mint := make(chan minted, 1)
	go func() {
		_, refresh, mintErr := f.module.Auth.IssueTokensForAuthenticatedOperator(ctx, f.operatorID, &proof.CredentialGeneration, operatorCredentialIP, "ua-test")
		mint <- minted{refresh: refresh, err: mintErr}
	}()
	mintPID := waitForBlockedBackend(t, f.db, []int{holderPID, changePID}, "the mint must queue behind the password change")
	type remembered struct {
		cookie string
		err    error
	}
	device := make(chan remembered, 1)
	go func() {
		cookie, _, deviceErr := f.module.OperatorMFA.IssueOperatorTrustedDevice(ctx, proof, "Mozilla/5.0 (Test)", net.ParseIP(operatorCredentialIP))
		device <- remembered{cookie: cookie, err: deviceErr}
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID, changePID, mintPID}, "the device must queue behind the password change")
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitDone(t, changed, "the password change"))
	var session minted
	select {
	case session = <-mint:
	case <-time.After(10 * time.Second):
		t.Fatal("the mint did not finish after the lock was released")
	}
	assert.ErrorIs(t, session.err, identityaccess.ErrOperatorInvalidCredentials, "the exchange mints no session for the old password")
	assert.Zero(t, countOperatorSessions(t, f.db, f.operatorID), "no refresh session survives the change")
	var stored remembered
	select {
	case stored = <-device:
	case <-time.After(10 * time.Second):
		t.Fatal("the device did not finish after the lock was released")
	}
	assert.Error(t, stored.err, "the exchange remembers no device after the password changed")
	if stored.cookie != "" {
		trusted, verifyErr := f.module.OperatorMFA.VerifyOperatorTrustedDevice(ctx, f.operatorID, stored.cookie)
		require.NoError(t, verifyErr)
		assert.False(t, trusted, "no remembered device skips the second factor of the new password")
	}
}

// TestOperatorChallengeIssuedOnAStaleCredentialIsRefused: the code of a
// sign-in is issued under the operator lock only while the operator still
// holds the generation the password check read. An issuance that waits for a
// password change committing first is refused, and so is a resend.
func TestOperatorChallengeIssuedOnAStaleCredentialIsRefused(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-challenge-issue-race", true)
	checked := f.generation(t)

	holder, holderPID := f.lockOperator(t)
	changed := make(chan error, 1)
	go func() { changed <- f.changePassword(testPassword, testNewPassword) }()
	changePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the password change must wait for the operator row")
	started := make(chan error, 1)
	go func() {
		_, startErr := f.module.OperatorMFA.StartOperatorMFAChallenge(context.Background(), f.operatorID, net.ParseIP(operatorCredentialIP), &checked)
		started <- startErr
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID, changePID}, "the challenge must queue behind the password change")
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitDone(t, changed, "the password change"))
	require.ErrorIs(t, awaitDone(t, started, "the challenge"), identityaccess.ErrMFAChallengeTokenInvalid,
		"no code goes out for a password the change replaced")
	assert.Nil(t, activeOperatorChallenge(t, f.db, f.operatorID), "no redeemable code was stored")
}

// TestOperatorEnrollmentTokenBeforeAPasswordChangeStartsNoEnrollment binds
// the enrollment token to the generation its password check read. Operators
// start at generation 1, so a token naming 0 (or no generation) would fail
// the positive start below. The start that queues behind a password change
// is refused, and a confirm with the old token consumes nobody's code.
func TestOperatorEnrollmentTokenBeforeAPasswordChangeStartsNoEnrollment(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-enrollment-race", false)
	ctx := context.Background()
	result := f.signIn(t, testPassword)
	require.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
	claimed, ok := decodePreviewTokenPayload(t, result.AccessToken)["credential_generation"].(float64)
	require.True(t, ok, "the enrollment token names the checked credential generation")
	checked := int64(claimed)
	require.Equal(t, f.generation(t), checked, "the token names the generation the password check read")
	_, err := f.module.OperatorMFA.StartOperatorMFAChallenge(ctx, f.operatorID, net.ParseIP(operatorCredentialIP), &checked)
	require.NoError(t, err, "while the generation holds, the token starts the enrollment")

	holder, holderPID := f.lockOperator(t)
	changed := make(chan error, 1)
	go func() { changed <- f.changePassword(testPassword, testNewPassword) }()
	changePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the password change must wait for the operator row")
	started := make(chan error, 1)
	go func() {
		_, startErr := f.module.OperatorMFA.StartOperatorMFAChallenge(ctx, f.operatorID, net.ParseIP(operatorCredentialIP), &checked)
		started <- startErr
	}()
	waitForBlockedBackend(t, f.db, []int{holderPID, changePID}, "the enrollment start must queue behind the password change")
	require.NoError(t, holder.Commit())
	require.NoError(t, awaitDone(t, changed, "the password change"))
	require.ErrorIs(t, awaitDone(t, started, "the enrollment start"), identityaccess.ErrMFAChallengeTokenInvalid,
		"the token of the old password starts no enrollment")

	// A code the operator gets under the new password stays theirs: the old
	// token's confirm is refused before the code is compared or consumed.
	_, err = f.module.OperatorMFA.StartOperatorMFAChallenge(ctx, f.operatorID, net.ParseIP(operatorCredentialIP), nil)
	require.NoError(t, err)
	active := activeOperatorChallenge(t, f.db, f.operatorID)
	require.NotNil(t, active)
	substituteOperatorChallengeCode(t, f.db, active.ID, "864200")
	require.ErrorIs(t, f.module.OperatorMFA.VerifyOperatorMFACode(ctx, f.operatorID, "864200", &checked), identityaccess.ErrMFACodeInvalid,
		"the old enrollment token confirms nothing")
	current := f.generation(t)
	require.NoError(t, f.module.OperatorMFA.VerifyOperatorMFACode(ctx, f.operatorID, "864200", &current),
		"the code is still redeemable under the new password")
}
