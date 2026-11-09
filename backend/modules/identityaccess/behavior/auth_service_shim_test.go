package behavior_test

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The behaviour suites in this directory were written against the retained
// auth service's call shapes: int account ids, the audit-less login and
// refresh convenience calls. Those
// shapes are gone with the ports (#3364); the flows behind them are the
// module's. The shims below keep the suites driving real behaviour without
// rewriting twenty thousand lines of assertions.

// retainedSessionCalls are the convenience shapes the suites call.
type retainedSessionCalls interface {
	Login(ctx context.Context, email, password string) (string, string, error)
	LoginParent(ctx context.Context, email, password string) (string, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	RevokeAllTokens(ctx context.Context, accountID int) error
	GetActiveTokens(ctx context.Context, accountID int) ([]identityaccess.ActiveSession, error)
}

// sessions is the module the shims delegate to.
func (s *fixtureOwnedAuthService) sessions() identityaccess.Owner { return s.module }

func (s *fixtureOwnedAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	return tenantPasswordLogin(s.sessions(), email, password, "", "", "")
}

func (s *fixtureOwnedAuthService) LoginParent(ctx context.Context, email, password string) (string, string, error) {
	return s.sessions().LoginParentWithAudit(ctx, email, password, "", "")
}

func (s *fixtureOwnedAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	return s.sessions().RefreshTokenWithAudit(ctx, refreshToken, "", "", "")
}

func (s *fixtureOwnedAuthService) RevokeAllTokens(ctx context.Context, accountID int) error {
	return s.sessions().RevokeAllTokensWithReason(ctx, int64(accountID), "administrative_revoke")
}

func (s *fixtureOwnedAuthService) GetActiveTokens(ctx context.Context, accountID int) ([]identityaccess.ActiveSession, error) {
	return s.sessions().ListActiveSessions(ctx, int64(accountID))
}

// tenantPasswordLogin is the tenant password login the HTTP route runs, on a
// context without a school like the public route's. It fails unless the
// login answered with a session.
func tenantPasswordLogin(login identityaccess.TenantAccountLogin, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	result, err := login.LoginWithMFAGate(context.Background(), email, password, ipAddress, userAgent, tenantSlug, "")
	if err != nil {
		return "", "", err
	}
	if result.Status != identityaccess.LoginStatusAuthenticated {
		return "", "", fmt.Errorf("tenant login answered %q instead of a session", result.Status)
	}
	return result.AccessToken, result.RefreshToken, nil
}

// issueSchoolTokens mints a school session through the school MFA exchange
// with a proof of the account's current reset generation, as a redeemed code
// carries it.
func issueSchoolTokens(login identityaccess.SchoolAccountLogin, ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	proof := identityaccess.VerifiedMFAChallenge{AccountID: accountID, TenantID: tenantID, Scope: identityaccess.MFAChallengeScopeSchool}
	return login.IssueSchoolTokensForVerifiedMFA(ctx, proof, ipAddress, userAgent)
}
