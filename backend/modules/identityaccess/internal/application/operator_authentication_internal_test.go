package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from services/platform (#3252): operator login with its mandatory
// MFA gate, the MFA-proven token issue, refresh with rotation recovery and
// replay detection, and the profile and password changes run over the
// module's ports instead of the retained repositories.

func TestOperatorLogin_WithoutGateIssuesTokenPairAndAudits(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	operator := f.seedOperator(42)

	result, err := f.auth.LoginWithMFAGate(context.Background(), " Operator42@Example.com ", "secret", "127.0.0.1", "ua", "")
	require.NoError(t, err)
	assert.Equal(t, domain.LoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	require.NotNil(t, result.Operator)
	assert.Equal(t, operator.ID, result.Operator.ID)
	assert.False(t, result.MFAEnrollmentRequired)

	claims := decodeAccess(t, result.AccessToken)
	assert.Equal(t, int64(42), claims.AccountID)
	assert.Equal(t, "operator:42", claims.Email, "the subject names the operator")
	assert.Equal(t, operator.Email, claims.Username)
	assert.Equal(t, []string{domain.OperatorRoleName}, claims.Roles)
	assert.Equal(t, domain.ScopePlatform, claims.Scope)
	assert.NotEmpty(t, claims.FamilyID)

	sessions := f.store.operatorSessionsOf(42)
	require.Len(t, sessions, 1)
	assert.Equal(t, decodeRefresh(t, result.RefreshToken).Token, sessions[0].Token)
	assert.NotEqual(t, "operator-refresh-42", sessions[0].Token, "the refresh handle is random, never derived from the operator id")
	assert.NotNil(t, f.store.operators[42].LastLogin, "login is stamped")
	require.Len(t, f.audit.actions(domain.OperatorAuditActionLogin), 1)
	assert.Equal(t, "127.0.0.1", f.audit.actions(domain.OperatorAuditActionLogin)[0].IPAddress)
}

func TestOperatorLogin_AuditFailureDoesNotFailLogin(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	f.audit.err = errors.New("audit log service unavailable")
	f.store.recordLoginErr = errors.New("stamp failed")

	result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "")
	require.NoError(t, err, "a failed audit row or login stamp is logged, never fatal")
	assert.NotEmpty(t, result.AccessToken)
	assert.Len(t, f.store.operatorSessionsOf(42), 1)
}

func TestOperatorLogin_CredentialFailures(t *testing.T) {
	t.Parallel()
	t.Run("unknown address", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.mfa.configured = true
		_, err := f.auth.LoginWithMFAGate(context.Background(), "nobody@example.com", "secret", "", "", "")
		require.ErrorIs(t, err, domain.ErrOperatorInvalidCredentials)
		assert.Empty(t, f.mfa.calls, "the gate is never consulted before the password check")
	})
	t.Run("wrong password", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.mfa.configured = true
		f.seedOperator(42)
		_, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "wrong", "", "", "")
		require.ErrorIs(t, err, domain.ErrOperatorInvalidCredentials)
		assert.Empty(t, f.mfa.calls)
		assert.Empty(t, f.store.operatorSessionsOf(42))
	})
	t.Run("inactive operator", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.store.addOperator(7, "inactive@example.com", "hash:secret", false)
		_, err := f.auth.LoginWithMFAGate(context.Background(), "inactive@example.com", "secret", "", "", "")
		require.ErrorIs(t, err, domain.ErrOperatorInactive)
	})
	t.Run("lookup failure propagates", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.store.findOperatorErr = errDB
		_, err := f.auth.LoginWithMFAGate(context.Background(), "operator@example.com", "secret", "", "", "")
		require.ErrorIs(t, err, errDB)
		require.NotErrorIs(t, err, domain.ErrOperatorInvalidCredentials)
	})
}

func TestOperatorLogin_MFAGate(t *testing.T) {
	t.Parallel()
	t.Run("not enrolled receives the enrollment token only", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolled = false

		result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "")
		require.NoError(t, err)
		assert.Equal(t, domain.LoginStatusMFAEnrollmentRequired, result.Status, "operator MFA is mandatory: no full session before enrollment")
		assert.Equal(t, "enroll:42:0:platform", result.AccessToken, "the enrollment token is platform-scoped")
		assert.Empty(t, result.RefreshToken)
		assert.Empty(t, result.ChallengeToken)
		assert.True(t, result.MFAEnrollmentRequired)
		assert.Equal(t, "o***@example.com", result.MaskedEmail)
		require.NotNil(t, result.Operator)
		assert.Empty(t, f.store.operatorSessionsOf(42), "no refresh session before enrollment")
	})
	t.Run("enrolled without cookie receives a challenge", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolled = true
		f.mfa.challenge = "challenge-xyz"

		result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "")
		require.NoError(t, err)
		assert.Equal(t, domain.LoginStatusMFARequired, result.Status)
		assert.Equal(t, "challenge-xyz", result.ChallengeToken)
		assert.Empty(t, result.AccessToken, "no access token until the challenge verifies")
		assert.True(t, result.TrustedDeviceEnabled)
		assert.Equal(t, 90, result.TrustedDeviceDays)
		assert.Equal(t, []string{"HasEnrollment", "StartChallenge"}, f.mfa.calls, "an empty cookie is not verified")
	})
	t.Run("valid trusted-device cookie skips the challenge", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolled = true
		f.mfa.trustedCookie = "valid-cookie"

		result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "valid-cookie")
		require.NoError(t, err)
		assert.Equal(t, domain.LoginStatusAuthenticated, result.Status)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, []string{"HasEnrollment", "VerifyTrustedDevice"}, f.mfa.calls)
	})
	t.Run("stale cookie falls through to the challenge", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolled = true
		f.mfa.trustedCookie = "valid-cookie"

		result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "stale-cookie")
		require.NoError(t, err)
		assert.Equal(t, domain.LoginStatusMFARequired, result.Status, "a non-verifiable cookie must not skip MFA")
	})
	t.Run("enrollment lookup failure refuses the login", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolledErr = errDB

		_, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "", "", "")
		require.ErrorIs(t, err, errDB, "an infrastructure error never downgrades an enrolled operator to the enrollment flow")
		assert.Empty(t, f.store.operatorSessionsOf(42))
	})
	t.Run("challenge failure surfaces its cause", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.mfa.configured = true
		f.mfa.enrolled = true
		f.mfa.challengeErr = errDB

		_, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "", "", "")
		require.ErrorIs(t, err, errDB)
	})
}

func TestIssueTokensForAuthenticatedOperator(t *testing.T) {
	t.Parallel()
	t.Run("mints a session and audits like a login", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		access, refresh, err := f.auth.IssueTokensForAuthenticatedOperator(context.Background(), 42, "127.0.0.1", "ua")
		require.NoError(t, err)
		assert.NotEmpty(t, access)
		assert.NotEmpty(t, refresh)
		assert.Len(t, f.store.operatorSessionsOf(42), 1)
		assert.Len(t, f.audit.actions(domain.OperatorAuditActionLogin), 1, "the post-MFA mint stays visible as a login")
	})
	t.Run("unknown operator", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		_, _, err := f.auth.IssueTokensForAuthenticatedOperator(context.Background(), 404, "", "")
		require.ErrorIs(t, err, domain.ErrOperatorNotFound)
	})
	t.Run("inactive operator", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.store.addOperator(7, "inactive@example.com", "hash:secret", false)
		_, _, err := f.auth.IssueTokensForAuthenticatedOperator(context.Background(), 7, "", "")
		require.ErrorIs(t, err, domain.ErrOperatorInactive, "inactive operators never receive a session token")
	})
	t.Run("lookup failure propagates", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.store.findOperatorErr = errDB
		_, _, err := f.auth.IssueTokensForAuthenticatedOperator(context.Background(), 42, "", "")
		require.ErrorIs(t, err, errDB)
	})
}

// login mints a session for operator 42 and returns its persisted handle.
func login(t *testing.T, f *operatorFixture) string {
	t.Helper()
	result, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "secret", "127.0.0.1", "ua", "")
	require.NoError(t, err)
	return decodeRefresh(t, result.RefreshToken).Token
}

func currentHandle(t *testing.T, f *operatorFixture) string {
	t.Helper()
	for _, session := range f.store.operatorSessionsOf(42) {
		if session.RotatedAt == nil {
			return session.Token
		}
	}
	t.Fatal("no live operator session")
	return ""
}

func TestOperatorRefresh_RotatesInsideOneAdminTransaction(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	handle := login(t, f)
	ctx := withProof(context.Background(), "first-proof")

	access, refresh, err := f.auth.RefreshToken(ctx, 42, handle)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	successor := decodeRefresh(t, refresh).Token
	assert.NotEqual(t, handle, successor, "refresh rotates to a new opaque handle")
	assert.Equal(t, 1, f.runtime.adminTxCount, "rotation and its hand-off commit in one administrative transaction")

	sessions := f.store.operatorSessionsOf(42)
	require.Len(t, sessions, 2, "the rotated predecessor stays as replay evidence")
	require.NotNil(t, sessions[0].RotatedAt)
	assert.Equal(t, successor, *sessions[0].ReplacementToken)
	assert.Equal(t, sessions[0].FamilyID, sessions[1].FamilyID)
	assert.Equal(t, 1, sessions[1].Generation)
}

func TestOperatorRefresh_RejectsBlankForeignAndUnknownHandles(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	f.seedOperator(43)
	handle := login(t, f)

	_, _, err := f.auth.RefreshToken(context.Background(), 42, " \t ")
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Equal(t, 0, f.runtime.adminTxCount, "a blank handle never opens a transaction")

	_, _, err = f.auth.RefreshToken(context.Background(), 42, "no-such-handle")
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid, "a handle without a row is stale or from the stateless scheme")

	_, _, err = f.auth.RefreshToken(context.Background(), 43, handle)
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid, "another operator's handle is refused")
	assert.Len(t, f.store.operatorSessionsOf(42), 1, "a foreign presentation does not touch the family")
}

func TestOperatorRefresh_InterruptedRotationRecoveredWithinGrace(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	handle := login(t, f)
	ctx := withProof(context.Background(), "recovery-proof")

	firstAccess, firstRefresh, err := f.auth.RefreshToken(ctx, 42, handle)
	require.NoError(t, err)
	current := currentHandle(t, f)

	secondAccess, secondRefresh, err := f.auth.RefreshToken(ctx, 42, handle)
	require.NoError(t, err, "the lost rotation response is recovered while the grace lasts")
	assert.NotEmpty(t, firstAccess)
	assert.NotEmpty(t, firstRefresh)
	assert.NotEmpty(t, secondAccess)
	assert.Equal(t, current, decodeRefresh(t, secondRefresh).Token, "recovery returns the existing successor")
	assert.Equal(t, current, currentHandle(t, f), "recovery does not rotate again")
	assert.Len(t, f.store.operatorSessionsOf(42), 2)
}

func TestOperatorRefresh_RecoveryFollowsMultipleHandoffs(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	predecessor := login(t, f)
	firstProof := withProof(context.Background(), "first-recovery-secret")
	_, _, err := f.auth.RefreshToken(firstProof, 42, predecessor)
	require.NoError(t, err)
	firstSuccessor := currentHandle(t, f)
	_, _, err = f.auth.RefreshToken(withProof(context.Background(), "second-recovery-secret"), 42, firstSuccessor)
	require.NoError(t, err)
	secondSuccessor := currentHandle(t, f)

	_, refresh, err := f.auth.RefreshToken(firstProof, 42, predecessor)
	require.NoError(t, err)
	assert.Equal(t, secondSuccessor, decodeRefresh(t, refresh).Token, "a delayed predecessor recovers the current successor across hops")
	assert.Len(t, f.store.operatorSessionsOf(42), 3, "multi-hop recovery preserves the family")
}

func TestOperatorRefresh_ReplayAfterGraceRevokesFamily(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	predecessor := login(t, f)
	ctx := withProof(context.Background(), "recovery-proof")
	_, _, err := f.auth.RefreshToken(ctx, 42, predecessor)
	require.NoError(t, err)

	// Age the hand-off past the recovery grace.
	for id, session := range f.store.operatorTokens {
		if session.Token == predecessor {
			expired := time.Now().Add(-fakeRotation{}.RecoveryGrace() - time.Minute)
			session.RotatedAt = &expired
			f.store.operatorTokens[id] = session
		}
	}

	_, _, err = f.auth.RefreshToken(ctx, 42, predecessor)
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Empty(t, f.store.operatorSessionsOf(42), "replay outside the recovery boundary revokes the whole family")
	revocations := f.audit.actions(domain.OperatorAuditActionTokenRevoked)
	require.Len(t, revocations, 1)
	assert.Equal(t, "replay_detected", revocations[0].RevokedSessions.Reason)
	assert.Equal(t, 2, revocations[0].RevokedSessions.Count)
}

func TestOperatorRefresh_WrongRecoveryProofRevokesFamily(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	predecessor := login(t, f)
	_, _, err := f.auth.RefreshToken(withProof(context.Background(), "the-real-proof"), 42, predecessor)
	require.NoError(t, err)

	_, _, err = f.auth.RefreshToken(withProof(context.Background(), "attacker-proof"), 42, predecessor)
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Empty(t, f.store.operatorSessionsOf(42), "a failed possession proof revokes the family")
}

func TestOperatorRefresh_ExpiredSessionRevokesFamily(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	handle := login(t, f)
	for id, session := range f.store.operatorTokens {
		session.Expiry = time.Now().Add(-time.Minute)
		f.store.operatorTokens[id] = session
	}

	_, _, err := f.auth.RefreshToken(context.Background(), 42, handle)
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Empty(t, f.store.operatorSessionsOf(42))
	require.Len(t, f.audit.actions(domain.OperatorAuditActionTokenRevoked), 1)
	assert.Equal(t, "token_expired", f.audit.actions(domain.OperatorAuditActionTokenRevoked)[0].RevokedSessions.Reason)
}

func TestOperatorRefresh_LineageMismatchRevokesFamily(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	handle := login(t, f)
	// A later generation in the family whose hand-off the presented session
	// does not record: the presented row is not the lineage's head.
	live := f.store.operatorSessionsOf(42)[0]
	_, _, err := f.store.InsertOperatorSession(context.Background(), domain.OperatorSession{
		OperatorID: 42, Token: "forged-successor", Expiry: time.Now().Add(time.Hour), FamilyID: live.FamilyID, Generation: 5,
	})
	require.NoError(t, err)

	_, _, err = f.auth.RefreshToken(context.Background(), 42, handle)
	require.ErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Empty(t, f.store.operatorSessionsOf(42))
	assert.Equal(t, "lineage_mismatch", f.audit.actions(domain.OperatorAuditActionTokenRevoked)[0].RevokedSessions.Reason)
}

func TestOperatorRefresh_HandoffLookupErrorDoesNotRevokeFamily(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	predecessor := login(t, f)
	ctx := withProof(context.Background(), "recovery-proof")
	_, _, err := f.auth.RefreshToken(ctx, 42, predecessor)
	require.NoError(t, err)
	f.store.findSessionErrs[currentHandle(t, f)] = errors.New("temporary database timeout")

	_, _, err = f.auth.RefreshToken(ctx, 42, predecessor)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temporary database timeout")
	require.NotErrorIs(t, err, domain.ErrOperatorRefreshTokenInvalid)
	assert.Len(t, f.store.operatorSessionsOf(42), 2, "transient infrastructure errors stay retryable and never revoke a valid session")
	assert.Empty(t, f.audit.actions(domain.OperatorAuditActionTokenRevoked))
}

func TestOperatorRefresh_OperatorStateIsRechecked(t *testing.T) {
	t.Parallel()
	t.Run("operator gone", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		handle := login(t, f)
		delete(f.store.operators, 42)
		_, _, err := f.auth.RefreshToken(context.Background(), 42, handle)
		require.ErrorIs(t, err, domain.ErrOperatorNotFound)
	})
	t.Run("operator deactivated", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		handle := login(t, f)
		operator := f.store.operators[42]
		operator.Active = false
		f.store.operators[42] = operator
		_, _, err := f.auth.RefreshToken(context.Background(), 42, handle)
		require.ErrorIs(t, err, domain.ErrOperatorInactive)
		assert.Len(t, f.store.operatorSessionsOf(42), 1, "the refusal rolls back, nothing is rotated")
	})
	t.Run("operator lookup failure", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		handle := login(t, f)
		f.store.findOperatorErr = errDB
		_, _, err := f.auth.RefreshToken(context.Background(), 42, handle)
		require.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "failed to find operator")
	})
}

func TestUpdateOperatorProfile(t *testing.T) {
	t.Parallel()
	t.Run("trims and stores the name", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		updated, err := f.auth.UpdateProfile(context.Background(), 42, "  New Name  ")
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.DisplayName)
		assert.Equal(t, "New Name", f.store.operators[42].DisplayName)
	})
	for name, displayName := range map[string]string{"empty": "", "whitespace": "   ", "too long": string(make([]byte, 101))} {
		t.Run(name+" name is invalid input", func(t *testing.T) {
			t.Parallel()
			f := newOperatorFixture(t)
			f.seedOperator(42)
			_, err := f.auth.UpdateProfile(context.Background(), 42, displayName)
			var invalid *domain.InvalidInputError
			require.ErrorAs(t, err, &invalid)
			assert.NotContains(t, f.store.calls, "UpdateOperator")
		})
	}
	t.Run("unknown operator", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		_, err := f.auth.UpdateProfile(context.Background(), 404, "Name")
		require.ErrorIs(t, err, domain.ErrOperatorNotFound)
	})
	t.Run("write failure", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.store.updateOperatorErr = errDB
		_, err := f.auth.UpdateProfile(context.Background(), 42, "Name")
		require.ErrorIs(t, err, errDB)
	})
}

func TestChangeOperatorPassword(t *testing.T) {
	t.Parallel()
	t.Run("rotates the hash and revokes every session and e-mail change", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		login(t, f)
		login(t, f)
		f.store.calls = nil

		require.NoError(t, f.auth.ChangePassword(context.Background(), 42, "secret", "ChangedPass789!"))
		assert.Equal(t, "hash:ChangedPass789!", f.store.operators[42].PasswordHash)
		assert.Empty(t, f.store.operatorSessionsOf(42), "a password change revokes every refresh session")
		assert.Equal(t, []int64{42}, f.credentials.invalidated, "pending e-mail change links die with the old password")
		assert.Len(t, f.audit.actions(domain.OperatorAuditActionTokenRevoked), 2, "one revocation entry per family")
		assert.Equal(t, 1, f.runtime.adminTxCount, "the rotation and the revocations share one administrative transaction")

		_, err := f.auth.LoginWithMFAGate(context.Background(), "operator42@example.com", "ChangedPass789!", "", "", "")
		require.NoError(t, err)
	})
	t.Run("wrong current password", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		err := f.auth.ChangePassword(context.Background(), 42, "wrong", "ChangedPass789!")
		require.ErrorIs(t, err, domain.ErrOperatorPasswordMismatch)
		assert.Equal(t, "hash:secret", f.store.operators[42].PasswordHash)
	})
	t.Run("weak new password", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		err := f.auth.ChangePassword(context.Background(), 42, "secret", "short")
		var invalid *domain.InvalidInputError
		require.ErrorAs(t, err, &invalid)
		assert.Contains(t, err.Error(), "complexity")
		assert.Equal(t, 0, f.runtime.adminTxCount)
	})
	t.Run("unknown operator", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		err := f.auth.ChangePassword(context.Background(), 404, "secret", "ChangedPass789!")
		require.ErrorIs(t, err, domain.ErrOperatorNotFound)
	})
	t.Run("revocation failure rolls the password back", func(t *testing.T) {
		t.Parallel()
		f := newOperatorFixture(t)
		f.seedOperator(42)
		f.credentials.err = errDB
		err := f.auth.ChangePassword(context.Background(), 42, "secret", "ChangedPass789!")
		require.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "failed to invalidate email change tokens")
	})
}
