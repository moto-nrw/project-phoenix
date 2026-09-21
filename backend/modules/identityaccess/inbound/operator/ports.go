package operator

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The invitation management, the profile e-mail change, the second factor,
// the passkey ceremonies and the school-account MFA admin consume the owner
// through this seam (#3364, moved here from api/operator by #3231): the
// capabilities, the values they carry and the outcomes the surface
// classifies on, all identical to the owner's so no translation, and
// therefore no drift, sits between them.

// The values the operator routes render.
type (
	// Operator is one operator identity row.
	Operator = identityaccess.Operator
	// OperatorInvitation is one invitation link with its delivery state.
	OperatorInvitation = identityaccess.OperatorInvitation
	// OperatorInvitationPreview is what the public accept page may show.
	OperatorInvitationPreview = identityaccess.OperatorInvitationPreview
	// OperatorInvitationRequest is one invitation to create.
	OperatorInvitationRequest = identityaccess.OperatorInvitationRequest
	// OperatorInvitationAcceptance is what an invitee supplies.
	OperatorInvitationAcceptance = identityaccess.OperatorInvitationAcceptance
	// OperatorEmailChangeRequest is one confirmation link to create.
	OperatorEmailChangeRequest = identityaccess.OperatorEmailChangeRequest
	// OperatorTrustedDevice is one operator remember-device row.
	OperatorTrustedDevice = identityaccess.OperatorTrustedDevice
	// OperatorPasskeyRegistrationStart starts an operator registration.
	OperatorPasskeyRegistrationStart = identityaccess.OperatorPasskeyRegistrationStart
	// OperatorPasskeyRegistrationFinish completes an operator registration.
	OperatorPasskeyRegistrationFinish = identityaccess.OperatorPasskeyRegistrationFinish
	// OperatorPasskeyLoginFinish completes an operator login ceremony.
	OperatorPasskeyLoginFinish = identityaccess.OperatorPasskeyLoginFinish
	// PasskeyEnrollmentChallenge is what a registration start hands a portal.
	PasskeyEnrollmentChallenge = identityaccess.PasskeyEnrollmentChallenge
	// PasskeyCeremonyOptions names one started ceremony.
	PasskeyCeremonyOptions = identityaccess.PasskeyCeremonyOptions
	// PasskeyCredentialSummary is one registered credential as a portal lists it.
	PasskeyCredentialSummary = identityaccess.PasskeyCredentialSummary
	// PasskeyLoginResult is the token pair a completed login ceremony minted.
	PasskeyLoginResult = identityaccess.PasskeyLoginResult
	// MFAAdminState is the read the operator "Manage MFA" modal shows.
	MFAAdminState = identityaccess.MFAAdminState
	// InvalidInputError carries input the owner refused, with the message
	// the operator surface translates into German.
	InvalidInputError = identityaccess.InvalidInputError
	// MFAPolicy is a resolved second-factor verdict with the role predicate
	// left unapplied.
	MFAPolicy = identityaccess.MFAPolicy
	// VerifiedMFAChallenge is what a redeemed account challenge reports.
	VerifiedMFAChallenge = identityaccess.VerifiedMFAChallenge
	// AccountTrustedDevice is one school-account remember-device row.
	AccountTrustedDevice = identityaccess.AccountTrustedDevice
)

// The capabilities the operator dashboard consumes.
type (
	// Operators reads and lists operator identity rows.
	Operators = identityaccess.OperatorQuery
	// OperatorSessions mints a token pair for an operator whose identity was
	// proven through a second factor or a passkey.
	OperatorSessions interface {
		IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	}
	// OperatorProvisioning is the invitation and e-mail change capability.
	OperatorProvisioning = identityaccess.OperatorProvisioning
	// OperatorMFA is the operator second factor.
	OperatorMFA = identityaccess.OperatorMFAFlows
	// OperatorPasskeys is the operator-portal ceremony capability.
	OperatorPasskeys = identityaccess.OperatorPasskeyFlows
	// AccountMFA is the school-account second factor the operator MFA admin
	// screens read and override on behalf of school staff.
	AccountMFA = identityaccess.AccountMFA
)

// OperatorAccess is what the invitation, profile and second-factor routes
// need: the operator directory, the token mint their second factors end in,
// and the invitation and e-mail change flows.
type OperatorAccess interface {
	Operators
	OperatorSessions
	OperatorProvisioning
}

// The admin-override values the operator MFA screens hand to the capability.
const (
	MFAAdminOverrideNone     = identityaccess.MFAAdminOverrideNone
	MFAAdminOverrideForceOff = identityaccess.MFAAdminOverrideForceOff
	MFAAdminOverrideForceOn  = identityaccess.MFAAdminOverrideForceOn
)

// IsValidMFAAdminOverride is the allow list the admin surfaces check before
// they hand a value to the capability.
func IsValidMFAAdminOverride(value string) bool {
	return identityaccess.IsValidMFAAdminOverride(value)
}

// The outcomes the operator routes classify on. They are the owner's
// values, so errors.Is decides exactly what it decided before.
var (
	ErrOperatorNotFound               = identityaccess.ErrOperatorNotFound
	ErrOperatorInactive               = identityaccess.ErrOperatorInactive
	ErrOperatorInvalidCredentials     = identityaccess.ErrOperatorInvalidCredentials
	ErrOperatorPasswordMismatch       = identityaccess.ErrOperatorPasswordMismatch
	ErrOperatorEmailExists            = identityaccess.ErrOperatorEmailExists
	ErrOperatorEmailInUse             = identityaccess.ErrOperatorEmailInUse
	ErrOperatorInvitationNotFound     = identityaccess.ErrOperatorInvitationNotFound
	ErrOperatorInvitationRateLimited  = identityaccess.ErrOperatorInvitationRateLimited
	ErrOperatorEmailChangeNotFound    = identityaccess.ErrOperatorEmailChangeNotFound
	ErrOperatorEmailChangeRateLimited = identityaccess.ErrOperatorEmailChangeRateLimited
	ErrOperatorEmailChangeSameEmail   = identityaccess.ErrOperatorEmailChangeSameEmail

	ErrMFAChallengeTokenInvalid = identityaccess.ErrMFAChallengeTokenInvalid
	ErrMFACodeInvalid           = identityaccess.ErrMFACodeInvalid
	ErrMFALocked                = identityaccess.ErrMFALocked
	ErrMFARateLimited           = identityaccess.ErrMFARateLimited
	ErrMFANotEnrolled           = identityaccess.ErrMFANotEnrolled
	ErrMFAAlreadyEnrolled       = identityaccess.ErrMFAAlreadyEnrolled
	ErrMFAPermissionDenied      = identityaccess.ErrMFAPermissionDenied
	ErrMFAInvalidOverride       = identityaccess.ErrMFAInvalidOverride
	ErrMFAStatusUnavailable     = identityaccess.ErrMFAStatusUnavailable

	ErrPasskeyOriginInvalid  = identityaccess.ErrPasskeyOriginInvalid
	ErrPasskeySessionInvalid = identityaccess.ErrPasskeySessionInvalid
	ErrPasskeyNotFound       = identityaccess.ErrPasskeyNotFound
)

// InvalidInput reports input the owner refused and hands back the message
// the operator surface translates into German. It is the only refusal whose
// text the surface reads.
func InvalidInput(err error) (error, bool) {
	var invalid *identityaccess.InvalidInputError
	if errors.As(err, &invalid) {
		return invalid.Err, true
	}
	return nil, false
}
