package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errOperatorAuthenticationUnavailable = identityaccess.ErrOperatorAuthenticationUnavailable

func operatorLoginResult(value *domain.OperatorLoginResult) *identityaccess.OperatorLoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.OperatorLoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	if value.Operator != nil {
		operator := identityaccess.Operator(*value.Operator)
		result.Operator = &operator
	}
	return &result
}

var operatorSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrOperatorEmailInvalid, identityaccess.ErrOperatorEmailInvalid},
	{domain.ErrOperatorEmailTooLong, identityaccess.ErrOperatorEmailTooLong},
	{domain.ErrOperatorPasswordTooWeak, identityaccess.ErrOperatorPasswordTooWeak},
	{domain.ErrOperatorDisplayNameRequired, identityaccess.ErrOperatorDisplayNameRequired},
	{domain.ErrOperatorDisplayNameTooLong, identityaccess.ErrOperatorDisplayNameTooLong},
	// A caterer account joins no school (ADR 0077); it answers like an
	// address that is taken.
	{domain.ErrCatererAccountJoinsNoSchool, identityaccess.ErrEmailAlreadyExists},
}

// operatorPublicSentinels are the operator outcomes the flows already report
// as public sentinels. A match keeps the error as it is, wrapped text
// included, instead of passing it on to the shared translation.
var operatorPublicSentinels = []error{
	identityaccess.ErrOperatorInvalidCredentials,
	identityaccess.ErrOperatorInactive,
	identityaccess.ErrOperatorNotFound,
	identityaccess.ErrOperatorRefreshTokenInvalid,
	identityaccess.ErrOperatorPasswordMismatch,
	identityaccess.ErrAccountTenantAccessNotFound,
	identityaccess.ErrAccountTenantAccessExists,
	identityaccess.ErrSchoolNotFound,
	identityaccess.ErrSchoolDeleted,
}

// operatorError translates an operator flow error to the public contract:
// an input rejection keeps its message under the public type, an operator
// input sentinel gains its public counterpart, a public operator sentinel
// stays as it is, and everything else falls through to the
// account-authentication translation the access flows share.
func operatorError(err error) error {
	if err == nil {
		return nil
	}
	if invalid, ok := errors.AsType[*domain.InvalidInputError](err); ok {
		return &identityaccess.InvalidInputError{Err: operatorError(invalid.Err)}
	}
	for _, sentinel := range operatorSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	for _, sentinel := range operatorPublicSentinels {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return authenticationError(err)
}
