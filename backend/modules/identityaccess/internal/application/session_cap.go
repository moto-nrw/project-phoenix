package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// capSessionsUnlessDemo enforces the session cap for every account except one
// a demo access signed in: all visitors of the public demo share that
// account, so the cap would end the demo of an earlier visitor (#3462). The
// session just minted always stays; the cap evicts among the others.
func (s *AccountAuthentication) capSessionsUnlessDemo(ctx context.Context, accountID int64, minted domain.AccountSession) error {
	demo, err := s.store.DemoAccountExists(ctx, accountID)
	if err != nil {
		return fmt.Errorf("check demo account before session cap: %w", err)
	}
	if demo {
		return nil
	}
	evicted, err := s.enforcePortalSessionCap(ctx, accountID, minted)
	if err != nil {
		return err
	}
	s.queuePushCleanup(ctx, accountID, evicted, "session_cap")
	return nil
}

// enforcePortalSessionCap keeps at most five active sessions in this portal
// group; other portals keep their own sessions.
func (s *AccountAuthentication) enforcePortalSessionCap(ctx context.Context, accountID int64, minted domain.AccountSession) ([]domain.AccountSession, error) {
	capped, err := s.sessions.EnforceAccountSessionCap(ctx, accountID, minted.PortalScope, domain.MaxActiveSessionsPerPortal, minted.ID)
	if err != nil {
		return nil, fmt.Errorf("enforce active session cap: %w", err)
	}
	evicted := domainAccountSessions(capped)
	if err := s.auditRevokedSessions(ctx, evicted, "session_cap", "", ""); err != nil {
		return nil, err
	}
	return evicted, nil
}
