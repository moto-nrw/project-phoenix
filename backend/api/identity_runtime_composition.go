package api

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// The root binds the portals' sign-in runtimes to Identity & Access and fixes
// the scope each portal acts in: a guardian resets and signs in on the
// parents portal and a Lehrkraft on the school portal, and a challenge or a
// reset link of another portal never reaches those handlers (#4006). The
// portals classify the refusals through the owner's rule tables
// (portalauth).

// SchoolPortalAuth binds the school portal's session runtime. A nil
// capability yields the zero runtime, which the portal reports as not
// composed.
func SchoolPortalAuth(sessions identityaccess.SchoolAccountLogin) schoolPortal.AuthRuntime {
	if sessions == nil {
		return schoolPortal.AuthRuntime{}
	}
	return schoolPortal.AuthRuntime{
		Login:         sessions.LoginSchoolWithMFAGate,
		LoginAtSchool: sessions.LoginSchoolAtTenantWithMFAGate,
		IssueTokens: func(ctx context.Context, proof schoolPortal.VerifiedChallenge, ipAddress, userAgent string) (string, string, error) {
			return sessions.IssueSchoolTokensForVerifiedMFA(ctx, identityaccess.VerifiedMFAChallenge{AccountID: proof.AccountID, TenantID: proof.TenantID, Scope: identityaccess.MFAChallengeScopeSchool, MFAResetGeneration: proof.MFAResetGeneration}, ipAddress, userAgent)
		},
		SwitchSchool: sessions.SwitchSchool,
	}
}

// SchoolPortalMFA binds the school portal's second factor. The school scope
// is applied here, so a tenant- or operator-portal challenge is refused
// before any code is compared.
func SchoolPortalMFA(mfa identityaccess.AccountMFA) schoolPortal.MFARuntime {
	if mfa == nil {
		return schoolPortal.MFARuntime{}
	}
	verified := func(proof identityaccess.VerifiedMFAChallenge) schoolPortal.VerifiedChallenge {
		return schoolPortal.VerifiedChallenge{AccountID: proof.AccountID, TenantID: proof.TenantID, MFAResetGeneration: proof.MFAResetGeneration}
	}
	return schoolPortal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (schoolPortal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool)
			return verified(proof), err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID, tenantID int64) (schoolPortal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, identityaccess.MFAChallengeScopeSchool, accountID, tenantID)
			return verified(proof), err
		},
		ResendChallenge: func(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
			return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, identityaccess.MFAChallengeScopeSchool)
		},
		StartChallenge: func(ctx context.Context, accountID, tenantID int64, ip net.IP, credentialGeneration int64) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, tenantID, identityaccess.MFAChallengeScopeSchool, ip, credentialGeneration)
		},
		Enroll:               mfa.EnrollMFA,
		PrepareTrustedDevice: mfa.PrepareTrustedDevice,
	}
}

// parentPortalLogin binds the parents portal's login runtime.
func parentPortalLogin(sessions identityaccess.ParentAccountLogin) parentAPI.LoginRuntime {
	if sessions == nil {
		return parentAPI.LoginRuntime{}
	}
	return parentAPI.LoginRuntime{Login: sessions.LoginParentWithAudit, ConfirmPassword: sessions.ConfirmAccountPassword}
}
