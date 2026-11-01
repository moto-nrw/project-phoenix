package catererportalhttp

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// SignIn is the Identity & Access sign-in of the caterer portal: the
// password with the second factor the caterer switched on, and the passkey
// (#4497).
type SignIn interface {
	LoginCaterer(ctx context.Context, request identityaccess.CatererLoginRequest) (identityaccess.CatererLoginResult, error)
	VerifyCatererMFA(ctx context.Context, verification identityaccess.CatererMFAVerification) (identityaccess.CatererSession, error)
	ResendCatererMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	BeginCatererPasskeyLogin(ctx context.Context, expectedOrigin string) (identityaccess.PasskeyCeremonyOptions, error)
	FinishCatererPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error)
}

// SecondFactor is the caterer's own management of its second factor in its
// account (#4497).
type SecondFactor interface {
	CatererMFAStatus(ctx context.Context, accountID int64) (identityaccess.CatererMFAState, error)
	StartCatererMFA(ctx context.Context, accountID int64, ip net.IP) (identityaccess.CatererMFAChallenge, error)
	ConfirmCatererMFA(ctx context.Context, confirmation identityaccess.CatererMFAConfirmation) (identityaccess.CatererSession, error)
	DisableCatererMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error
	ListCatererTrustedDevices(ctx context.Context, accountID int64) ([]identityaccess.AccountTrustedDevice, error)
	RevokeCatererTrustedDevice(ctx context.Context, accountID, deviceID int64) error
	StartCatererPasskeyEnrollment(ctx context.Context, accountID int64, ip net.IP) (identityaccess.PasskeyEnrollmentChallenge, error)
	BeginCatererPasskeyRegistration(ctx context.Context, request identityaccess.CatererPasskeyRegistrationStart) (identityaccess.PasskeyCeremonyOptions, error)
	FinishCatererPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationFinish) (identityaccess.PasskeyCredentialSummary, error)
	ListCatererPasskeys(ctx context.Context, accountID int64) ([]identityaccess.PasskeyCredentialSummary, error)
	RevokeCatererPasskey(ctx context.Context, accountID, passkeyID int64) error
}
