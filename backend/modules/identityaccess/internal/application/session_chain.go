package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// revokeSessionChain revokes every session a logout of presented must end
// (#5194): its family, and for a row without family the whole chain it
// links into (domain.SessionChain). Logout reports success only after all of
// them are gone, so no access token of the chain stays valid. It runs inside
// the logout's transaction under the account lock.
func (s *AccountAuthentication) revokeSessionChain(ctx context.Context, presented domain.AccountSession) ([]domain.AccountSession, error) {
	listed, err := s.sessions.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: presented.AccountID})
	if err != nil {
		return nil, err
	}
	revoked := make([]domain.AccountSession, 0, len(listed))
	families := make(map[string]bool)
	for _, session := range domain.SessionChain(presented, domainAccountSessions(listed)) {
		if session.FamilyID == "" {
			if err := s.sessions.DeleteAccountSession(ctx, session.ID); err != nil {
				return nil, err
			}
			revoked = append(revoked, session)
			continue
		}
		if families[session.FamilyID] {
			continue
		}
		families[session.FamilyID] = true
		family, err := s.sessions.RevokeAccountSessionFamily(ctx, session.FamilyID)
		if err != nil {
			return nil, err
		}
		revoked = append(revoked, domainAccountSessions(family)...)
	}
	return revoked, nil
}
