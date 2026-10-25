package api

import (
	"context"
	"errors"
	"net"
	"time"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// The school and parents portals and the enrollment routes may not name the
// Identity & Access contract, so the root binds each of them a runtime of
// plain-typed closures over the owner's capability (#3332, #3364). The scope
// a portal acts in is fixed here: a guardian resets and signs in on the
// parents portal and a Lehrkraft on the school portal, and a challenge or a
// reset link of another portal never reaches those handlers (#4006).

// schoolPasswordResets binds the school portal's reset runtime: only
// accounts holding a school-portal role are served, and the link lands on
// the school host. A nil capability yields the zero runtime, which the
// portal reports as unavailable.
func schoolPasswordResets(resets identityaccess.PasswordResets) schoolPortal.PasswordResetRuntime {
	if resets == nil {
		return schoolPortal.PasswordResetRuntime{}
	}
	return schoolPortal.PasswordResetRuntime{
		Initiate:     initiatePasswordReset(resets, identityaccess.PasswordResetScopeSchool),
		Reset:        resets.ResetPassword,
		LinkUnusable: passwordResetLinkUnusable,
		TooWeak:      passwordResetTooWeak,
		RetryAfter:   passwordResetRetryAfter,
	}
}

// parentPasswordResets binds the parents portal's reset runtime: only
// accounts with guardian access are served, and the link lands on the
// parents host.
func parentPasswordResets(resets identityaccess.PasswordResets) parentAPI.PasswordResetRuntime {
	if resets == nil {
		return parentAPI.PasswordResetRuntime{}
	}
	return parentAPI.PasswordResetRuntime{
		Initiate:     initiatePasswordReset(resets, identityaccess.PasswordResetScopeParent),
		Reset:        resets.ResetPassword,
		LinkUnusable: passwordResetLinkUnusable,
		TooWeak:      passwordResetTooWeak,
		RetryAfter:   passwordResetRetryAfter,
	}
}

// initiatePasswordReset issues a link of one portal. The link itself never
// leaves the owner: the portal answers the same body whether or not one was
// issued.
func initiatePasswordReset(resets identityaccess.PasswordResets, scope identityaccess.PasswordResetScope) func(context.Context, string) error {
	return func(ctx context.Context, email string) error {
		_, err := resets.InitiatePasswordReset(ctx, email, scope)
		return err
	}
}

// passwordResetLinkUnusable reports a link that is unknown, expired or
// already spent. A missing row counts: the portals never say which of the
// three it was.
func passwordResetLinkUnusable(err error) bool {
	cause := identityCause(err)
	return errors.Is(cause, identityaccess.ErrInvalidToken) || errors.Is(cause, transactionruntime.ErrNoRows)
}

// passwordResetTooWeak reports a credential the password policy refused.
func passwordResetTooWeak(err error) bool {
	return errors.Is(identityCause(err), identityaccess.ErrPasswordTooWeak)
}

// passwordResetRetryAfter reports the rate-limit rejection and the seconds
// until the address may request again.
func passwordResetRetryAfter(err error) (int, bool) {
	var limited *identityaccess.PasswordResetRateLimitError
	if !errors.As(err, &limited) {
		return 0, false
	}
	return limited.RetryAfterSeconds(time.Now()), true
}

// enrollmentGuardianInvitations hands the enrollment decision routes the one
// call they make: approving the first child of a family fires one
// invitation, fire-and-forget. A nil capability yields the zero runtime,
// which the routes report as not configured.
func enrollmentGuardianInvitations(invitations identityaccess.GuardianInvitations) enrollmentAPI.GuardianInvitationRuntime {
	if invitations == nil {
		return enrollmentAPI.GuardianInvitationRuntime{}
	}
	return enrollmentAPI.GuardianInvitationRuntime{
		Create: func(ctx context.Context, guardianProfileID, createdBy int64) error {
			_, err := invitations.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
				GuardianProfileID: guardianProfileID, CreatedBy: createdBy,
			})
			return err
		},
	}
}

// SchoolPortalAuth binds the school portal's session runtime. A nil
// capability yields the zero runtime, which the portal reports as not
// composed.
func SchoolPortalAuth(sessions identityaccess.SchoolAccountLogin) schoolPortal.AuthRuntime {
	if sessions == nil {
		return schoolPortal.AuthRuntime{}
	}
	return schoolPortal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*schoolPortal.LoginResult, error) {
			result, err := sessions.LoginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
			return schoolPortalLoginResult(result), err
		},
		LoginAtSchool: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*schoolPortal.LoginResult, error) {
			result, err := sessions.LoginSchoolAtTenantWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
			return schoolPortalLoginResult(result), err
		},
		IssueTokens: func(ctx context.Context, proof schoolPortal.VerifiedChallenge, ipAddress, userAgent string) (string, string, error) {
			return sessions.IssueSchoolTokensForVerifiedMFA(ctx, identityaccess.VerifiedMFAChallenge{AccountID: proof.AccountID, TenantID: proof.TenantID, Scope: identityaccess.MFAChallengeScopeSchool, MFAResetGeneration: proof.MFAResetGeneration}, ipAddress, userAgent)
		},
		SwitchSchool:       sessions.SwitchSchool,
		InvalidSession:     identityIs(identityaccess.ErrInvalidToken),
		Cause:              identityCause,
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NoSchoolPortalRole: identityIs(identityaccess.ErrAccountNoSchoolPortalRole),
		SchoolNotFound:     identityIs(identityaccess.ErrTenantNotFound),
		SchoolAccessDenied: identityIs(identityaccess.ErrTenantAccessDenied),
		MFABlocked:         identityMFABlocked,
		MFAUnavailable:     identityIs(identityaccess.ErrMFAStatusUnavailable),
		PortalNotBooked:    identityPortalNotBooked,
	}
}

func schoolPortalLoginResult(result *identityaccess.LoginResult) *schoolPortal.LoginResult {
	if result == nil {
		return nil
	}
	return &schoolPortal.LoginResult{
		Status: schoolPortal.LoginStatus(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.MFAEnrollmentRequired,
		TrustedDeviceEnabled:  result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
}

// SchoolPortalMFA binds the school portal's second factor. The school scope
// is applied here, so a tenant- or operator-portal challenge is refused
// before any code is compared.
func SchoolPortalMFA(mfa identityaccess.AccountMFA) schoolPortal.MFARuntime {
	if mfa == nil {
		return schoolPortal.MFARuntime{}
	}
	return schoolPortal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (schoolPortal.VerifiedChallenge, error) {
			verified, err := mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool)
			return schoolPortal.VerifiedChallenge{AccountID: verified.AccountID, TenantID: verified.TenantID, MFAResetGeneration: verified.MFAResetGeneration}, err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (schoolPortal.VerifiedChallenge, error) {
			verified, err := mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool, accountID, tenantID)
			return schoolPortal.VerifiedChallenge{AccountID: verified.AccountID, TenantID: verified.TenantID, MFAResetGeneration: verified.MFAResetGeneration}, err
		},
		ResendChallenge: func(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
			return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, identityaccess.MFAChallengeScopeSchool)
		},
		StartChallenge: func(ctx context.Context, accountID, tenantID int64, ip net.IP) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, tenantID, identityaccess.MFAChallengeScopeSchool, ip)
		},
		Enroll: mfa.EnrollMFA,
		PrepareTrustedDevice: func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (schoolPortal.TrustedDeviceIssue, error) {
			issue, err := mfa.PrepareTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
			return schoolPortal.TrustedDeviceIssue(issue), err
		},
		ChallengeUnusable: identityChallengeUnusable,
		Blocked:           identityMFABlocked,
		Unavailable:       identityIs(identityaccess.ErrMFAStatusUnavailable),
		AlreadyEnrolled:   identityIs(identityaccess.ErrMFAAlreadyEnrolled),
	}
}

// parentPortalLogin binds the parents portal's login runtime.
func parentPortalLogin(sessions identityaccess.ParentAccountLogin) parentAPI.LoginRuntime {
	if sessions == nil {
		return parentAPI.LoginRuntime{}
	}
	return parentAPI.LoginRuntime{
		Login:              sessions.LoginParentWithAudit,
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NotAGuardian:       identityIs(identityaccess.ErrAccountNoGuardianRole),
		PortalNotBooked:    identityPortalNotBooked,
		ConfirmPassword:    sessions.ConfirmAccountPassword,
	}
}

// identityPortalNotBooked reports a portal session refused because the
// school has not booked the portal's module (#4895).
func identityPortalNotBooked(err error) (module, moduleLabel string, ok bool) {
	notBooked, ok := identityaccess.PortalNotBooked(err)
	if !ok {
		return "", "", false
	}
	return notBooked.Module, notBooked.ModuleLabel, true
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
