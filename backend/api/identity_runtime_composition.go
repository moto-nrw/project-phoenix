package api

import (
	"context"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/services"
)

// The portals and the enrollment routes consume Identity & Access through
// runtimes of plain-typed closures the service root binds (#3332). The
// mappings below carry those runtimes across the adapter boundary, so no
// adapter names the owner's contract and the root does not name the
// adapters.

func schoolPasswordResets(runtime services.PasswordResetRuntime) schoolPortal.PasswordResetRuntime {
	return schoolPortal.PasswordResetRuntime{
		Initiate:     runtime.Initiate,
		Reset:        runtime.Reset,
		LinkUnusable: runtime.LinkUnusable,
		TooWeak:      runtime.TooWeak,
		RetryAfter:   runtime.RetryAfter,
	}
}

func parentPasswordResets(runtime services.PasswordResetRuntime) parentAPI.PasswordResetRuntime {
	return parentAPI.PasswordResetRuntime{
		Initiate:     runtime.Initiate,
		Reset:        runtime.Reset,
		LinkUnusable: runtime.LinkUnusable,
		TooWeak:      runtime.TooWeak,
		RetryAfter:   runtime.RetryAfter,
	}
}

// The enrollment decision routes fire a guardian invitation after an
// approval. They may not name the Identity & Access contract either, so the
// root hands them the one call they make (#3332).
func enrollmentGuardianInvitations(invitations services.GuardianInvitationCapability) enrollmentAPI.GuardianInvitationRuntime {
	runtime := services.EnrollmentGuardianInvitationRuntime(invitations)
	return enrollmentAPI.GuardianInvitationRuntime{Create: runtime.Create}
}

// schoolPortalAuth carries the school portal's session runtime across the
// adapter boundary (#3364).
func schoolPortalAuth(runtime services.SchoolPortalAuthRuntime) schoolPortal.AuthRuntime {
	if runtime.Login == nil {
		return schoolPortal.AuthRuntime{}
	}
	return schoolPortal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*schoolPortal.LoginResult, error) {
			result, err := runtime.Login(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
			return schoolPortalLoginResult(result), err
		},
		LoginAtSchool: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*schoolPortal.LoginResult, error) {
			result, err := runtime.LoginAtSchool(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
			return schoolPortalLoginResult(result), err
		},
		IssueTokens:        runtime.IssueTokens,
		SwitchSchool:       runtime.SwitchSchool,
		Cause:              runtime.Cause,
		InvalidCredentials: runtime.InvalidCredentials,
		AccountInactive:    runtime.AccountInactive,
		NoSchoolPortalRole: runtime.NoSchoolPortalRole,
		SchoolNotFound:     runtime.SchoolNotFound,
		SchoolAccessDenied: runtime.SchoolAccessDenied,
		MFABlocked:         runtime.MFABlocked,
		MFAUnavailable:     runtime.MFAUnavailable,
	}
}

func schoolPortalLoginResult(result *services.SchoolPortalLoginResult) *schoolPortal.LoginResult {
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

// schoolPortalMFA carries the school portal's second-factor runtime across
// the adapter boundary.
func schoolPortalMFA(runtime services.SchoolPortalMFARuntime) schoolPortal.MFARuntime {
	if runtime.VerifyChallenge == nil {
		return schoolPortal.MFARuntime{}
	}
	return schoolPortal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (schoolPortal.VerifiedChallenge, error) {
			verified, err := runtime.VerifyChallenge(ctx, challengeToken, code)
			return schoolPortal.VerifiedChallenge(verified), err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (schoolPortal.VerifiedChallenge, error) {
			verified, err := runtime.VerifyChallengeForOwner(ctx, challengeToken, code, accountID, tenantID)
			return schoolPortal.VerifiedChallenge(verified), err
		},
		ResendChallenge:    runtime.ResendChallenge,
		StartChallenge:     runtime.StartChallenge,
		Enroll:             runtime.Enroll,
		IssueTrustedDevice: runtime.IssueTrustedDevice,
		ChallengeUnusable:  runtime.ChallengeUnusable,
		Blocked:            runtime.Blocked,
		Unavailable:        runtime.Unavailable,
		AlreadyEnrolled:    runtime.AlreadyEnrolled,
	}
}

// parentPortalLogin carries the parents portal's login runtime across the
// adapter boundary.
func parentPortalLogin(runtime services.ParentLoginRuntime) parentAPI.LoginRuntime {
	return parentAPI.LoginRuntime{
		Login:              runtime.Login,
		InvalidCredentials: runtime.InvalidCredentials,
		AccountInactive:    runtime.AccountInactive,
		NotAGuardian:       runtime.NotAGuardian,
	}
}
