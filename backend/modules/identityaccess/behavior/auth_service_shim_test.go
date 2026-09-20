package behavior_test

import (
	"context"

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
	GetActiveTokens(ctx context.Context, accountID int) ([]identityaccess.AccountSession, error)
}

// sessions is the module the shims delegate to.
func (s *fixtureOwnedAuthService) sessions() *identityaccess.Module { return s.module }

func (s *fixtureOwnedAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.sessions().LoginWithAudit(ctx, email, password, "", "", "")
}

func (s *fixtureOwnedAuthService) LoginParent(ctx context.Context, email, password string) (string, string, error) {
	return s.sessions().LoginParentWithAudit(ctx, email, password, "", "")
}

func (s *fixtureOwnedAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	return s.sessions().RefreshTokenWithAudit(ctx, refreshToken, "", "")
}

func (s *fixtureOwnedAuthService) RevokeAllTokens(ctx context.Context, accountID int) error {
	return s.sessions().RevokeAllTokensWithReason(ctx, int64(accountID), "administrative_revoke")
}

func (s *fixtureOwnedAuthService) GetActiveTokens(ctx context.Context, accountID int) ([]identityaccess.AccountSession, error) {
	return s.sessions().ListActiveSessions(ctx, int64(accountID))
}
