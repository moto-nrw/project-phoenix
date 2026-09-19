package schoolportal

import (
	"context"
	"errors"
	"net"
	"time"
)

// The school login, the school session mint, the school switch and the
// account second factor are owned by Identity & Access (#3251, #3331). The
// school portal may not name the owner's contract, so it consumes them
// through the two runtimes below, which the composition root binds — the
// same shape the reset runtime next to it uses (#3364). The school scope is
// fixed by the binding: a challenge of another portal never reaches these
// handlers.

// LoginStatus discriminates the shapes a school login response can take.
type LoginStatus string

const (
	// LoginStatusAuthenticated means the response carries a usable token pair.
	LoginStatusAuthenticated LoginStatus = "authenticated"
	// LoginStatusMFARequired means the account must present a second factor
	// before tokens are issued.
	LoginStatusMFARequired LoginStatus = "mfa_required"
	// LoginStatusMFAEnrollmentRequired means the school requires MFA and the
	// account has no credential yet; the response carries an
	// enrollment-scoped access token.
	LoginStatusMFAEnrollmentRequired LoginStatus = "mfa_enrollment_required"
)

// LoginResult is the discriminated login answer the portal renders.
type LoginResult struct {
	Status                LoginStatus
	AccessToken           string
	RefreshToken          string
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	// TrustedDeviceEnabled and TrustedDeviceDays are populated on the
	// MFA-required branch only and mirror the school's settings, so the
	// frontend renders the label that matches the cookie it will get.
	TrustedDeviceEnabled bool
	TrustedDeviceDays    int
}

// VerifiedChallenge is what a redeemed school challenge hands the portal.
type VerifiedChallenge struct {
	AccountID int64
	TenantID  int64
}

// AuthRuntime is the school portal's session port.
type AuthRuntime struct {
	// Login authenticates without a pinned school and picks the first
	// school where the account holds a school-portal role.
	Login func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error)
	// LoginAtSchool is the selected-school variant: the account must hold a
	// school-portal role at tenantSlug.
	LoginAtSchool func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error)
	// IssueTokens mints the school-scope pair for an account that proved
	// its identity through the second factor; the role is re-checked.
	IssueTokens func(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// SwitchSchool re-authenticates a school session at another school.
	SwitchSchool func(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (accessToken, refreshToken string, err error)

	// Cause strips the owner's operation envelope so the portal renders the
	// refusal itself, not the envelope around it.
	Cause func(error) error
	// InvalidCredentials reports an unknown address or a wrong password;
	// the portal masks both as one answer.
	InvalidCredentials func(error) bool
	// AccountInactive reports a deactivated account.
	AccountInactive func(error) bool
	// NoSchoolPortalRole reports an account without a school-portal role.
	NoSchoolPortalRole func(error) bool
	// SchoolNotFound reports a pinned school that is deactivated or gone.
	SchoolNotFound func(error) bool
	// SchoolAccessDenied reports a revoked membership at the school.
	SchoolAccessDenied func(error) bool
	// MFABlocked reports a locked-out or rate-limited second factor.
	MFABlocked func(error) bool
	// MFAUnavailable reports the fail-closed gate: the portal answers 503
	// instead of treating it as bad credentials.
	MFAUnavailable func(error) bool
}

func (rt *AuthRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.LoginAtSchool != nil && rt.IssueTokens != nil &&
		rt.SwitchSchool != nil && rt.Cause != nil && rt.InvalidCredentials != nil &&
		rt.AccountInactive != nil && rt.NoSchoolPortalRole != nil && rt.SchoolNotFound != nil &&
		rt.SchoolAccessDenied != nil && rt.MFABlocked != nil && rt.MFAUnavailable != nil
}

// MFARuntime is the school portal's second-factor port. Every challenge call
// is bound to the school scope by the composition root.
type MFARuntime struct {
	VerifyChallenge func(ctx context.Context, challengeToken, code string) (VerifiedChallenge, error)
	// VerifyChallengeForOwner additionally pins the challenge to the account
	// and school the enrollment token names, before any code is compared.
	VerifyChallengeForOwner func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (VerifiedChallenge, error)
	// ResendChallenge re-issues the code and answers with the renewed JWT.
	ResendChallenge func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	// StartChallenge mails the enrollment code and names its row.
	StartChallenge     func(ctx context.Context, accountID, tenantID int64, ip net.IP) (string, error)
	Enroll             func(ctx context.Context, accountID int64) error
	IssueTrustedDevice func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)

	// ChallengeUnusable reports an expired, spent, mis-scoped or wrong code.
	ChallengeUnusable func(error) bool
	// Blocked reports a locked-out or rate-limited second factor.
	Blocked func(error) bool
	// Unavailable reports the fail-closed status lookup.
	Unavailable func(error) bool
	// AlreadyEnrolled reports a repeated enrollment, which is not a failure.
	AlreadyEnrolled func(error) bool
}

func (rt *MFARuntime) complete() bool {
	return rt != nil && rt.VerifyChallenge != nil && rt.VerifyChallengeForOwner != nil &&
		rt.ResendChallenge != nil && rt.StartChallenge != nil && rt.Enroll != nil &&
		rt.IssueTrustedDevice != nil && rt.ChallengeUnusable != nil && rt.Blocked != nil &&
		rt.Unavailable != nil && rt.AlreadyEnrolled != nil
}

// The sentences the school portal renders for a refused login. They are this
// package's wire contract; the owner reports the same texts.
var (
	// ErrLoginUnavailable reports a resource composed without the session
	// runtime.
	ErrLoginUnavailable = errors.New("account sessions are not composed")
	// ErrInvalidCredentials is the 401 body of a masked login refusal.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAccountNotFound is the 401 body of a switch against a gone account.
	ErrAccountNotFound = errors.New("account not found")
	// ErrAccountInactive is the 401 body of a deactivated account.
	ErrAccountInactive = errors.New("account is inactive")
	// ErrAccountNoSchoolPortalRole is the 403 body of an account with no
	// school-portal role.
	ErrAccountNoSchoolPortalRole = errors.New("account has no school portal role at any school")
	// ErrTenantNotFound is the 404 body of a deactivated or deleted school.
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrTenantAccessDenied is the 401 body of a revoked membership.
	ErrTenantAccessDenied = errors.New("account does not have access to this tenant")
)
