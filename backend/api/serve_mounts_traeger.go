package api

import (
	"context"
	"errors"
	"net"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/traegerportal"
	traegerViewCompose "github.com/moto-nrw/project-phoenix/modules/traegerview/compose"
	traegerViewHTTP "github.com/moto-nrw/project-phoenix/modules/traegerview/http"
)

// mountTraegerPortalRoutes serves the Träger-Portal ("moto träger", ADR 0084,
// #4649) at /traeger: the public sign-in and the Träger route group, which
// accepts only Träger sessions. Token refresh and logout go through the
// shared scope-preserving /auth/refresh and /auth/logout.
func mountTraegerPortalRoutes(root chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery) error {
	portal, err := NewTraegerPortalHTTP(identity, organizations)
	if err != nil {
		return err
	}
	root.Mount("/traeger", portal.RouterWithAuthRateLimiter(limiters.authMiddleware(), ProtectedTraegerRoutes))
	return nil
}

// TraegerPortalIdentity is what the Träger-Portal takes from Identity &
// Access: the Träger sign-in and the account second factor.
type TraegerPortalIdentity interface {
	identityaccess.TraegerAccountLogin
	identityaccess.AccountMFAChallenges
	identityaccess.AccountMFAEnrollment
}

// NewTraegerPortalHTTP binds the Träger sign-in, its second factor and the
// Träger projection to the portal's route ports.
func NewTraegerPortalHTTP(identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery) (*traegerportal.Resource, error) {
	if identity == nil || organizations == nil {
		return nil, errors.New("traeger portal: identity and organizations are required")
	}
	overview, err := traegerViewCompose.New(organizations)
	if err != nil {
		return nil, err
	}
	return traegerportal.NewResource(TraegerPortalAuth(identity), TraegerPortalMFA(identity), traegerViewHTTP.NewResource(overview)), nil
}

// TraegerPortalAuth binds the Träger-Portal's session port to Identity &
// Access.
func TraegerPortalAuth(sessions identityaccess.TraegerAccountLogin) traegerportal.AuthRuntime {
	if sessions == nil {
		return traegerportal.AuthRuntime{}
	}
	return traegerportal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*traegerportal.LoginResult, error) {
			result, err := sessions.LoginTraeger(ctx, email, password, ipAddress, userAgent, organizationID)
			if err != nil || result == nil {
				return nil, err
			}
			return traegerPortalLoginResult(result), nil
		},
		IssueTokens: func(ctx context.Context, proof traegerportal.VerifiedChallenge, organizationID int64, ipAddress, userAgent string) (*traegerportal.LoginResult, error) {
			result, err := sessions.IssueTraegerTokensForVerifiedMFA(ctx, identityaccess.VerifiedMFAChallenge{
				AccountID: proof.AccountID, Scope: identityaccess.MFAChallengeScopeOrg, MFAResetGeneration: proof.MFAResetGeneration,
			}, organizationID, ipAddress, userAgent)
			if err != nil || result == nil {
				return nil, err
			}
			return traegerPortalLoginResult(result), nil
		},
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NoMembership:       identityIs(identityaccess.ErrNoTraegerMembership),
		MFABlocked:         identityMFABlocked,
		MFAUnavailable:     identityIs(identityaccess.ErrMFAStatusUnavailable),
		InvalidSession:     identityIs(identityaccess.ErrInvalidToken),
	}
}

func traegerPortalLoginResult(result *identityaccess.TraegerLoginResult) *traegerportal.LoginResult {
	organizations := make([]traegerportal.Organization, 0, len(result.Organizations))
	for _, organization := range result.Organizations {
		organizations = append(organizations, traegerportal.Organization(organization))
	}
	return &traegerportal.LoginResult{
		Status: traegerportal.LoginStatus(result.Status), AccessToken: result.AccessToken,
		RefreshToken: result.RefreshToken, ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		OrganizationID: result.OrganizationID, Organizations: organizations,
	}
}

// TraegerPortalMFA binds the Träger second factor. The org challenge scope is
// applied here, so a challenge of another portal is refused before any code
// is compared. A Träger challenge names no school.
func TraegerPortalMFA(mfa interface {
	identityaccess.AccountMFAChallenges
	identityaccess.AccountMFAEnrollment
}) traegerportal.MFARuntime {
	if mfa == nil {
		return traegerportal.MFARuntime{}
	}
	verified := func(proof identityaccess.VerifiedMFAChallenge) traegerportal.VerifiedChallenge {
		return traegerportal.VerifiedChallenge{AccountID: proof.AccountID, MFAResetGeneration: proof.MFAResetGeneration}
	}
	return traegerportal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (traegerportal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeOrg)
			return verified(proof), err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID int64) (traegerportal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, identityaccess.MFAChallengeScopeOrg, accountID, 0)
			return verified(proof), err
		},
		ResendChallenge: func(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
			return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, identityaccess.MFAChallengeScopeOrg)
		},
		StartChallenge: func(ctx context.Context, accountID int64, ip net.IP) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, 0, identityaccess.MFAChallengeScopeOrg, ip)
		},
		Enroll:            mfa.EnrollMFA,
		ChallengeUnusable: identityChallengeUnusable,
		Blocked:           identityMFABlocked,
		Unavailable:       identityIs(identityaccess.ErrMFAStatusUnavailable),
		AlreadyEnrolled:   identityIs(identityaccess.ErrMFAAlreadyEnrolled),
	}
}
