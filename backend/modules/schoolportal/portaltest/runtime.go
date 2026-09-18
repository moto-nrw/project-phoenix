// Package portaltest binds the school-portal runtimes the service root
// composes to the plain-typed runtimes the portal declares, for the suites
// that mount the portal router. The production hand-off lives in
// api/identity_runtime_composition.go; this is the test support's copy of it,
// so a router suite exercises the same scope binding production does (#3364).
package portaltest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/services"
)

// AuthRuntime maps the composed school-portal authentication runtime onto the
// portal's own runtime type.
func AuthRuntime(runtime services.SchoolPortalAuthRuntime) schoolportal.AuthRuntime {
	if runtime.Login == nil {
		return schoolportal.AuthRuntime{}
	}
	return schoolportal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*schoolportal.LoginResult, error) {
			result, err := runtime.Login(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
			return loginResult(result), err
		},
		LoginAtSchool: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*schoolportal.LoginResult, error) {
			result, err := runtime.LoginAtSchool(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
			return loginResult(result), err
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

func loginResult(result *services.SchoolPortalLoginResult) *schoolportal.LoginResult {
	if result == nil {
		return nil
	}
	return &schoolportal.LoginResult{
		Status: schoolportal.LoginStatus(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.MFAEnrollmentRequired,
		TrustedDeviceEnabled:  result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
}

// MFARuntime maps the composed school-portal second factor onto the portal's
// own runtime type. The school challenge scope is applied by the composition
// this receives, not by the handlers.
func MFARuntime(runtime services.SchoolPortalMFARuntime) schoolportal.MFARuntime {
	if runtime.VerifyChallenge == nil {
		return schoolportal.MFARuntime{}
	}
	return schoolportal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (schoolportal.VerifiedChallenge, error) {
			verified, err := runtime.VerifyChallenge(ctx, challengeToken, code)
			return schoolportal.VerifiedChallenge(verified), err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (schoolportal.VerifiedChallenge, error) {
			verified, err := runtime.VerifyChallengeForOwner(ctx, challengeToken, code, accountID, tenantID)
			return schoolportal.VerifiedChallenge(verified), err
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
