package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errAccountAuthenticationUnavailable = identityaccess.ErrAccountAuthenticationUnavailable

func loginResult(value *domain.LoginResult) *identityaccess.LoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.LoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	return &result
}

// authenticationSentinels translates in precedence order. Most entries pair a
// public sentinel with itself: the flows report it already, and the entry
// keeps a wrapped error's text under it instead of letting MapError reduce
// it. The account lookup, the tenant requirement and the passkey origin keep
// their domain twins, which the store and the ceremony rules report.
var authenticationSentinels = []struct {
	internal error
	public   error
}{
	{identityaccess.ErrInvalidCredentials, identityaccess.ErrInvalidCredentials},
	{identityaccess.ErrAccountInactive, identityaccess.ErrAccountInactive},
	{domain.ErrAccountNotFound, identityaccess.ErrAccountNotFound},
	{identityaccess.ErrTenantNotFound, identityaccess.ErrTenantNotFound},
	{identityaccess.ErrTenantAccessDenied, identityaccess.ErrTenantAccessDenied},
	{identityaccess.ErrParentMustUseParentPortal, identityaccess.ErrParentMustUseParentPortal},
	{identityaccess.ErrAccountNoGuardianRole, identityaccess.ErrAccountNoGuardianRole},
	{identityaccess.ErrAccountNoSchoolPortalRole, identityaccess.ErrAccountNoSchoolPortalRole},
	{identityaccess.ErrMustUseSchoolPortal, identityaccess.ErrMustUseSchoolPortal},
	{identityaccess.ErrDemoSessionTenantLocked, identityaccess.ErrDemoSessionTenantLocked},
	{identityaccess.ErrInvalidToken, identityaccess.ErrInvalidToken},
	{identityaccess.ErrTokenExpired, identityaccess.ErrTokenExpired},
	{identityaccess.ErrTokenNotFound, identityaccess.ErrTokenNotFound},
	{identityaccess.ErrMFAStatusUnavailable, identityaccess.ErrMFAStatusUnavailable},
	{domain.ErrTenantRequired, identityaccess.ErrTenantRequired},
	{identityaccess.ErrAccountSessionNotFound, identityaccess.ErrAccountSessionNotFound},
	{identityaccess.ErrAccountSessionRotated, identityaccess.ErrAccountSessionRotated},
	{identityaccess.ErrMFAChallengeTokenInvalid, identityaccess.ErrMFAChallengeTokenInvalid},
	{identityaccess.ErrMFACodeInvalid, identityaccess.ErrMFACodeInvalid},
	{identityaccess.ErrMFALocked, identityaccess.ErrMFALocked},
	{identityaccess.ErrMFARateLimited, identityaccess.ErrMFARateLimited},
	{identityaccess.ErrMFANotEnrolled, identityaccess.ErrMFANotEnrolled},
	{identityaccess.ErrMFAAlreadyEnrolled, identityaccess.ErrMFAAlreadyEnrolled},
	{identityaccess.ErrMFAPermissionDenied, identityaccess.ErrMFAPermissionDenied},
	{identityaccess.ErrMFAInvalidOverride, identityaccess.ErrMFAInvalidOverride},
	{identityaccess.ErrMFAUnsupportedScope, identityaccess.ErrMFAUnsupportedScope},
	{domain.ErrPasskeyOriginInvalid, identityaccess.ErrPasskeyOriginInvalid},
	{identityaccess.ErrPasskeySessionInvalid, identityaccess.ErrPasskeySessionInvalid},
	{identityaccess.ErrPasskeyNotFound, identityaccess.ErrPasskeyNotFound},
}

// authenticationError translates a flow error to the public contract: the
// operation envelope keeps its text, and a cause that carries an internal
// sentinel gains the public one while keeping its message.
func authenticationError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: authenticationError(operation.Err)}
	}
	for _, sentinel := range authenticationSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return MapError(err)
}

// translatedError keeps a wrapped cause's text while exposing the public
// sentinel to errors.Is.
type translatedError struct {
	text   string
	public error
	cause  error
}

func (e *translatedError) Error() string { return e.text }

func (e *translatedError) Unwrap() []error { return []error{e.public, e.cause} }
