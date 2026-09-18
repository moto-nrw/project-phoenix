package auth_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// The second factor and the school-portal ceremonies live in Identity & Access
// since #3331, and the HTTP surfaces that may not name the module reach them
// through the retained ports. Every status code those handlers render is
// decided by an errors.Is against a retained sentinel, so a module identity
// that arrived untranslated would silently turn a 401 or a 429 into a 500.

// failingAccountMFA answers the calls the port forwards with one error.
type failingAccountMFA struct {
	identityaccess.AccountMFA
	err error
}

func (f failingAccountMFA) EnrollMFA(context.Context, int64) error { return f.err }

func (f failingAccountMFA) VerifyMFAChallenge(context.Context, string, string) (identityaccess.VerifiedMFAChallenge, error) {
	return identityaccess.VerifiedMFAChallenge{}, f.err
}

// failingAccountPasskeys answers the login ceremony with one error.
type failingAccountPasskeys struct {
	identityaccess.AccountPasskeyFlows
	err error
}

func (f failingAccountPasskeys) FinishAccountPasskeyLogin(context.Context, identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	return identityaccess.PasskeyLoginResult{}, f.err
}

func TestAccountMFAPortTranslatesEverySentinel(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		public   error
		retained error
	}{
		{identityaccess.ErrMFAChallengeTokenInvalid, auth.ErrMFAChallengeTokenInvalid},
		{identityaccess.ErrMFACodeInvalid, auth.ErrMFACodeInvalid},
		{identityaccess.ErrMFALocked, auth.ErrMFALocked},
		{identityaccess.ErrMFARateLimited, auth.ErrMFARateLimited},
		{identityaccess.ErrMFANotEnrolled, auth.ErrMFANotEnrolled},
		{identityaccess.ErrMFAAlreadyEnrolled, auth.ErrMFAAlreadyEnrolled},
		{identityaccess.ErrMFAPermissionDenied, auth.ErrMFAPermissionDenied},
		{identityaccess.ErrMFAInvalidOverride, auth.ErrMFAInvalidOverride},
		{identityaccess.ErrMFAUnsupportedScope, auth.ErrMFAUnsupportedScope},
		{identityaccess.ErrMFAStatusUnavailable, auth.ErrMFAStatusUnavailable},
	}
	for _, pair := range pairs {
		port := services.NewAccountMFAPortForTests(failingAccountMFA{err: pair.public})
		err := port.EnrollMFA(context.Background(), 1)
		assert.ErrorIs(t, err, pair.retained, "the retained twin of %v must reach the handler", pair.public)
		assert.Equal(t, pair.public.Error(), err.Error(), "the message the client sees must not change")
	}
}

// The flows report a failure as an operation error around the cause, and the
// handlers classify on the cause.
func TestAccountMFAPortTranslatesTheWrappedSentinel(t *testing.T) {
	t.Parallel()

	wrapped := &identityaccess.AuthenticationError{Op: "verify mfa challenge", Err: identityaccess.ErrMFACodeInvalid}
	port := services.NewAccountMFAPortForTests(failingAccountMFA{err: wrapped})

	_, err := port.VerifyMFAChallenge(context.Background(), "token", "123456")
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid)
	assert.Equal(t, wrapped.Error(), err.Error(), "the operation text is preserved")
}

// A passkey login mints a session, so it reports the login refusals too — the
// school portal answers those with 401 and 404.
func TestAccountPasskeyPortTranslatesTheLoginRefusals(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		public   error
		retained error
	}{
		{identityaccess.ErrAccountInactive, auth.ErrAccountInactive},
		{identityaccess.ErrAccountNotFound, auth.ErrAccountNotFound},
		{identityaccess.ErrInvalidCredentials, auth.ErrInvalidCredentials},
		{identityaccess.ErrTenantAccessDenied, auth.ErrTenantAccessDenied},
		{identityaccess.ErrTenantNotFound, auth.ErrTenantNotFound},
		{identityaccess.ErrMustUseSchoolPortal, auth.ErrMustUseSchoolPortal},
		{identityaccess.ErrParentMustUseParentPortal, auth.ErrParentMustUseParentPortal},
		{identityaccess.ErrPasskeySessionInvalid, auth.ErrPasskeySessionInvalid},
		{identityaccess.ErrPasskeyNotFound, auth.ErrPasskeyNotFound},
		{identityaccess.ErrPasskeyOriginInvalid, auth.ErrPasskeyOriginInvalid},
	}
	for _, pair := range pairs {
		port := services.NewAccountPasskeyPortForTests(failingAccountPasskeys{err: pair.public})
		_, err := port.FinishAccountPasskeyLogin(context.Background(), auth.AccountPasskeyLoginFinish{})
		assert.ErrorIs(t, err, pair.retained, "the retained twin of %v must reach the handler", pair.public)
	}
}

// The gate seam runs the translation backwards: a composition handed the
// second factor as the retained port composes the module over it, and the
// module's own login paths compare against its identities.
func TestModuleAccountMFATranslatesBack(t *testing.T) {
	t.Parallel()

	capability := services.NewModuleAccountMFAForTests(retainedFailingMFA{err: auth.ErrMFAStatusUnavailable})
	_, err := capability.HasMFAEnrollment(context.Background(), 1)
	assert.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)

	assert.Nil(t, services.NewModuleAccountMFAForTests(nil), "a composition without the port composes nothing")
	assert.Nil(t, services.NewAccountMFAPortForTests(nil))
	assert.Nil(t, services.NewAccountPasskeyPortForTests(nil))
}

// retainedFailingMFA answers the read the gate performs with one error.
type retainedFailingMFA struct {
	auth.MFAService
	err error
}

func (r retainedFailingMFA) HasMFAEnrollment(context.Context, int64) (bool, error) {
	return false, r.err
}

func (r retainedFailingMFA) IssueTrustedDevice(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, r.err
}

// The sentinels are what the routes render, so their text is the response
// body a client reads. It must stay the text the retained services produced.
func TestRetainedSentinelsKeepTheirWireText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "passkey origin is invalid", auth.ErrPasskeyOriginInvalid.Error())
	assert.Equal(t, "passkey session is invalid", auth.ErrPasskeySessionInvalid.Error())
	assert.Equal(t, "passkey not found", auth.ErrPasskeyNotFound.Error())
	assert.Equal(t, "mfa status unavailable, please retry", auth.ErrMFAStatusUnavailable.Error())
	assert.Equal(t, "invalid or expired code", auth.ErrMFACodeInvalid.Error())
	assert.Equal(t, "invalid or expired challenge token", auth.ErrMFAChallengeTokenInvalid.Error())

	// And the module's own identities carry the same text, so the
	// translation never rewrites a body.
	for _, pair := range []struct{ public, retained error }{
		{identityaccess.ErrPasskeyOriginInvalid, auth.ErrPasskeyOriginInvalid},
		{identityaccess.ErrPasskeySessionInvalid, auth.ErrPasskeySessionInvalid},
		{identityaccess.ErrPasskeyNotFound, auth.ErrPasskeyNotFound},
		{identityaccess.ErrMFACodeInvalid, auth.ErrMFACodeInvalid},
		{identityaccess.ErrMFALocked, auth.ErrMFALocked},
		{identityaccess.ErrMFARateLimited, auth.ErrMFARateLimited},
		{identityaccess.ErrMFANotEnrolled, auth.ErrMFANotEnrolled},
		{identityaccess.ErrMFAAlreadyEnrolled, auth.ErrMFAAlreadyEnrolled},
		{identityaccess.ErrMFAPermissionDenied, auth.ErrMFAPermissionDenied},
		{identityaccess.ErrMFAInvalidOverride, auth.ErrMFAInvalidOverride},
		{identityaccess.ErrMFAUnsupportedScope, auth.ErrMFAUnsupportedScope},
		{identityaccess.ErrMFAStatusUnavailable, auth.ErrMFAStatusUnavailable},
	} {
		assert.Equal(t, pair.public.Error(), pair.retained.Error())
	}
}
