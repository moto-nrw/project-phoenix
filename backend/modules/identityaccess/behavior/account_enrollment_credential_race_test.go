package behavior_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The enrollment write of the account portals' enrollment confirm (#6518):
// the confirm redeems the code, records the enrollment and runs the exchange.
// The enrollment is recorded under the account lock a password write and an
// MFA reset take, and only while the account still holds the credential
// generation the redeemed code was bound to; check and write commit together.
// The tenant, school, Träger and caterer confirms share this one write.

// newEnrollmentFixture is an account with a password at its school that has
// not enrolled its second factor yet.
func newEnrollmentFixture(t *testing.T, prefix string) credentialFixture {
	t.Helper()
	db := lockRacePool(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	tenantID := testpkg.Tenant(t)
	email := fmt.Sprintf("%s-%d@test.local", prefix, time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, db, email, testPassword)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	return credentialFixture{db: db, module: module, tokenAuth: challengeTokenAuth(t, module), accountID: account.ID, tenantID: tenantID, email: email}
}

// verifiedEnrollment runs the enrollment confirm up to the redeemed code:
// the enrollment start mails a code bound to the generation the enrollment
// token names (the account's current one), the test substitutes a known code
// for it, and the confirm redeems it pinned to the token's account and school.
func (f credentialFixture) verifiedEnrollment(t *testing.T, code string) identityaccess.VerifiedMFAChallenge {
	t.Helper()
	ctx := context.Background()
	checked := f.generation(t)
	token, err := f.module.MFA.StartMFAChallenge(ctx, f.accountID, f.tenantID, identityaccess.MFAChallengeScopeTenant, net.ParseIP("203.0.113.10"), checked)
	require.NoError(t, err)
	result, err := f.db.NewRaw(`UPDATE auth.mfa_email_challenges SET code_hash = ?
		WHERE account_id = ? AND consumed_at IS NULL AND mfa_reset_generation = ?`,
		testpkg.HashTestPassword(t, code), f.accountID, checked).Exec(ctx)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected, "the enrollment start mailed one code at the checked generation")
	verified, err := f.module.MFA.VerifyMFAChallengeForOwner(ctx, token, code, identityaccess.MFAChallengeScopeTenant, f.accountID, f.tenantID)
	require.NoError(t, err)
	require.Equal(t, checked, verified.MFAResetGeneration, "the proof names the generation its code was bound to")
	return verified
}

func (f credentialFixture) enroll(proof identityaccess.VerifiedMFAChallenge) error {
	return f.module.MFA.EnrollMFA(context.Background(), proof)
}

// resetMFA runs as the school request that disables the factor: its
// ledger row is filed at that school (#6989).
func (f credentialFixture) resetMFA() error {
	return f.module.MFA.DisableMFA(testpkg.TenantContext(f.tenantID), f.accountID)
}

func (f credentialFixture) assertNotEnrolled(t *testing.T, message string) {
	t.Helper()
	enrolled, err := f.module.MFA.HasMFAEnrollment(context.Background(), f.accountID)
	require.NoError(t, err)
	assert.False(t, enrolled, message)
}

// assertStaleExchangeRefused runs the exchange of the confirm with the old
// proof: it mints no session, so the confirm stores no remembered device
// either (the device is stored only after a minted session).
func (f credentialFixture) assertStaleExchangeRefused(t *testing.T, proof identityaccess.VerifiedMFAChallenge) {
	t.Helper()
	_, _, err := f.module.Auth.IssueTokensForVerifiedMFA(context.Background(), proof, "203.0.113.7", "Mozilla/5.0 (Test)")
	require.Error(t, err, "the old proof mints no session")
}

// credentialWrites are the two writes that end a proof: the account's own
// password change and an MFA reset.
var credentialWrites = map[string]func(f credentialFixture) error{
	"password change": func(f credentialFixture) error { return f.changePassword(testpkg.NewTestPassword()) },
	"mfa reset":       credentialFixture.resetMFA,
}

// TestAccountEnrollmentAfterACredentialWriteRecordsNothing: the code was
// redeemed, then a password change or an MFA reset committed before the
// confirm records the enrollment. The enrollment of the old proof is refused
// with the sign-in's refusal and leaves no trace; a fresh enrollment after
// the write completes and mints its session.
func TestAccountEnrollmentAfterACredentialWriteRecordsNothing(t *testing.T) {
	t.Parallel()
	for name, write := range credentialWrites {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newEnrollmentFixture(t, "account-enroll-after-write")
			stale := f.verifiedEnrollment(t, "402020")

			require.NoError(t, write(f))

			require.ErrorIs(t, f.enroll(stale), identityaccess.ErrInvalidCredentials,
				"the confirm of the old proof records no enrollment")
			f.assertNotEnrolled(t, "no enrollment from the old proof")
			f.assertStaleExchangeRefused(t, stale)

			fresh := f.verifiedEnrollment(t, "513131")
			require.NoError(t, f.enroll(fresh), "an enrollment after the write completes")
			_, _, err := f.module.Auth.IssueTokensForVerifiedMFA(context.Background(), fresh, "203.0.113.7", "Mozilla/5.0 (Test)")
			require.NoError(t, err, "and mints its session")
		})
	}
}

// TestAccountEnrollmentRacingACredentialWriteRecordsNothing: a transaction
// holds the account row; the password change or MFA reset queues behind it,
// the enrollment write queues behind both. Once the write committed, the
// enrollment records nothing. Another account's enrollment is untouched.
func TestAccountEnrollmentRacingACredentialWriteRecordsNothing(t *testing.T) {
	t.Parallel()
	for name, write := range credentialWrites {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newEnrollmentFixture(t, "account-enroll-race")
			other := newEnrollmentFixture(t, "account-enroll-bystander")
			stale := f.verifiedEnrollment(t, "620620")
			bystander := other.verifiedEnrollment(t, "731731")

			holder, holderPID := f.lockAccount(t)
			written := make(chan error, 1)
			go func() { written <- write(f) }()
			writePID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the credential write must wait for the account row")
			enrolled := make(chan error, 1)
			go func() { enrolled <- f.enroll(stale) }()
			waitForBlockedBackend(t, f.db, []int{holderPID, writePID}, "the enrollment must queue behind the credential write")
			require.NoError(t, holder.Commit())

			require.NoError(t, awaitDone(t, written, "the credential write"))
			require.ErrorIs(t, awaitDone(t, enrolled, "the enrollment"), identityaccess.ErrInvalidCredentials,
				"the enrollment of the old proof is refused")
			f.assertNotEnrolled(t, "no late enrollment after the credential write")
			f.assertStaleExchangeRefused(t, stale)

			require.NoError(t, other.enroll(bystander), "another account enrolls")
			ok, err := other.module.MFA.HasMFAEnrollment(context.Background(), other.accountID)
			require.NoError(t, err)
			assert.True(t, ok)
		})
	}
}

// TestAccountEnrollmentBeforeAPasswordChangeStands: an enrollment that
// committed before the password change stays; the change only ends the old
// proof. A retried confirm of the current proof still finds the enrollment.
func TestAccountEnrollmentBeforeAPasswordChangeStands(t *testing.T) {
	t.Parallel()
	f := newEnrollmentFixture(t, "account-enroll-before-change")
	proof := f.verifiedEnrollment(t, "842842")
	require.NoError(t, f.enroll(proof))
	require.ErrorIs(t, f.enroll(proof), identityaccess.ErrMFAAlreadyEnrolled, "a retried confirm finds the enrollment")

	require.NoError(t, f.changePassword(testpkg.NewTestPassword()))

	ok, err := f.module.MFA.HasMFAEnrollment(context.Background(), f.accountID)
	require.NoError(t, err)
	assert.True(t, ok, "the enrollment before the change stands")
	f.assertStaleExchangeRefused(t, proof)
}

// TestCatererSwitchOnRacingAPasswordChangeRecordsNothing: the caterer's own
// switch-on confirm redeems its code while its password change waits for the
// account row; the confirm's enrollment write queues behind the change and,
// once the change committed, records nothing and mints no session.
func TestCatererSwitchOnRacingAPasswordChangeRecordsNothing(t *testing.T) {
	t.Parallel()
	sc := newCatererMFAScenarioOn(t, lockRacePool(t))
	ctx := context.Background()
	accountID := sc.account.AccountID
	challenge, err := sc.caterers.StartCatererMFA(ctx, accountID, net.ParseIP("192.0.2.40"))
	require.NoError(t, err)
	code := sc.lastCode(t)
	f := credentialFixture{db: sc.db, accountID: accountID}

	holder, holderPID := f.lockAccount(t)
	changed := make(chan error, 1)
	go func() {
		changed <- sc.caterers.ChangeCatererPassword(ctx, accountID, catererPassword, testpkg.NewTestPassword(), "192.0.2.40", "test")
	}()
	changePID := waitForBlockedBackend(t, sc.db, []int{holderPID}, "the password change must wait for the account row")
	confirmed := make(chan error, 1)
	go func() {
		_, confirmErr := sc.caterers.ConfirmCatererMFA(ctx, identityaccess.CatererMFAConfirmation{
			AccountID: accountID, ChallengeToken: challenge.ChallengeToken, Code: code, IPAddress: "192.0.2.40", UserAgent: "test",
		})
		confirmed <- confirmErr
	}()
	waitForBlockedBackend(t, sc.db, []int{holderPID, changePID}, "the confirm must queue behind the password change")
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitDone(t, changed, "the password change"))
	require.ErrorIs(t, awaitDone(t, confirmed, "the confirm"), identityaccess.ErrInvalidCredentials,
		"the code of the old password switches nothing on")
	state, err := sc.caterers.CatererMFAStatus(ctx, accountID)
	require.NoError(t, err)
	assert.False(t, state.Enabled, "no enrollment from the old password")
	assert.Empty(t, liveTokens(t, sc.db, accountID), "and no session")
}
