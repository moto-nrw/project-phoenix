package identityaccess

import (
	"context"
	"fmt"
)

// SignInDataRetention deletes short-lived sign-in data after the fixed
// windows of the retention list (#3954): invitations for parents and
// caterers 30 days after redemption or expiry, e-mail codes and passkey ceremonies of accounts
// and operators 7 days after expiry. The hourly token cleanup runs it in an
// administrative transaction that reaches every school.
type SignInDataRetention interface {
	DeleteExpiredGuardianInvitations(ctx context.Context) (int, error)
	DeleteExpiredCatererInvitations(ctx context.Context) (int, error)
	DeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error)
	DeleteExpiredPasskeySessions(ctx context.Context) (int, error)
}

func (m *Module) DeleteExpiredGuardianInvitations(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredGuardianInvitations(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired guardian invitations: %w", err)
	}
	return deleted, nil
}

func (m *Module) DeleteExpiredCatererInvitations(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredCatererInvitations(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired caterer invitations: %w", err)
	}
	return deleted, nil
}

func (m *Module) DeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredMFAEmailChallenges(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired mfa email challenges: %w", err)
	}
	return deleted, nil
}

func (m *Module) DeleteExpiredPasskeySessions(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredPasskeySessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired passkey sessions: %w", err)
	}
	return deleted, nil
}
