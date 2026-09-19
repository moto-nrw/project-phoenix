package services

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The school and parents portals may not name the Identity & Access
// contract, so the root hands each of them a runtime of plain-typed
// closures over the login, the session mint and the second factor — the
// shape the reset runtime next to it already uses (#3364). The scope a
// challenge belongs to is fixed here, so a challenge of another portal
// never reaches those handlers.

// SchoolPortalLoginResult is the discriminated login answer the school
// portal renders. Its field names and types match the struct the portal
// declares, so the HTTP composition maps it across without either side
// importing the other.
type SchoolPortalLoginResult struct {
	Status                string
	AccessToken           string
	RefreshToken          string
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	TrustedDeviceEnabled  bool
	TrustedDeviceDays     int
}

// SchoolPortalAuthRuntime is the plain-typed session runtime of the school
// portal.
type SchoolPortalAuthRuntime struct {
	Login         func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*SchoolPortalLoginResult, error)
	LoginAtSchool func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*SchoolPortalLoginResult, error)
	IssueTokens   func(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error)
	SwitchSchool  func(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error)

	Cause              func(error) error
	InvalidCredentials func(error) bool
	AccountInactive    func(error) bool
	NoSchoolPortalRole func(error) bool
	SchoolNotFound     func(error) bool
	SchoolAccessDenied func(error) bool
	MFABlocked         func(error) bool
	MFAUnavailable     func(error) bool
}

// SchoolPortalVerifiedChallenge is what a redeemed school challenge hands
// the portal.
type SchoolPortalVerifiedChallenge struct {
	AccountID int64
	TenantID  int64
}

// SchoolPortalMFARuntime is the plain-typed second-factor runtime of the
// school portal. Every challenge call is bound to the school scope.
type SchoolPortalMFARuntime struct {
	VerifyChallenge         func(ctx context.Context, challengeToken, code string) (SchoolPortalVerifiedChallenge, error)
	VerifyChallengeForOwner func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (SchoolPortalVerifiedChallenge, error)
	ResendChallenge         func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	StartChallenge          func(ctx context.Context, accountID, tenantID int64, ip net.IP) (string, error)
	Enroll                  func(ctx context.Context, accountID int64) error
	IssueTrustedDevice      func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error)

	ChallengeUnusable func(error) bool
	Blocked           func(error) bool
	Unavailable       func(error) bool
	AlreadyEnrolled   func(error) bool
}

// ParentLoginRuntime is the plain-typed login runtime of the parents portal.
type ParentLoginRuntime struct {
	Login              func(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error)
	InvalidCredentials func(error) bool
	AccountInactive    func(error) bool
	NotAGuardian       func(error) bool
}

// SchoolPortalAuthentication binds the school portal's session runtime. A
// nil capability yields the zero runtime, which the portal reports as not
// composed.
func (f *Factory) SchoolPortalAuthentication() SchoolPortalAuthRuntime {
	return SchoolPortalAuthenticationOver(f.AccountAuthentication())
}

// SchoolPortalAuthenticationOver binds the school portal's session runtime
// over the given capability, the way the factory binds it.
func SchoolPortalAuthenticationOver(sessions identityaccess.AccountAuthentication) SchoolPortalAuthRuntime {
	if sessions == nil {
		return SchoolPortalAuthRuntime{}
	}
	return SchoolPortalAuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*SchoolPortalLoginResult, error) {
			result, err := sessions.LoginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
			return schoolPortalLoginResult(result), err
		},
		LoginAtSchool: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*SchoolPortalLoginResult, error) {
			result, err := sessions.LoginSchoolAtTenantWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
			return schoolPortalLoginResult(result), err
		},
		IssueTokens:        sessions.IssueSchoolTokensForAuthenticatedAccount,
		SwitchSchool:       sessions.SwitchSchool,
		Cause:              identityCause,
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NoSchoolPortalRole: identityIs(identityaccess.ErrAccountNoSchoolPortalRole),
		SchoolNotFound:     identityIs(identityaccess.ErrTenantNotFound),
		SchoolAccessDenied: identityIs(identityaccess.ErrTenantAccessDenied),
		MFABlocked:         identityMFABlocked,
		MFAUnavailable:     identityIs(identityaccess.ErrMFAStatusUnavailable),
	}
}

// SchoolPortalMFA binds the school portal's second factor. The school scope
// is applied here, so a tenant- or operator-portal challenge is refused
// before any code is compared.
func (f *Factory) SchoolPortalMFA() SchoolPortalMFARuntime {
	return SchoolPortalMFAOver(f.MFA)
}

// SchoolPortalMFAOver binds the school portal's second factor over the given
// capability, with the school scope applied, the way the factory binds it.
func SchoolPortalMFAOver(mfa identityaccess.AccountMFA) SchoolPortalMFARuntime {
	if mfa == nil {
		return SchoolPortalMFARuntime{}
	}
	return SchoolPortalMFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (SchoolPortalVerifiedChallenge, error) {
			verified, err := mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool)
			return SchoolPortalVerifiedChallenge{AccountID: verified.AccountID, TenantID: verified.TenantID}, err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (SchoolPortalVerifiedChallenge, error) {
			verified, err := mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool, accountID, tenantID)
			return SchoolPortalVerifiedChallenge{AccountID: verified.AccountID, TenantID: verified.TenantID}, err
		},
		ResendChallenge: func(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
			return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, identityaccess.MFAChallengeScopeSchool)
		},
		StartChallenge: func(ctx context.Context, accountID, tenantID int64, ip net.IP) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, tenantID, identityaccess.MFAChallengeScopeSchool, ip)
		},
		Enroll:             mfa.EnrollMFA,
		IssueTrustedDevice: mfa.IssueTrustedDevice,
		ChallengeUnusable:  identityChallengeUnusable,
		Blocked:            identityMFABlocked,
		Unavailable:        identityIs(identityaccess.ErrMFAStatusUnavailable),
		AlreadyEnrolled:    identityIs(identityaccess.ErrMFAAlreadyEnrolled),
	}
}

// ParentPortalLogin binds the parents portal's login runtime.
func (f *Factory) ParentPortalLogin() ParentLoginRuntime {
	return ParentPortalLoginOver(f.AccountAuthentication())
}

// ParentPortalLoginOver binds the parents portal's login runtime over the
// given capability, the way the factory binds it.
func ParentPortalLoginOver(sessions identityaccess.AccountAuthentication) ParentLoginRuntime {
	if sessions == nil {
		return ParentLoginRuntime{}
	}
	return ParentLoginRuntime{
		Login:              sessions.LoginParentWithAudit,
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NotAGuardian:       identityIs(identityaccess.ErrAccountNoGuardianRole),
	}
}

func schoolPortalLoginResult(result *identityaccess.LoginResult) *SchoolPortalLoginResult {
	if result == nil {
		return nil
	}
	return &SchoolPortalLoginResult{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.MFAEnrollmentRequired,
		TrustedDeviceEnabled:  result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
}

// identityIs reports the outcome by its owner sentinel.
func identityIs(sentinel error) func(error) bool {
	return func(err error) bool { return errors.Is(err, sentinel) }
}

// identityInvalidCredentials reports the two refusals a login masks as one:
// a wrong password and an address nobody holds.
func identityInvalidCredentials(err error) bool {
	return errors.Is(err, identityaccess.ErrInvalidCredentials) || errors.Is(err, identityaccess.ErrAccountNotFound)
}

// identityMFABlocked reports a locked-out or rate-limited second factor.
func identityMFABlocked(err error) bool {
	return errors.Is(err, identityaccess.ErrMFALocked) || errors.Is(err, identityaccess.ErrMFARateLimited)
}

// identityChallengeUnusable reports an expired, spent, mis-scoped challenge
// or a wrong code.
func identityChallengeUnusable(err error) bool {
	return errors.Is(err, identityaccess.ErrMFAChallengeTokenInvalid) ||
		errors.Is(err, identityaccess.ErrMFACodeInvalid) ||
		errors.Is(err, identityaccess.ErrMFAUnsupportedScope)
}

// identityCause strips the owner's operation envelope so a portal renders
// the refusal itself, not the envelope around it.
func identityCause(err error) error {
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation.Err != nil {
		return operation.Err
	}
	return err
}
