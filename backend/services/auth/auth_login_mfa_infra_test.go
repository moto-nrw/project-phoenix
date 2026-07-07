package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
)

// The former stubMFAService is now authsvc.MFAStub (defined in
// mfa_stub_test.go, package auth — reachable here via the test-augmented
// build of services/auth) constructed with Strict: true, reproducing the
// original "an accidental call must surface immediately" contract: every
// method beyond IsRequired / HasEnrollment / AccountBelongsToTenant panics.

// TestLoginWithMFAGate_IsRequiredInfraError_ReturnsMFAStatusUnavailable proves
// the Item #3 contract: when IsRequired surfaces a non-not-found infra error
// (settings DB outage, etc.) the login is refused with ErrMFAStatusUnavailable.
// Before the fix the gate silently treated the failure as "MFA off" and
// issued a full session token pair — an attacker who could DoS the settings
// table could downgrade MFA-required tenants to no-MFA-at-all.
func TestLoginWithMFAGate_IsRequiredInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	sc := newLoginGateScenario(t, false) // no real MFA service — we inject our own

	stub := &authsvc.MFAStub{
		Strict:        true,
		IsRequiredErr: errors.New("settings DB timed out"),
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *authsvc.AuthError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError so handleLoginError can switch on it")
	assert.True(t, errors.Is(authErr.Err, authsvc.ErrMFAStatusUnavailable),
		"infra-error from IsRequired must surface as ErrMFAStatusUnavailable")
}

// TestLoginWithMFAGate_HasEnrollmentInfraError_ReturnsMFAStatusUnavailable
// mirrors the previous test for the enrollment-lookup branch. The fix lifts
// HasEnrollment from "swallow ALL errors as (false, nil)" to
// "swallow only sql.ErrNoRows; propagate everything else".
func TestLoginWithMFAGate_HasEnrollmentInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	stub := &authsvc.MFAStub{
		Strict:           true,
		IsRequiredResult: true, // MFA required for this account
		HasEnrollmentErr: errors.New("mfa_credentials lookup failed: conn reset"),
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *authsvc.AuthError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError")
	assert.True(t, errors.Is(authErr.Err, authsvc.ErrMFAStatusUnavailable),
		"infra-error from HasEnrollment must surface as ErrMFAStatusUnavailable")
}

// TestLoginWithMFAGate_StubReturnsNotEnrolledNoError_TreatedAsNotEnrolled
// proves that when HasEnrollment correctly returns (false, nil) — which is
// exactly the contract the real implementation upholds for sql.ErrNoRows —
// the gate proceeds normally and issues tokens (or, for required+enrolled,
// the enrollment token, depending on isRequired). The complementary
// sql.ErrNoRows-translation-inside-HasEnrollment behaviour is verified
// against the real DB by TestMFAService_EnrollDisableLifecycle (a fresh
// account hits FindByAccountID's sql.ErrNoRows path and HasEnrollment must
// still return (false, nil)).
func TestLoginWithMFAGate_StubReturnsNotEnrolledNoError_TreatedAsNotEnrolled(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	stub := &authsvc.MFAStub{
		Strict:           true,
		IsRequiredResult: false, // MFA off → plain token pair, regardless of enrollment
		HasEnrollmentRes: false,
		HasEnrollmentErr: nil,
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, authsvc.LoginStatusAuthenticated, result.Status)
}
