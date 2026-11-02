package identityaccess

import (
	"context"
	"errors"
	"net"
	"time"
)

// The caterer's second factor (#4497, ADR 0077 point 7). It is voluntary:
// the caterer switches it on in its account, and no school or Träger can
// require it. It belongs to the caterer's account, not to a school, so its
// challenges, its trusted devices and its passkey ceremonies name no school.
// The factors are the existing ones: the e-mail code, a passkey and the
// remembered device. A caterer who lost access gets the second factor reset
// by a moto operator.

// MFAChallengeScopeCaterer is the caterer portal (#4497). A code started
// there is redeemable only there.
const MFAChallengeScopeCaterer = "caterer"

var (
	// ErrCatererMFANotEnabled refuses switching off a second factor the
	// caterer never switched on.
	ErrCatererMFANotEnabled = errors.New("caterer second factor is not switched on")
	// ErrCatererNotFound reports an operator action on a caterer that does
	// not exist.
	ErrCatererNotFound = errors.New("caterer not found")
	// ErrCatererMFAResetReasonRequired refuses an operator reset without a
	// reason someone can read in the audit.
	ErrCatererMFAResetReasonRequired = errors.New("reason must be at least 3 characters")
)

// CatererLoginStatus is the outcome of a caterer's password sign-in.
type CatererLoginStatus string

const (
	// CatererLoginAuthenticated carries the session pair.
	CatererLoginAuthenticated CatererLoginStatus = "authenticated"
	// CatererLoginMFARequired carries the challenge of the mailed code.
	CatererLoginMFARequired CatererLoginStatus = "mfa_required"
)

// CatererLoginRequest is a caterer's password sign-in. TrustedDeviceCookie
// is the remember-device cookie the browser sent, or empty.
type CatererLoginRequest struct {
	Email               string
	Password            string
	IPAddress           string
	UserAgent           string
	TrustedDeviceCookie string
}

// CatererLoginResult is either the session pair or, for a caterer with the
// second factor switched on, the challenge of the code mailed to it.
type CatererLoginResult struct {
	Status            CatererLoginStatus
	AccessToken       string
	RefreshToken      string
	ChallengeToken    string
	MaskedEmail       string
	TrustedDeviceDays int
}

// CatererMFAVerification redeems the mailed code of a caterer sign-in.
type CatererMFAVerification struct {
	ChallengeToken string
	Code           string
	RememberDevice bool
	IPAddress      string
	UserAgent      string
}

// CatererSession is a session pair proven by a second factor. A remembered
// device adds its signed cookie; an empty cookie remembers none.
type CatererSession struct {
	AccessToken            string
	RefreshToken           string
	TrustedDeviceCookie    string
	TrustedDeviceExpiresAt time.Time
}

// CatererMFAState is what the caterer's account page shows.
type CatererMFAState struct {
	Enabled     bool
	MaskedEmail string
}

// CatererMFAChallenge names the code mailed to switch the second factor on.
type CatererMFAChallenge struct {
	ChallengeToken string
	MaskedEmail    string
}

// CatererMFAConfirmation switches the second factor on with the mailed code.
type CatererMFAConfirmation struct {
	AccountID      int64
	ChallengeToken string
	Code           string
	IPAddress      string
	UserAgent      string
}

// CatererPasskeyRegistrationStart starts a passkey registration on the
// caterer portal, gated by a mailed code like every registration.
type CatererPasskeyRegistrationStart struct {
	AccountID      int64
	ExpectedOrigin string
	Code           string
	Name           string
}

// CatererSecuritySummary is one caterer account as the operator sees it
// for the reset of its second factor.
type CatererSecuritySummary struct {
	ID         int64
	AccountID  int64
	Name       string
	Email      string
	MFAEnabled bool
}

// CatererSignIn is the caterer portal's sign-in: the password with the
// second factor where the caterer switched it on, and the passkey.
type CatererSignIn interface {
	LoginCaterer(ctx context.Context, request CatererLoginRequest) (CatererLoginResult, error)
	VerifyCatererMFA(ctx context.Context, verification CatererMFAVerification) (CatererSession, error)
	// ResendCatererMFA mails a new code for a caterer challenge and returns
	// the renewed challenge.
	ResendCatererMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	BeginCatererPasskeyLogin(ctx context.Context, expectedOrigin string) (PasskeyCeremonyOptions, error)
	FinishCatererPasskeyLogin(ctx context.Context, request AccountPasskeyLoginFinish) (PasskeyLoginResult, error)
}

// CatererSecondFactor is the caterer's own management of its second factor
// in its account.
type CatererSecondFactor interface {
	CatererMFAStatus(ctx context.Context, accountID int64) (CatererMFAState, error)
	StartCatererMFA(ctx context.Context, accountID int64, ip net.IP) (CatererMFAChallenge, error)
	// ConfirmCatererMFA switches the second factor on and mints a session
	// proven by it; the account's other sessions end at their next refresh.
	ConfirmCatererMFA(ctx context.Context, confirmation CatererMFAConfirmation) (CatererSession, error)
	// DisableCatererMFA switches the second factor off after checking the
	// current password like a sign-in.
	DisableCatererMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error
	ListCatererTrustedDevices(ctx context.Context, accountID int64) ([]AccountTrustedDevice, error)
	RevokeCatererTrustedDevice(ctx context.Context, accountID, deviceID int64) error
	StartCatererPasskeyEnrollment(ctx context.Context, accountID int64, ip net.IP) (PasskeyEnrollmentChallenge, error)
	BeginCatererPasskeyRegistration(ctx context.Context, request CatererPasskeyRegistrationStart) (PasskeyCeremonyOptions, error)
	FinishCatererPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationFinish) (PasskeyCredentialSummary, error)
	ListCatererPasskeys(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error)
	RevokeCatererPasskey(ctx context.Context, accountID, passkeyID int64) error
}

// CatererMFAAdministration is the operator's recovery of a caterer who lost
// access to its second factor. School admins have none: a caterer supplies
// several schools.
type CatererMFAAdministration interface {
	ListCaterersForOperator(ctx context.Context) ([]CatererSecuritySummary, error)
	// OperatorResetCatererMFA removes the e-mail factor and the trusted
	// devices of one caterer account, writes the operator audit with the
	// reason and mails the caterer, in one administrative unit. An account
	// without the factor is refused with ErrCatererMFANotEnabled.
	OperatorResetCatererMFA(ctx context.Context, operatorID, catererID, accountID int64, reason string) error
}
