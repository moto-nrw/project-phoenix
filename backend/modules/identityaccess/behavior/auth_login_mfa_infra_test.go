package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The MFA gate must never fail open. Since #3331 the gate is composed inside
// Identity & Access, so these tests break one statement of its record port
// and drive the real login: the previous injection point (SetMFAService with
// a stub) is gone, but the contract it pinned is the same one.
//
// Before the fix a failure was silently read as "MFA off" and a full session
// token pair was issued — an attacker who could DoS the settings table could
// downgrade MFA-required schools to no MFA at all.

// A failing override lookup is the settings-side half of the gate: it cannot
// be decided, so the login is refused with ErrMFAStatusUnavailable rather
// than downgraded.
func TestLoginWithMFAGate_PolicyInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t, withFailingMFARecords(failingMFARecords{
		findGlobalOverride: func(context.Context, int64) (identityaccess.AccountMFAOverride, bool, error) {
			return identityaccess.AccountMFAOverride{}, false, errors.New("settings DB timed out")
		},
	}))

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "the infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *identityaccess.AuthenticationError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError so handleLoginError can switch on it")
	assert.True(t, errors.Is(authErr.Err, identityaccess.ErrMFAStatusUnavailable),
		"an undecidable policy must surface as ErrMFAStatusUnavailable")
}

// The enrollment lookup is the other half. Only a missing row means "not
// enrolled"; anything else refuses this login.
func TestLoginWithMFAGate_EnrollmentInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t, withFailingMFARecords(failingMFARecords{
		findCredential: func(context.Context, int64) (identityaccess.AccountMFACredential, bool, error) {
			return identityaccess.AccountMFACredential{}, false, errors.New("mfa_credentials lookup failed: conn reset")
		},
	}))
	sc.setOverride(identityaccess.MFAAdminOverrideForceOn)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "the infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *identityaccess.AuthenticationError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError")
	assert.True(t, errors.Is(authErr.Err, identityaccess.ErrMFAStatusUnavailable),
		"an unreadable enrollment must surface as ErrMFAStatusUnavailable")
}

// A missing enrollment row is the legitimate "not enrolled" signal every
// fresh account hits: the gate proceeds instead of refusing.
func TestLoginWithMFAGate_MissingEnrollmentRow_TreatedAsNotEnrolled(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
}
