package behavior_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/test"
)

// The enrollment write of the operator's enrollment confirm (#6459): the
// confirm redeems the code, records the enrollment and runs the exchange.
// The enrollment is recorded under the operator lock a password write takes,
// and only while the operator still holds the credential generation the
// enrollment token names; check and write commit together.

// verifiedEnrollment runs the enrollment confirm up to the redeemed code: the
// sign-in hands out the enrollment token, its start mails a code, the confirm
// redeems it. The proof names the generation the token carries.
func (f operatorCredentialFixture) verifiedEnrollment(t *testing.T, password, code string) identityaccess.OperatorMFAProof {
	t.Helper()
	ctx := context.Background()
	result := f.signIn(t, password)
	require.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
	claimed, ok := decodePreviewTokenPayload(t, result.AccessToken)["credential_generation"].(float64)
	require.True(t, ok, "the enrollment token names the checked credential generation")
	checked := int64(claimed)
	_, err := f.module.OperatorMFA.StartOperatorMFAChallenge(ctx, f.operatorID, net.ParseIP(operatorCredentialIP), &checked)
	require.NoError(t, err)
	active := activeOperatorChallenge(t, f.db, f.operatorID)
	require.NotNil(t, active, "the enrollment start mailed a code")
	substituteOperatorChallengeCode(t, f.db, active.ID, code)
	require.NoError(t, f.module.OperatorMFA.VerifyOperatorMFACode(ctx, f.operatorID, code, &checked))
	return identityaccess.OperatorMFAProof{OperatorID: f.operatorID, CredentialGeneration: checked}
}

func (f operatorCredentialFixture) enroll(proof identityaccess.OperatorMFAProof) error {
	return f.module.OperatorMFA.EnrollOperatorMFA(context.Background(), proof)
}

// assertNotEnrolled checks that neither the enrollment nor its audit entry
// was written.
func (f operatorCredentialFixture) assertNotEnrolled(t *testing.T, message string) {
	t.Helper()
	enrolled, err := f.module.OperatorMFA.HasOperatorMFAEnrollment(context.Background(), f.operatorID)
	require.NoError(t, err)
	assert.False(t, enrolled, message)
	assert.Zero(t, countOperatorAudit(t, f.db, f.operatorID, "mfa_enrolled"), "no enrollment audit from the old password")
}

// assertStaleExchangeRefused runs the exchange of the confirm with the old
// proof: neither a session nor a remembered device comes out of it.
func (f operatorCredentialFixture) assertStaleExchangeRefused(t *testing.T, proof identityaccess.OperatorMFAProof) {
	t.Helper()
	ctx := context.Background()
	_, _, err := f.module.Auth.IssueTokensForAuthenticatedOperator(ctx, f.operatorID, &proof.CredentialGeneration, operatorCredentialIP, "ua-test")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials, "the old proof mints no session")
	cookie, _, err := f.module.OperatorMFA.IssueOperatorTrustedDevice(ctx, proof, "Mozilla/5.0 (Test)", net.ParseIP(operatorCredentialIP))
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials, "the old proof remembers no device")
	assert.Empty(t, cookie)
}

// TestOperatorEnrollmentAfterAPasswordChangeRecordsNothing: the code was
// redeemed, then a password change committed before the confirm records the
// enrollment. The enrollment of the old password is refused and leaves no
// trace; a fresh enrollment under the new password completes.
func TestOperatorEnrollmentAfterAPasswordChangeRecordsNothing(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-enroll-after-change", false)
	ctx := context.Background()
	stale := f.verifiedEnrollment(t, testPassword, "402020")

	changed := test.NewTestPassword()
	require.NoError(t, f.changePassword(testPassword, changed))

	require.ErrorIs(t, f.enroll(stale), identityaccess.ErrOperatorInvalidCredentials,
		"the confirm of the old password records no enrollment")
	f.assertNotEnrolled(t, "no enrollment from the old password")
	f.assertStaleExchangeRefused(t, stale)

	fresh := f.verifiedEnrollment(t, changed, "513131")
	require.NoError(t, f.enroll(fresh), "an enrollment under the new password completes")
	_, _, err := f.module.Auth.IssueTokensForAuthenticatedOperator(ctx, f.operatorID, &fresh.CredentialGeneration, operatorCredentialIP, "ua-test")
	require.NoError(t, err, "and mints its session")
	assert.Equal(t, 1, countOperatorAudit(t, f.db, f.operatorID, "mfa_enrolled"))
}

// TestOperatorEnrollmentRacingAPasswordChangeRecordsNothing: the enrollment
// write queues behind a password change on the operator row and, once the
// change committed, records nothing. Another operator's enrollment is
// untouched.
func TestOperatorEnrollmentRacingAPasswordChangeRecordsNothing(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-enroll-race", false)
	other := newOperatorCredentialFixture(t, "operator-enroll-bystander", false)
	stale := f.verifiedEnrollment(t, testPassword, "620620")
	bystander := other.verifiedEnrollment(t, testPassword, "731731")

	err := f.raceAgainstPasswordChange(t, "the enrollment", func() error { return f.enroll(stale) })
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials, "the enrollment of the old password is refused")
	f.assertNotEnrolled(t, "no late enrollment after the password change")
	f.assertStaleExchangeRefused(t, stale)

	require.NoError(t, other.enroll(bystander), "another operator enrolls")
	enrolled, err := other.module.OperatorMFA.HasOperatorMFAEnrollment(context.Background(), other.operatorID)
	require.NoError(t, err)
	assert.True(t, enrolled)
}

// TestOperatorEnrollmentBeforeAPasswordChangeStands: an enrollment that
// committed before the password change stays; the change only ends the old
// proof, and the next sign-in asks for the second factor, not an enrollment.
// A retried confirm of the current password still finds the enrollment.
func TestOperatorEnrollmentBeforeAPasswordChangeStands(t *testing.T) {
	t.Parallel()
	f := newOperatorCredentialFixture(t, "operator-enroll-before-change", false)
	proof := f.verifiedEnrollment(t, testPassword, "842842")
	require.NoError(t, f.enroll(proof))
	require.ErrorIs(t, f.enroll(proof), identityaccess.ErrMFAAlreadyEnrolled, "a retried confirm finds the enrollment")

	changed := test.NewTestPassword()
	require.NoError(t, f.changePassword(testPassword, changed))

	enrolled, err := f.module.OperatorMFA.HasOperatorMFAEnrollment(context.Background(), f.operatorID)
	require.NoError(t, err)
	assert.True(t, enrolled, "the enrollment before the change stands")
	assert.Equal(t, 1, countOperatorAudit(t, f.db, f.operatorID, "mfa_enrolled"))
	f.assertStaleExchangeRefused(t, proof)
	assert.Equal(t, identityaccess.LoginStatusMFARequired, f.signIn(t, changed).Status,
		"the next sign-in asks for the second factor")
}
